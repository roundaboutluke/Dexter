package command

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"poraclego/internal/config"
	"poraclego/internal/db"
	"poraclego/internal/i18n"
)

func TestCommunityAddRefreshesOnceAndSkipsMissingTargets(t *testing.T) {
	env := newCommunityTestEnv(t, map[string][]map[string]any{
		"humans": {
			{"id": "admin", "name": "Admin", "type": "discord:user", "community_membership": "[]"},
			{"id": "1", "name": "Alice", "type": "discord:user", "community_membership": "[]"},
			{"id": "2", "name": "Bob", "type": "discord:user", "community_membership": "[\"beta\"]"},
		},
	}, 0)

	reply, err := env.registry.Execute(env.ctx, "community add alpha 1 999 2")
	if err != nil {
		t.Fatalf("execute community add: %v", err)
	}
	if env.refreshCount != 1 {
		t.Fatalf("refreshCount=%d, want 1", env.refreshCount)
	}
	if strings.Contains(reply, "999") {
		t.Fatalf("reply=%q, want missing target skipped", reply)
	}

	row1, err := env.query.SelectOneQuery("humans", map[string]any{"id": "1"})
	if err != nil {
		t.Fatalf("select human 1: %v", err)
	}
	if got := fmt.Sprint(row1["community_membership"]); got != "[\"alpha\"]" {
		t.Fatalf("human 1 community_membership=%s, want [\"alpha\"]", got)
	}
	if got := fmt.Sprint(row1["area_restriction"]); got != "[\"fence-a\"]" {
		t.Fatalf("human 1 area_restriction=%s, want [\"fence-a\"]", got)
	}

	row2, err := env.query.SelectOneQuery("humans", map[string]any{"id": "2"})
	if err != nil {
		t.Fatalf("select human 2: %v", err)
	}
	if got := fmt.Sprint(row2["community_membership"]); got != "[\"alpha\",\"beta\"]" {
		t.Fatalf("human 2 community_membership=%s, want [\"alpha\",\"beta\"]", got)
	}
	if got := fmt.Sprint(row2["area_restriction"]); got != "[\"fence-a\",\"fence-b\"]" {
		t.Fatalf("human 2 area_restriction=%s, want [\"fence-a\",\"fence-b\"]", got)
	}
}

func TestCommunityClearSkipsMissingTargetsAndRefreshesOnce(t *testing.T) {
	env := newCommunityTestEnv(t, map[string][]map[string]any{
		"humans": {
			{"id": "admin", "name": "Admin", "type": "discord:user", "community_membership": "[]"},
			{"id": "1", "name": "Alice", "type": "discord:user", "community_membership": "[\"alpha\"]", "area_restriction": "[\"fence-a\"]"},
		},
	}, 0)

	reply, err := env.registry.Execute(env.ctx, "community clear 1 999")
	if err != nil {
		t.Fatalf("execute community clear: %v", err)
	}
	if env.refreshCount != 1 {
		t.Fatalf("refreshCount=%d, want 1", env.refreshCount)
	}
	if strings.Contains(reply, "999") {
		t.Fatalf("reply=%q, want missing target skipped", reply)
	}

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": "1"})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if got := fmt.Sprint(row["community_membership"]); got != "[]" {
		t.Fatalf("community_membership=%s, want []", got)
	}
	if value, ok := row["area_restriction"]; ok && value != nil {
		t.Fatalf("area_restriction=%v, want nil", value)
	}
}

func TestCommunityRemoveRollbackSkipsRefreshOnFailure(t *testing.T) {
	env := newCommunityTestEnv(t, map[string][]map[string]any{
		"humans": {
			{"id": "admin", "name": "Admin", "type": "discord:user", "community_membership": "[]"},
			{"id": "1", "name": "Alice", "type": "discord:user", "community_membership": "[\"alpha\",\"beta\"]", "area_restriction": "[\"fence-a\",\"fence-b\"]"},
			{"id": "2", "name": "Bob", "type": "discord:user", "community_membership": "[\"alpha\"]", "area_restriction": "[\"fence-a\"]"},
		},
	}, 2)

	if _, err := env.registry.Execute(env.ctx, "community remove alpha 1 2"); err == nil {
		t.Fatalf("expected community remove failure")
	}
	if env.refreshCount != 0 {
		t.Fatalf("refreshCount=%d, want 0", env.refreshCount)
	}

	row1, err := env.query.SelectOneQuery("humans", map[string]any{"id": "1"})
	if err != nil {
		t.Fatalf("select human 1: %v", err)
	}
	if got := fmt.Sprint(row1["community_membership"]); got != "[\"alpha\",\"beta\"]" {
		t.Fatalf("human 1 community_membership=%s, want rollback", got)
	}

	row2, err := env.query.SelectOneQuery("humans", map[string]any{"id": "2"})
	if err != nil {
		t.Fatalf("select human 2: %v", err)
	}
	if got := fmt.Sprint(row2["community_membership"]); got != "[\"alpha\"]" {
		t.Fatalf("human 2 community_membership=%s, want rollback", got)
	}
}

