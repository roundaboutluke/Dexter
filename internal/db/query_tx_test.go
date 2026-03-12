package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestWithTxCommitPersistsChangesAndRunsAfterCommit(t *testing.T) {
	db, _ := openTxStubDB(t, nil, 0)
	query := NewQuery(db)
	afterCommit := 0

	if err := query.WithTx(context.Background(), func(tx *Query) error {
		if _, err := tx.InsertQuery("profiles", map[string]any{"id": "user-1", "profile_no": 1, "name": "default"}); err != nil {
			return err
		}
		tx.AfterCommit(func() { afterCommit++ })
		return nil
	}); err != nil {
		t.Fatalf("with tx commit: %v", err)
	}

	rows, err := query.SelectAllQuery("profiles", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("select rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d, want 1", len(rows))
	}
	if afterCommit != 1 {
		t.Fatalf("afterCommit=%d, want 1", afterCommit)
	}
}

func TestWithTxRollbackDiscardsChangesAndSkipsAfterCommit(t *testing.T) {
	db, _ := openTxStubDB(t, nil, 0)
	query := NewQuery(db)
	afterCommit := 0

	errBoom := errors.New("boom")
	err := query.WithTx(context.Background(), func(tx *Query) error {
		if _, err := tx.InsertQuery("profiles", map[string]any{"id": "user-1", "profile_no": 1, "name": "default"}); err != nil {
			return err
		}
		tx.AfterCommit(func() { afterCommit++ })
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("with tx err=%v, want %v", err, errBoom)
	}

	rows, err := query.SelectAllQuery("profiles", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("select rows: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows=%d, want 0", len(rows))
	}
	if afterCommit != 0 {
		t.Fatalf("afterCommit=%d, want 0", afterCommit)
	}
}

func TestWithTxNestedReusesActiveTransaction(t *testing.T) {
	db, state := openTxStubDB(t, nil, 0)
	query := NewQuery(db)

	if err := query.WithTx(context.Background(), func(tx *Query) error {
		if _, err := tx.InsertQuery("profiles", map[string]any{"id": "user-1", "profile_no": 1, "name": "default"}); err != nil {
			return err
		}
		return tx.WithTx(context.Background(), func(nested *Query) error {
			if _, err := nested.UpdateQuery("profiles", map[string]any{"name": "nested"}, map[string]any{"id": "user-1", "profile_no": 1}); err != nil {
				return err
			}
			return nil
		})
	}); err != nil {
		t.Fatalf("with tx commit: %v", err)
	}

	rows, err := query.SelectAllQuery("profiles", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("select rows: %v", err)
	}
	if len(rows) != 1 || rows[0]["name"] != "nested" {
		t.Fatalf("rows=%#v, want nested update", rows)
	}
	if begins := state.beginCount(); begins != 1 {
		t.Fatalf("beginCount=%d, want 1", begins)
	}
}

func TestWithTxRollbackOnDriverExecFailure(t *testing.T) {
	db, _ := openTxStubDB(t, nil, 2)
	query := NewQuery(db)

	err := query.WithTx(context.Background(), func(tx *Query) error {
		if _, err := tx.InsertQuery("profiles", map[string]any{"id": "user-1", "profile_no": 1, "name": "default"}); err != nil {
			return err
		}
		if _, err := tx.InsertQuery("profiles", map[string]any{"id": "user-2", "profile_no": 2, "name": "backup"}); err != nil {
			return err
		}
		return nil
	})
	if err == nil {
		t.Fatalf("expected driver failure")
	}

	rows, err := query.SelectAllQuery("profiles", nil)
	if err != nil {
		t.Fatalf("select rows: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows=%#v, want rollback", rows)
	}
}

var (
	txStubRegisterOnce sync.Once
	txStubDSNSeq       int64
	txStubStates       sync.Map
)

func openTxStubDB(t *testing.T, seed map[string][]map[string]any, failExecAt int) (*sql.DB, *txStubState) {
	t.Helper()
	txStubRegisterOnce.Do(func() {
		sql.Register("poracle-txstub", txStubDriver{})
	})

	state := newTxStubState(seed, failExecAt)
	dsn := fmt.Sprintf("txstub-%d", atomic.AddInt64(&txStubDSNSeq, 1))
	txStubStates.Store(dsn, state)
	t.Cleanup(func() {
		txStubStates.Delete(dsn)
	})

	db, err := sql.Open("poracle-txstub", dsn)
	if err != nil {
		t.Fatalf("open tx stub db: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db, state
}

type txStubDriver struct{}

func (txStubDriver) Open(name string) (driver.Conn, error) {
	value, ok := txStubStates.Load(name)
	if !ok {
		return nil, fmt.Errorf("unknown tx stub dsn %q", name)
	}
	return &txStubConn{state: value.(*txStubState)}, nil
}

type txStubState struct {
	mu         sync.Mutex
	tables     map[string][]map[string]any
	execCount  int
	failExecAt int
	begins     int
}

func newTxStubState(seed map[string][]map[string]any, failExecAt int) *txStubState {
	return &txStubState{
		tables:     cloneTables(seed),
		failExecAt: failExecAt,
	}
}

func (s *txStubState) beginCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.begins
}

type txStubConn struct {
	state    *txStubState
	inTx     bool
	txTables map[string][]map[string]any
}

func (c *txStubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}
func (c *txStubConn) Close() error { return nil }
func (c *txStubConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *txStubConn) BeginTx(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if c.inTx {
		return nil, errors.New("transaction already open")
	}
	c.inTx = true
	c.txTables = cloneTables(c.state.tables)
	c.state.begins++
	return &txStubTx{conn: c}, nil
}

func (c *txStubConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	plainArgs := namedValuesToAny(args)
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.execCount++
	if c.state.failExecAt > 0 && c.state.execCount == c.state.failExecAt {
		return nil, errors.New("forced exec failure")
	}
	tables := c.tables()
	affected, err := applyExec(tables, query, plainArgs)
	if err != nil {
		return nil, err
	}
	return txStubResult(affected), nil
}

func (c *txStubConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	plainArgs := namedValuesToAny(args)
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	rows, columns, err := applyQuery(c.tables(), query, plainArgs)
	if err != nil {
		return nil, err
	}
	return &txStubRows{columns: columns, rows: rows}, nil
}

func (c *txStubConn) Ping(context.Context) error { return nil }

func (c *txStubConn) tables() map[string][]map[string]any {
	if c.inTx {
		return c.txTables
	}
	return c.state.tables
}

type txStubTx struct {
	conn *txStubConn
}

func (tx *txStubTx) Commit() error {
	tx.conn.state.mu.Lock()
	defer tx.conn.state.mu.Unlock()
	if !tx.conn.inTx {
		return errors.New("transaction closed")
	}
	tx.conn.state.tables = cloneTables(tx.conn.txTables)
	tx.conn.txTables = nil
	tx.conn.inTx = false
	return nil
}

func (tx *txStubTx) Rollback() error {
	tx.conn.state.mu.Lock()
	defer tx.conn.state.mu.Unlock()
	tx.conn.txTables = nil
	tx.conn.inTx = false
	return nil
}

type txStubRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *txStubRows) Columns() []string { return r.columns }
func (r *txStubRows) Close() error      { return nil }

func (r *txStubRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

type txStubResult int64

func (r txStubResult) LastInsertId() (int64, error) { return 0, nil }
func (r txStubResult) RowsAffected() (int64, error) { return int64(r), nil }

func namedValuesToAny(values []driver.NamedValue) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value.Value)
	}
	return out
}

