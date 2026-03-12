package bot

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"poraclego/internal/alertstate"
	"poraclego/internal/db"
	"poraclego/internal/webhook"
)

func TestPersistSlashHumanUpdateRefreshesAlertState(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":                   "user-1",
			"preferred_profile_no": 1,
			"current_profile_no":   1,
			"schedule_disabled":    1,
		}},
	}, 0)

	if err := env.discord.persistSlashHumanUpdate("user-1", map[string]any{
		"preferred_profile_no": 2,
		"current_profile_no":   2,
	}); err != nil {
		t.Fatalf("persist human update: %v", err)
	}

	env.waitForRefresh(t, 1)

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if got := toInt(row["preferred_profile_no"], 0); got != 2 {
		t.Fatalf("preferred_profile_no=%d, want 2", got)
	}
	if got := toInt(row["current_profile_no"], 0); got != 2 {
		t.Fatalf("current_profile_no=%d, want 2", got)
	}
}

func TestPersistSlashHumanUpdateSkipsRefreshOnFailure(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":                   "user-1",
			"preferred_profile_no": 1,
			"current_profile_no":   1,
			"schedule_disabled":    0,
		}},
	}, 1)

	if err := env.discord.persistSlashHumanUpdate("user-1", map[string]any{"schedule_disabled": 1}); err == nil {
		t.Fatalf("expected update failure")
	}

	env.assertNoRefresh(t)

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if got := toInt(row["schedule_disabled"], 0); got != 0 {
		t.Fatalf("schedule_disabled=%d, want 0", got)
	}
}

func TestPersistSlashScheduleUpdatesRefreshesAlertState(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"profiles": {{
			"id":           "user-1",
			"profile_no":   1,
			"name":         "Default",
			"active_hours": "[]",
		}},
	}, 0)

	want := []scheduleEntry{{Day: 1, StartMin: 9 * 60, EndMin: 10 * 60}}
	if err := env.discord.persistSlashScheduleUpdates("user-1", map[int][]scheduleEntry{
		1: want,
	}); err != nil {
		t.Fatalf("persist schedule updates: %v", err)
	}

	env.waitForRefresh(t, 1)

	row, err := env.query.SelectOneQuery("profiles", map[string]any{"id": "user-1", "profile_no": 1})
	if err != nil {
		t.Fatalf("select profile: %v", err)
	}
	if got := scheduleEntriesFromRaw(row["active_hours"]); !reflect.DeepEqual(got, want) {
		t.Fatalf("active_hours=%v, want %v", got, want)
	}
}

func TestBuildScheduleEditAssignUpdatesSameProfileReplacesOriginalEntry(t *testing.T) {
	profiles := []map[string]any{{
		"id":         "user-1",
		"profile_no": 1,
		"name":       "Default",
		"active_hours": encodeScheduleEntries([]scheduleEntry{
			{Day: 1, StartMin: 9 * 60, EndMin: 10 * 60},
			{Day: 3, StartMin: 14 * 60, EndMin: 15 * 60},
		}),
	}}
	original := scheduleEntry{ProfileNo: 1, Day: 1, StartMin: 9 * 60, EndMin: 10 * 60}

	updates, errText := buildScheduleEditAssignUpdates(profiles, profileRowByNo(profiles, 1), original, 2, 11*60, 12*60)
	if errText != "" {
		t.Fatalf("build edit updates errText=%q", errText)
	}

	want := []scheduleEntry{
		{Day: 2, StartMin: 11 * 60, EndMin: 12 * 60},
		{Day: 3, StartMin: 14 * 60, EndMin: 15 * 60},
	}
	if got := updates[1]; !reflect.DeepEqual(got, want) {
		t.Fatalf("same-profile updates=%v, want %v", got, want)
	}
}