type communityTestEnv struct {
	ctx          *Context
	query        *db.Query
	registry     *Registry
	refreshCount int
}

func newCommunityTestEnv(t *testing.T, seed map[string][]map[string]any, failExecAt int) *communityTestEnv {
	t.Helper()
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
		"areaSecurity": map[string]any{
			"communities": map[string]any{
				"alpha": map[string]any{"locationFence": []any{"fence-a"}},
				"beta":  map[string]any{"locationFence": []any{"fence-b"}},
			},
		},
	})
	sqlDB := openCommunityStubDB(t, seed, failExecAt)
	query := db.NewQuery(sqlDB)
	env := &communityTestEnv{
		query:    query,
		registry: &Registry{handlers: map[string]Handler{"community": &CommunityCommand{}}},
	}
	env.ctx = &Context{
		Config:            cfg,
		Query:             query,
		I18n:              i18n.NewFactory("", cfg),
		RefreshAlertCache: func() { env.refreshCount++ },
		Language:          "en",
		Platform:          "discord",
		Prefix:            "!",
		IsAdmin:           true,
		TargetOverride: &Target{
			ID:   "admin",
			Type: "discord:user",
			Name: "Admin",
		},
	}
	return env
}

var (
	communityStubRegisterOnce sync.Once
	communityStubDSNSeq       int64
	communityStubStates       sync.Map
)

func openCommunityStubDB(t *testing.T, seed map[string][]map[string]any, failExecAt int) *sql.DB {
	t.Helper()
	communityStubRegisterOnce.Do(func() {
		sql.Register("poracle-community-txstub", communityStubDriver{})
	})

	state := newCommunityStubState(seed, failExecAt)
	dsn := fmt.Sprintf("community-txstub-%d", atomic.AddInt64(&communityStubDSNSeq, 1))
	communityStubStates.Store(dsn, state)
	t.Cleanup(func() {
		communityStubStates.Delete(dsn)
	})

	db, err := sql.Open("poracle-community-txstub", dsn)
	if err != nil {
		t.Fatalf("open community stub db: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

type communityStubDriver struct{}

func (communityStubDriver) Open(name string) (driver.Conn, error) {
	value, ok := communityStubStates.Load(name)
	if !ok {
		return nil, fmt.Errorf("unknown community stub dsn %q", name)
	}
	return &communityStubConn{state: value.(*communityStubState)}, nil
}

type communityStubState struct {
	mu         sync.Mutex
	tables     map[string][]map[string]any
	execCount  int
	failExecAt int
}

func newCommunityStubState(seed map[string][]map[string]any, failExecAt int) *communityStubState {
	return &communityStubState{
		tables:     cloneCommunityTables(seed),
		failExecAt: failExecAt,
	}
}

type communityStubConn struct {
	state    *communityStubState
	inTx     bool
	txTables map[string][]map[string]any
}

func (c *communityStubConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepare not supported")
}
func (c *communityStubConn) Close() error { return nil }
func (c *communityStubConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *communityStubConn) BeginTx(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if c.inTx {
		return nil, fmt.Errorf("transaction already open")
	}
	c.inTx = true
	c.txTables = cloneCommunityTables(c.state.tables)
	return &communityStubTx{conn: c}, nil
}

func (c *communityStubConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	plainArgs := namedCommunityValues(args)
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.execCount++
	if c.state.failExecAt > 0 && c.state.execCount == c.state.failExecAt {
		return nil, fmt.Errorf("forced exec failure")
	}
	affected, err := communityApplyExec(c.tables(), query, plainArgs)
	if err != nil {
		return nil, err
	}
	return communityStubResult(affected), nil
}

func (c *communityStubConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	plainArgs := namedCommunityValues(args)
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	rows, columns, err := communityApplyQuery(c.tables(), query, plainArgs)
	if err != nil {
		return nil, err
	}
	return &communityStubRows{columns: columns, rows: rows}, nil
}

func (c *communityStubConn) Ping(context.Context) error { return nil }

func (c *communityStubConn) tables() map[string][]map[string]any {
	if c.inTx {
		return c.txTables
	}
	return c.state.tables
}

type communityStubTx struct {
	conn *communityStubConn
}

func (tx *communityStubTx) Commit() error {
	tx.conn.state.mu.Lock()
	defer tx.conn.state.mu.Unlock()
	tx.conn.state.tables = cloneCommunityTables(tx.conn.txTables)
	tx.conn.txTables = nil
	tx.conn.inTx = false
	return nil
}

func (tx *communityStubTx) Rollback() error {
	tx.conn.state.mu.Lock()
	defer tx.conn.state.mu.Unlock()
	tx.conn.txTables = nil
	tx.conn.inTx = false
	return nil
}

type communityStubRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *communityStubRows) Columns() []string { return r.columns }
func (r *communityStubRows) Close() error      { return nil }

func (r *communityStubRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

type communityStubResult int64

func (r communityStubResult) LastInsertId() (int64, error) { return 0, nil }
func (r communityStubResult) RowsAffected() (int64, error) { return int64(r), nil }

func namedCommunityValues(values []driver.NamedValue) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value.Value)
	}
	return out
}