func cloneTables(input map[string][]map[string]any) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for table, rows := range input {
		cloned := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			copy := map[string]any{}
			for key, value := range row {
				copy[key] = value
			}
			cloned = append(cloned, copy)
		}
		out[table] = cloned
	}
	return out
}

func applyExec(tables map[string][]map[string]any, query string, args []any) (int64, error) {
	switch {
	case strings.HasPrefix(strings.ToUpper(query), "INSERT INTO "):
		return applyInsert(tables, query, args)
	case strings.HasPrefix(strings.ToUpper(query), "UPDATE "):
		return applyUpdate(tables, query, args)
	case strings.HasPrefix(strings.ToUpper(query), "DELETE FROM "):
		return applyDelete(tables, query, args)
	default:
		return 0, fmt.Errorf("unsupported exec query: %s", query)
	}
}

func applyQuery(tables map[string][]map[string]any, query string, args []any) ([][]driver.Value, []string, error) {
	upper := strings.ToUpper(query)
	switch {
	case strings.HasPrefix(upper, "SELECT COUNT(*) FROM "):
		table, where, err := parseSelect(query, "SELECT COUNT(*) FROM ")
		if err != nil {
			return nil, nil, err
		}
		matches, err := filterRows(tables[table], where, args)
		if err != nil {
			return nil, nil, err
		}
		return [][]driver.Value{{int64(len(matches))}}, []string{"COUNT(*)"}, nil
	case strings.HasPrefix(upper, "SELECT * FROM "):
		table, where, err := parseSelect(query, "SELECT * FROM ")
		if err != nil {
			return nil, nil, err
		}
		matches, err := filterRows(tables[table], where, args)
		if err != nil {
			return nil, nil, err
		}
		columns := []string{}
		if len(matches) > 0 {
			columns = sortedKeys(matches[0])
		}
		values := make([][]driver.Value, 0, len(matches))
		for _, row := range matches {
			entry := make([]driver.Value, 0, len(columns))
			for _, column := range columns {
				entry = append(entry, driver.Value(row[column]))
			}
			values = append(values, entry)
		}
		return values, columns, nil
	default:
		return nil, nil, fmt.Errorf("unsupported query: %s", query)
	}
}

func parseSelect(query, prefix string) (string, string, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(query, prefix))
	rest = strings.SplitN(rest, " LIMIT ", 2)[0]
	parts := strings.SplitN(rest, " WHERE ", 2)
	table := strings.TrimSpace(parts[0])
	if table == "" {
		return "", "", fmt.Errorf("missing table in query %q", query)
	}
	where := ""
	if len(parts) == 2 {
		where = strings.TrimSpace(parts[1])
	}
	return table, where, nil
}