func TestPersistSlashScheduleMoveRefreshesAlertState(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"profiles": {
			{
				"id":         "user-1",
				"profile_no": 1,
				"name":       "Default",
				"active_hours": encodeScheduleEntries([]scheduleEntry{
					{Day: 1, StartMin: 9 * 60, EndMin: 10 * 60},
				}),
			},
			{
				"id":           "user-1",
				"profile_no":   2,
				"name":         "Night",
				"active_hours": "[]",
			},
		},
	}, 0)

	profiles, err := env.query.SelectAllQuery("profiles", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	original := scheduleEntry{ProfileNo: 1, Day: 1, StartMin: 9 * 60, EndMin: 10 * 60}
	updates, errText := buildScheduleEditAssignUpdates(profiles, profileRowByNo(profiles, 2), original, 2, 18*60, 19*60)
	if errText != "" {
		t.Fatalf("build move updates errText=%q", errText)
	}

	if err := env.discord.persistSlashScheduleUpdates("user-1", updates); err != nil {
		t.Fatalf("persist move: %v", err)
	}

	env.waitForRefresh(t, 1)

	source, err := env.query.SelectOneQuery("profiles", map[string]any{"id": "user-1", "profile_no": 1})
	if err != nil {
		t.Fatalf("select source: %v", err)
	}
	target, err := env.query.SelectOneQuery("profiles", map[string]any{"id": "user-1", "profile_no": 2})
	if err != nil {
		t.Fatalf("select target: %v", err)
	}
	if got := scheduleEntriesFromRaw(source["active_hours"]); len(got) != 0 {
		t.Fatalf("source active_hours=%v, want empty", got)
	}
	wantTarget := []scheduleEntry{{Day: 2, StartMin: 18 * 60, EndMin: 19 * 60}}
	if got := scheduleEntriesFromRaw(target["active_hours"]); !reflect.DeepEqual(got, wantTarget) {
		t.Fatalf("target active_hours=%v, want %v", got, wantTarget)
	}
}

func TestPersistSlashScheduleMoveRollbackSkipsRefreshOnFailure(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"profiles": {
			{
				"id":         "user-1",
				"profile_no": 1,
				"name":       "Default",
				"active_hours": encodeScheduleEntries([]scheduleEntry{
					{Day: 1, StartMin: 9 * 60, EndMin: 10 * 60},
				}),
			},
			{
				"id":           "user-1",
				"profile_no":   2,
				"name":         "Night",
				"active_hours": "[]",
			},
		},
	}, 2)

	profiles, err := env.query.SelectAllQuery("profiles", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	original := scheduleEntry{ProfileNo: 1, Day: 1, StartMin: 9 * 60, EndMin: 10 * 60}
	updates, errText := buildScheduleEditAssignUpdates(profiles, profileRowByNo(profiles, 2), original, 2, 18*60, 19*60)
	if errText != "" {
		t.Fatalf("build move updates errText=%q", errText)
	}

	if err := env.discord.persistSlashScheduleUpdates("user-1", updates); err == nil {
		t.Fatalf("expected persist failure")
	}

	env.assertNoRefresh(t)

	source, err := env.query.SelectOneQuery("profiles", map[string]any{"id": "user-1", "profile_no": 1})
	if err != nil {
		t.Fatalf("select source: %v", err)
	}
	target, err := env.query.SelectOneQuery("profiles", map[string]any{"id": "user-1", "profile_no": 2})
	if err != nil {
		t.Fatalf("select target: %v", err)
	}
	wantSource := []scheduleEntry{{Day: 1, StartMin: 9 * 60, EndMin: 10 * 60}}
	if got := scheduleEntriesFromRaw(source["active_hours"]); !reflect.DeepEqual(got, wantSource) {
		t.Fatalf("source active_hours=%v, want %v", got, wantSource)
	}
	if got := scheduleEntriesFromRaw(target["active_hours"]); len(got) != 0 {
		t.Fatalf("target active_hours=%v, want empty", got)
	}
}

type slashMutationTestEnv struct {
	discord   *Discord
	query     *db.Query
	refreshes int32
	refreshCh chan struct{}
}

func newSlashMutationTestEnv(t *testing.T, seed map[string][]map[string]any, failExecAt int) *slashMutationTestEnv {
	t.Helper()
	sqlDB := openBotTxStubDB(t, seed, failExecAt)
	query := db.NewQuery(sqlDB)
	env := &slashMutationTestEnv{
		query:     query,
		refreshCh: make(chan struct{}, 8),
	}
	processor := &webhook.Processor{}
	processor.SetAlertStateLoader(func() (*alertstate.Snapshot, error) {
		atomic.AddInt32(&env.refreshes, 1)
		select {
		case env.refreshCh <- struct{}{}:
		default:
		}
		return &alertstate.Snapshot{
			Tables:       map[string][]map[string]any{},
			Humans:       map[string]map[string]any{},
			Profiles:     map[string]map[string]any{},
			HasSchedules: map[string]bool{},
		}, nil
	})
	manager := &Manager{
		query:     query,
		processor: processor,
	}
	env.discord = &Discord{manager: manager}
	return env
}