func cloneCommunityTables(input map[string][]map[string]any) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for table, rows := range input {
		cloned := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			copyRow := map[string]any{}
			for key, value := range row {
				copyRow[key] = value
			}
			cloned = append(cloned, copyRow)
		}
		out[table] = cloned
	}
	return out
}

func communityApplyExec(tables map[string][]map[string]any, query string, args []any) (int64, error) {
	switch {
	case strings.HasPrefix(query, "UPDATE "):
		return communityApplyUpdate(tables, query, args)
	case strings.HasPrefix(query, "INSERT INTO "):
		return communityApplyInsert(tables, query, args)
	default:
		return 0, fmt.Errorf("unsupported exec query %q", query)
	}
}

func communityApplyUpdate(tables map[string][]map[string]any, query string, args []any) (int64, error) {
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
		setColumns = append(setColumns, strings.Trim(column, "`"))
	}
	if len(args) < len(setColumns) {
		return 0, fmt.Errorf("invalid update args for %q", query)
	}
	setArgs := args[:len(setColumns)]
	whereArgs := args[len(setColumns):]
	affected := int64(0)
	for _, row := range tables[table] {
		matched, err := communityRowMatches(row, wherePart, whereArgs)
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

func communityApplyInsert(tables map[string][]map[string]any, query string, args []any) (int64, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(query, "INSERT INTO "))
	open := strings.Index(rest, "(")
	if open <= 0 {
		return 0, fmt.Errorf("invalid insert query %q", query)
	}
	table := strings.TrimSpace(rest[:open])
	afterOpen := rest[open+1:]
	close := strings.Index(afterOpen, ")")
	if close < 0 {
		return 0, fmt.Errorf("invalid insert query %q", query)
	}
	columnTokens := strings.Split(afterOpen[:close], ",")
	columns := make([]string, 0, len(columnTokens))
	for _, token := range columnTokens {
		columns = append(columns, strings.Trim(token, "` "))
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("invalid insert query %q", query)
	}
	valuesPart := strings.TrimSpace(afterOpen[close+1:])
	if !strings.HasPrefix(valuesPart, "VALUES ") {
		return 0, fmt.Errorf("invalid insert query %q", query)
	}
	rowCount := strings.Count(valuesPart, "(")
	if rowCount == 0 || len(args) != rowCount*len(columns) {
		return 0, fmt.Errorf("invalid insert args for %q", query)
	}
	argIndex := 0
	for range rowCount {
		row := map[string]any{}
		for _, column := range columns {
			row[column] = args[argIndex]
			argIndex++
		}
		tables[table] = append(tables[table], row)
	}
	return int64(rowCount), nil
}

func communityApplyQuery(tables map[string][]map[string]any, query string, args []any) ([][]driver.Value, []string, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(query, "SELECT * FROM "))
	parts := strings.SplitN(rest, " WHERE ", 2)
	table := strings.TrimSpace(parts[0])
	where := ""
	if len(parts) == 2 {
		where = strings.TrimSpace(parts[1])
	}
	matches, err := communityFilterRows(tables[table], where, args)
	if err != nil {
		return nil, nil, err
	}
	columns := []string{}
	if len(matches) > 0 {
		columns = communitySortedKeys(matches[0])
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
}

func communityFilterRows(rows []map[string]any, where string, args []any) ([]map[string]any, error) {
	if strings.TrimSpace(where) == "" {
		return append([]map[string]any{}, rows...), nil
	}
	out := []map[string]any{}
	for _, row := range rows {
		matched, err := communityRowMatches(row, where, args)
		if err != nil {
			return nil, err
		}
		if matched {
			out = append(out, row)
		}
	}
	return out, nil
}

func communityRowMatches(row map[string]any, where string, args []any) (bool, error) {
	if strings.TrimSpace(where) == "" {
		return true, nil
	}
	clauses := strings.Split(where, " AND ")
	localArgIndex := 0
	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if !strings.Contains(clause, " = ?") {
			return false, fmt.Errorf("unsupported where clause %q", clause)
		}
		column := strings.Trim(strings.TrimSuffix(clause, " = ?"), "` ")
		if localArgIndex >= len(args) {
			return false, fmt.Errorf("missing arg for %q", clause)
		}
		if fmt.Sprint(row[column]) != fmt.Sprint(args[localArgIndex]) {
			return false, nil
		}
		localArgIndex++
	}
	return true, nil
}

func communitySortedKeys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