func applyInsert(tables map[string][]map[string]any, query string, args []any) (int64, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(query, "INSERT INTO "))
	startCols := strings.Index(rest, "(")
	endCols := strings.Index(rest, ")")
	if startCols <= 0 || endCols <= startCols {
		return 0, fmt.Errorf("invalid insert query %q", query)
	}
	table := strings.TrimSpace(rest[:startCols])
	columnTokens := strings.Split(rest[startCols+1:endCols], ",")
	columns := make([]string, 0, len(columnTokens))
	for _, token := range columnTokens {
		columns = append(columns, trimIdent(token))
	}
	if len(columns) == 0 || len(args)%len(columns) != 0 {
		return 0, fmt.Errorf("invalid insert args for %q", query)
	}
	rows := tables[table]
	for offset := 0; offset < len(args); offset += len(columns) {
		row := map[string]any{}
		for index, column := range columns {
			row[column] = args[offset+index]
		}
		rows = append(rows, row)
	}
	tables[table] = rows
	return int64(len(args) / len(columns)), nil
}

func applyUpdate(tables map[string][]map[string]any, query string, args []any) (int64, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(query, "UPDATE "))
	setIndex := strings.Index(rest, " SET ")
	if setIndex <= 0 {
		return 0, fmt.Errorf("invalid update query %q", query)
	}
	table := strings.TrimSpace(rest[:setIndex])
	setAndWhere := rest[setIndex+5:]
	whereIndex := strings.Index(setAndWhere, " WHERE ")
	setPart := setAndWhere
	wherePart := ""
	if whereIndex >= 0 {
		setPart = setAndWhere[:whereIndex]
		wherePart = setAndWhere[whereIndex+7:]
	}
	setTokens := strings.Split(setPart, ",")
	setColumns := make([]string, 0, len(setTokens))
	for _, token := range setTokens {
		column := strings.TrimSpace(strings.SplitN(token, "=", 2)[0])
		setColumns = append(setColumns, trimIdent(column))
	}
	if len(args) < len(setColumns) {
		return 0, fmt.Errorf("invalid update args for %q", query)
	}
	setArgs := args[:len(setColumns)]
	whereArgs := args[len(setColumns):]
	affected := int64(0)
	for _, row := range tables[table] {
		matched, err := rowMatches(row, wherePart, whereArgs)
		if err != nil {
			return 0, err
		}
		if !matched {
			continue
		}
		for index, column := range setColumns {
			row[column] = setArgs[index]
		}
		affected++
	}
	return affected, nil
}

func applyDelete(tables map[string][]map[string]any, query string, args []any) (int64, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(query, "DELETE FROM "))
	parts := strings.SplitN(rest, " WHERE ", 2)
	table := strings.TrimSpace(parts[0])
	where := ""
	if len(parts) == 2 {
		where = parts[1]
	}
	remaining := make([]map[string]any, 0, len(tables[table]))
	affected := int64(0)
	for _, row := range tables[table] {
		matched, err := rowMatches(row, where, args)
		if err != nil {
			return 0, err
		}
		if matched {
			affected++
			continue
		}
		remaining = append(remaining, row)
	}
	tables[table] = remaining
	return affected, nil
}

func filterRows(rows []map[string]any, where string, args []any) ([]map[string]any, error) {
	if strings.TrimSpace(where) == "" {
		return append([]map[string]any{}, rows...), nil
	}
	out := []map[string]any{}
	for _, row := range rows {
		matched, err := rowMatches(row, where, args)
		if err != nil {
			return nil, err
		}
		if matched {
			out = append(out, row)
		}
	}
	return out, nil
}

func rowMatches(row map[string]any, where string, args []any) (bool, error) {
	if strings.TrimSpace(where) == "" {
		return true, nil
	}
	clauses := strings.Split(where, " AND ")
	localArgIndex := 0
	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		switch {
		case strings.Contains(clause, " IN "):
			parts := strings.SplitN(clause, " IN ", 2)
			column := trimIdent(parts[0])
			count := strings.Count(parts[1], "?")
			if localArgIndex+count > len(args) {
				return false, fmt.Errorf("missing IN args for %q", clause)
			}
			values := args[localArgIndex : localArgIndex+count]
			localArgIndex += count
			if !containsValue(values, row[column]) {
				return false, nil
			}
		case strings.Contains(clause, " = ?"):
			column := trimIdent(strings.TrimSuffix(clause, " = ?"))
			if localArgIndex >= len(args) {
				return false, fmt.Errorf("missing arg for %q", clause)
			}
			if !sameValue(row[column], args[localArgIndex]) {
				return false, nil
			}
			localArgIndex++
		default:
			return false, fmt.Errorf("unsupported where clause %q", clause)
		}
	}
	return true, nil
}

func containsValue(values []any, target any) bool {
	for _, value := range values {
		if sameValue(value, target) {
			return true
		}
	}
	return false
}

func sameValue(left, right any) bool {
	return fmt.Sprint(left) == fmt.Sprint(right)
}

func trimIdent(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`")
	return value
}