func (e *slashMutationTestEnv) waitForRefresh(t *testing.T, want int32) {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for atomic.LoadInt32(&e.refreshes) < want {
		select {
		case <-e.refreshCh:
		case <-timeout:
			t.Fatalf("refresh count=%d, want at least %d", atomic.LoadInt32(&e.refreshes), want)
		}
	}
	time.Sleep(20 * time.Millisecond)
	if got := atomic.LoadInt32(&e.refreshes); got != want {
		t.Fatalf("refresh count=%d, want %d", got, want)
	}
}

func (e *slashMutationTestEnv) assertNoRefresh(t *testing.T) {
	t.Helper()
	select {
	case <-e.refreshCh:
		t.Fatalf("unexpected alert-state refresh")
	case <-time.After(150 * time.Millisecond):
	}
	if got := atomic.LoadInt32(&e.refreshes); got != 0 {
		t.Fatalf("refresh count=%d, want 0", got)
	}
}

var (
	botTxStubRegisterOnce sync.Once
	botTxStubDSNSeq       int64
	botTxStubStates       sync.Map
)

func openBotTxStubDB(t *testing.T, seed map[string][]map[string]any, failExecAt int) *sql.DB {
	t.Helper()
	botTxStubRegisterOnce.Do(func() {
		sql.Register("poracle-bot-txstub", botTxStubDriver{})
	})

	state := newBotTxStubState(seed, failExecAt)
	dsn := fmt.Sprintf("bot-txstub-%d", atomic.AddInt64(&botTxStubDSNSeq, 1))
	botTxStubStates.Store(dsn, state)
	t.Cleanup(func() {
		botTxStubStates.Delete(dsn)
	})

	db, err := sql.Open("poracle-bot-txstub", dsn)
	if err != nil {
		t.Fatalf("open bot tx stub db: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

type botTxStubDriver struct{}

func (botTxStubDriver) Open(name string) (driver.Conn, error) {
	value, ok := botTxStubStates.Load(name)
	if !ok {
		return nil, fmt.Errorf("unknown bot tx stub dsn %q", name)
	}
	return &botTxStubConn{state: value.(*botTxStubState)}, nil
}

type botTxStubState struct {
	mu         sync.Mutex
	tables     map[string][]map[string]any
	execCount  int
	failExecAt int
}

func newBotTxStubState(seed map[string][]map[string]any, failExecAt int) *botTxStubState {
	return &botTxStubState{
		tables:     cloneBotTables(seed),
		failExecAt: failExecAt,
	}
}

type botTxStubConn struct {
	state    *botTxStubState
	inTx     bool
	txTables map[string][]map[string]any
}

func (c *botTxStubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}

func (c *botTxStubConn) Close() error { return nil }

func (c *botTxStubConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *botTxStubConn) BeginTx(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if c.inTx {
		return nil, errors.New("transaction already open")
	}
	c.inTx = true
	c.txTables = cloneBotTables(c.state.tables)
	return &botTxStubTx{conn: c}, nil
}

func (c *botTxStubConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	plainArgs := namedBotValuesToAny(args)
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.execCount++
	if c.state.failExecAt > 0 && c.state.execCount == c.state.failExecAt {
		return nil, errors.New("forced exec failure")
	}
	tables := c.tables()
	affected, err := applyBotExec(tables, query, plainArgs)
	if err != nil {
		return nil, err
	}
	return botTxStubResult(affected), nil
}

func (c *botTxStubConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	plainArgs := namedBotValuesToAny(args)
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	rows, columns, err := applyBotQuery(c.tables(), query, plainArgs)
	if err != nil {
		return nil, err
	}
	return &botTxStubRows{columns: columns, rows: rows}, nil
}

func (c *botTxStubConn) Ping(_ context.Context) error { return nil }

func (c *botTxStubConn) tables() map[string][]map[string]any {
	if c.inTx {
		return c.txTables
	}
	return c.state.tables
}

type botTxStubTx struct {
	conn *botTxStubConn
}

func (tx *botTxStubTx) Commit() error {
	tx.conn.state.mu.Lock()
	defer tx.conn.state.mu.Unlock()
	if !tx.conn.inTx {
		return errors.New("transaction closed")
	}
	tx.conn.state.tables = cloneBotTables(tx.conn.txTables)
	tx.conn.txTables = nil
	tx.conn.inTx = false
	return nil
}

func (tx *botTxStubTx) Rollback() error {
	tx.conn.state.mu.Lock()
	defer tx.conn.state.mu.Unlock()
	tx.conn.txTables = nil
	tx.conn.inTx = false
	return nil
}

type botTxStubRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *botTxStubRows) Columns() []string { return r.columns }
func (r *botTxStubRows) Close() error      { return nil }

func (r *botTxStubRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

type botTxStubResult int64

func (r botTxStubResult) LastInsertId() (int64, error) { return 0, nil }
func (r botTxStubResult) RowsAffected() (int64, error) { return int64(r), nil }

func namedBotValuesToAny(values []driver.NamedValue) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value.Value)
	}
	return out
}

func cloneBotTables(input map[string][]map[string]any) map[string][]map[string]any {
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

func applyBotExec(tables map[string][]map[string]any, query string, args []any) (int64, error) {
	switch {
	case strings.HasPrefix(strings.ToUpper(query), "INSERT INTO "):
		return applyBotInsert(tables, query, args)
	case strings.HasPrefix(strings.ToUpper(query), "UPDATE "):
		return applyBotUpdate(tables, query, args)
	case strings.HasPrefix(strings.ToUpper(query), "DELETE FROM "):
		return applyBotDelete(tables, query, args)
	default:
		return 0, fmt.Errorf("unsupported exec query: %s", query)
	}
}

func applyBotQuery(tables map[string][]map[string]any, query string, args []any) ([][]driver.Value, []string, error) {
	upper := strings.ToUpper(query)
	switch {
	case strings.HasPrefix(upper, "SELECT COUNT(*) FROM "):
		table, where, err := parseBotSelect(query, "SELECT COUNT(*) FROM ")
		if err != nil {
			return nil, nil, err
		}
		matches, err := filterBotRows(tables[table], where, args)
		if err != nil {
			return nil, nil, err
		}
		return [][]driver.Value{{int64(len(matches))}}, []string{"COUNT(*)"}, nil
	case strings.HasPrefix(upper, "SELECT * FROM "):
		table, where, err := parseBotSelect(query, "SELECT * FROM ")
		if err != nil {
			return nil, nil, err
		}
		matches, err := filterBotRows(tables[table], where, args)
		if err != nil {
			return nil, nil, err
		}
		columns := []string{}
		if len(matches) > 0 {
			columns = sortedBotKeys(matches[0])
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

func parseBotSelect(query, prefix string) (string, string, error) {
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

func applyBotInsert(tables map[string][]map[string]any, query string, args []any) (int64, error) {
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
		columns = append(columns, trimBotIdent(token))
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

func applyBotUpdate(tables map[string][]map[string]any, query string, args []any) (int64, error) {
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
		setColumns = append(setColumns, trimBotIdent(column))
	}
	if len(args) < len(setColumns) {
		return 0, fmt.Errorf("invalid update args for %q", query)
	}
	setArgs := args[:len(setColumns)]
	whereArgs := args[len(setColumns):]
	affected := int64(0)
	for _, row := range tables[table] {
		matched, err := botRowMatches(row, wherePart, whereArgs)
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

func applyBotDelete(tables map[string][]map[string]any, query string, args []any) (int64, error) {
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
		matched, err := botRowMatches(row, where, args)
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

func filterBotRows(rows []map[string]any, where string, args []any) ([]map[string]any, error) {
	if strings.TrimSpace(where) == "" {
		return append([]map[string]any{}, rows...), nil
	}
	out := []map[string]any{}
	for _, row := range rows {
		matched, err := botRowMatches(row, where, args)
		if err != nil {
			return nil, err
		}
		if matched {
			out = append(out, row)
		}
	}
	return out, nil
}

func botRowMatches(row map[string]any, where string, args []any) (bool, error) {
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
			column := trimBotIdent(parts[0])
			count := strings.Count(parts[1], "?")
			if localArgIndex+count > len(args) {
				return false, fmt.Errorf("missing IN args for %q", clause)
			}
			values := args[localArgIndex : localArgIndex+count]
			localArgIndex += count
			if !containsBotValue(values, row[column]) {
				return false, nil
			}
		case strings.Contains(clause, " = ?"):
			column := trimBotIdent(strings.TrimSuffix(clause, " = ?"))
			if localArgIndex >= len(args) {
				return false, fmt.Errorf("missing arg for %q", clause)
			}
			if !sameBotValue(row[column], args[localArgIndex]) {
				return false, nil
			}
			localArgIndex++
		default:
			return false, fmt.Errorf("unsupported where clause %q", clause)
		}
	}
	return true, nil
}

func containsBotValue(values []any, target any) bool {
	for _, value := range values {
		if sameBotValue(value, target) {
			return true
		}
	}
	return false
}

func sameBotValue(left, right any) bool {
	return fmt.Sprint(left) == fmt.Sprint(right)
}

func trimBotIdent(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`")
	return value
}

func sortedBotKeys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
