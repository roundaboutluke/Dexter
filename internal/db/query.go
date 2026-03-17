package db

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// Query provides database helpers similar to PoracleJS.
type Query struct {
	db         *sql.DB
	tx         *sql.Tx
	postCommit *[]func()
}

// NewQuery constructs a query helper.
func NewQuery(conn *sql.DB) *Query {
	return &Query{db: conn}
}

type queryRunner interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

func (q *Query) runner() queryRunner {
	if q == nil {
		return nil
	}
	if q.tx != nil {
		return q.tx
	}
	return q.db
}

// WithTx runs fn inside a transaction. Nested calls reuse the active transaction.
func (q *Query) WithTx(ctx context.Context, fn func(*Query) error) error {
	if q == nil || q.db == nil {
		return fmt.Errorf("withTx: database not initialized")
	}
	if fn == nil {
		return nil
	}
	if q.tx != nil {
		return fn(q)
	}
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("withTx: %w", err)
	}
	callbacks := []func(){}
	txQuery := &Query{
		db:         q.db,
		tx:         tx,
		postCommit: &callbacks,
	}
	if err := fn(txQuery); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("withTx: %w", err)
	}
	for _, cb := range callbacks {
		if cb != nil {
			cb()
		}
	}
	return nil
}

// AfterCommit registers a callback to run after the surrounding transaction commits.
// When no transaction is active, the callback runs immediately.
func (q *Query) AfterCommit(fn func()) {
	if q == nil || fn == nil {
		return
	}
	if q.tx == nil || q.postCommit == nil {
		fn()
		return
	}
	*q.postCommit = append(*q.postCommit, fn)
}

// SelectOneQuery returns the first row matching conditions.
func (q *Query) SelectOneQuery(table string, conditions map[string]any) (map[string]any, error) {
	rows, err := q.SelectAllQuery(table, conditions)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// SelectAllQuery returns all rows matching conditions.
func (q *Query) SelectAllQuery(table string, conditions map[string]any) ([]map[string]any, error) {
	if q == nil || q.runner() == nil {
		return nil, fmt.Errorf("selectAllQuery: database not initialized")
	}
	whereSQL, args := buildWhere(conditions)
	query := fmt.Sprintf("SELECT * FROM %s%s", table, whereSQL)
	rows, err := q.runner().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("selectAllQuery: %w", err)
	}
	defer rows.Close()
	return rowsToMaps(rows)
}

// SelectAllQueryLimit returns up to limit rows matching conditions.
// A non-positive limit behaves like SelectAllQuery.
func (q *Query) SelectAllQueryLimit(table string, conditions map[string]any, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		return q.SelectAllQuery(table, conditions)
	}
	if q == nil || q.runner() == nil {
		return nil, fmt.Errorf("selectAllQueryLimit: database not initialized")
	}
	whereSQL, args := buildWhere(conditions)
	query := fmt.Sprintf("SELECT * FROM %s%s LIMIT ?", table, whereSQL)
	args = append(args, limit)
	rows, err := q.runner().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("selectAllQueryLimit: %w", err)
	}
	defer rows.Close()
	return rowsToMaps(rows)
}

// SelectWhereInQuery returns rows where column IN (values).
func (q *Query) SelectWhereInQuery(table string, values []any, valuesColumn string) ([]map[string]any, error) {
	if len(values) == 0 {
		return []map[string]any{}, nil
	}
	runner := q.runner()
	if runner == nil {
		return nil, fmt.Errorf("selectWhereInQuery: database not initialized")
	}
	placeholders := strings.Repeat("?,", len(values))
	placeholders = strings.TrimSuffix(placeholders, ",")
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s IN (%s)", table, quoteIdent(valuesColumn), placeholders)
	rows, err := runner.Query(query, values...)
	if err != nil {
		return nil, fmt.Errorf("selectWhereInQuery: %w", err)
	}
	defer rows.Close()
	return rowsToMaps(rows)
}

// SelectWhereInLikeQuery returns rows where likeColumn LIKE value and inColumn IN (values).
func (q *Query) SelectWhereInLikeQuery(table, likeColumn, likeValue, inColumn string, inValues []any) ([]map[string]any, error) {
	if len(inValues) == 0 {
		return []map[string]any{}, nil
	}
	runner := q.runner()
	if runner == nil {
		return nil, fmt.Errorf("selectWhereInLikeQuery: database not initialized")
	}
	placeholders := strings.Repeat("?,", len(inValues))
	placeholders = strings.TrimSuffix(placeholders, ",")
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s LIKE ? AND %s IN (%s)", table, quoteIdent(likeColumn), quoteIdent(inColumn), placeholders)
	args := make([]any, 0, len(inValues)+1)
	args = append(args, "%"+likeValue+"%")
	args = append(args, inValues...)
	rows, err := runner.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("selectWhereInLikeQuery: %w", err)
	}
	defer rows.Close()
	return rowsToMaps(rows)
}

// UpdateQuery updates rows matching conditions.
func (q *Query) UpdateQuery(table string, values map[string]any, conditions map[string]any) (int64, error) {
	if len(values) == 0 {
		return 0, nil
	}
	runner := q.runner()
	if runner == nil {
		return 0, fmt.Errorf("updateQuery: database not initialized")
	}
	setSQL, setArgs := buildSet(values)
	whereSQL, whereArgs := buildWhere(conditions)
	query := fmt.Sprintf("UPDATE %s%s%s", table, setSQL, whereSQL)
	res, err := runner.Exec(query, append(setArgs, whereArgs...)...)
	if err != nil {
		return 0, fmt.Errorf("updateQuery: %w", err)
	}
	return res.RowsAffected()
}

// IncrementQuery increments a numeric column by value for matching rows.
func (q *Query) IncrementQuery(table string, where map[string]any, target string, value int64) (int64, error) {
	runner := q.runner()
	if runner == nil {
		return 0, fmt.Errorf("incrementQuery: database not initialized")
	}
	whereSQL, whereArgs := buildWhere(where)
	targetIdent := quoteIdent(target)
	query := fmt.Sprintf("UPDATE %s SET %s = %s + ?%s", table, targetIdent, targetIdent, whereSQL)
	args := append([]any{value}, whereArgs...)
	res, err := runner.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("incrementQuery: %w", err)
	}
	return res.RowsAffected()
}

// CountQuery returns a count of rows matching conditions.
func (q *Query) CountQuery(table string, conditions map[string]any) (int64, error) {
	runner := q.runner()
	if runner == nil {
		return 0, fmt.Errorf("countQuery: database not initialized")
	}
	whereSQL, args := buildWhere(conditions)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s%s", table, whereSQL)
	row := runner.QueryRow(query, args...)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("countQuery: %w", err)
	}
	return count, nil
}

// CountGroupedQuery returns row counts grouped by a column.
// The result maps group key values to their counts.
func (q *Query) CountGroupedQuery(table string, conditions map[string]any, groupBy string) (map[int]int64, error) {
	runner := q.runner()
	if runner == nil {
		return nil, fmt.Errorf("countGroupedQuery: database not initialized")
	}
	whereSQL, args := buildWhere(conditions)
	col := quoteIdent(groupBy)
	query := fmt.Sprintf("SELECT %s, COUNT(*) FROM %s%s GROUP BY %s", col, table, whereSQL, col)
	rows, err := runner.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("countGroupedQuery: %w", err)
	}
	defer rows.Close()
	result := map[int]int64{}
	for rows.Next() {
		var key int
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			return nil, fmt.Errorf("countGroupedQuery scan: %w", err)
		}
		result[key] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("countGroupedQuery rows: %w", err)
	}
	return result, nil
}

// InsertQuery inserts one or more rows.
func (q *Query) InsertQuery(table string, values any) (int64, error) {
	table = strings.TrimSpace(table)
	rows := normalizeRows(values)
	if len(rows) == 0 {
		return 0, nil
	}
	runner := q.runner()
	if runner == nil {
		return 0, fmt.Errorf("insertQuery: database not initialized")
	}
	switch strings.ToLower(table) {
	case "monsters", "raid", "egg", "quest", "invasion", "weather", "lures", "gym", "nests", "forts":
		for _, row := range rows {
			if value, ok := row["ping"]; !ok || value == nil {
				row["ping"] = ""
			}
		}
	}
	columns := sortedKeys(rows[0])
	quotedColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		quotedColumns = append(quotedColumns, quoteIdent(column))
	}
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	valueGroups := make([]string, 0, len(rows))
	args := make([]any, 0, len(rows)*len(columns))
	for _, row := range rows {
		valueGroups = append(valueGroups, fmt.Sprintf("(%s)", strings.Join(placeholders, ",")))
		for _, key := range columns {
			args = append(args, row[key])
		}
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", table, strings.Join(quotedColumns, ","), strings.Join(valueGroups, ","))
	res, err := runner.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("insertQuery: %w", err)
	}
	return res.RowsAffected()
}

// ExecQuery runs a non-select statement and returns affected rows.
func (q *Query) ExecQuery(sqlQuery string, args ...any) (int64, error) {
	runner := q.runner()
	if runner == nil {
		return 0, fmt.Errorf("execQuery: database not initialized")
	}
	res, err := runner.Exec(sqlQuery, args...)
	if err != nil {
		return 0, fmt.Errorf("execQuery: %w", err)
	}
	return res.RowsAffected()
}

// MysteryQuery runs a raw SQL query and returns rows.
// Pass args to use parameterized placeholders and avoid SQL injection.
func (q *Query) MysteryQuery(sqlQuery string, args ...any) ([]map[string]any, error) {
	runner := q.runner()
	if runner == nil {
		return nil, fmt.Errorf("mysteryQuery: database not initialized")
	}
	rows, err := runner.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("mysteryQuery: %w", err)
	}
	defer rows.Close()
	return rowsToMaps(rows)
}

// DeleteWhereInQuery deletes rows by IN clause and optional id constraint.
func (q *Query) DeleteWhereInQuery(table string, id any, values []any, valuesColumn string) (int64, error) {
	if len(values) == 0 {
		return 0, nil
	}
	runner := q.runner()
	if runner == nil {
		return 0, fmt.Errorf("deleteWhereInQuery: database not initialized")
	}
	placeholders := strings.Repeat("?,", len(values))
	placeholders = strings.TrimSuffix(placeholders, ",")
	args := make([]any, 0, len(values)+4)
	args = append(args, values...)

	conditions := map[string]any{}
	if id != nil {
		switch v := id.(type) {
		case map[string]any:
			for key, val := range v {
				conditions[key] = val
			}
		default:
			conditions["id"] = v
		}
	}
	whereSQL, whereArgs := buildWhere(conditions)
	args = append(args, whereArgs...)
	if whereSQL != "" {
		whereSQL = " AND " + strings.TrimPrefix(whereSQL, " WHERE ")
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)%s", table, quoteIdent(valuesColumn), placeholders, whereSQL)
	res, err := runner.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("deleteWhereInQuery: %w", err)
	}
	return res.RowsAffected()
}

// InsertOrUpdateQuery inserts rows if not already present (mysql behavior in PoracleJS).
func (q *Query) InsertOrUpdateQuery(table string, values any) (int64, error) {
	rows := normalizeRows(values)
	var rowsAffected int64
	for _, row := range rows {
		count, err := q.CountQuery(table, row)
		if err != nil {
			return rowsAffected, err
		}
		if count == 0 {
			affected, err := q.InsertQuery(table, row)
			if err != nil {
				return rowsAffected, err
			}
			rowsAffected += affected
		}
	}
	return rowsAffected, nil
}

// DeleteQuery deletes rows matching values.
func (q *Query) DeleteQuery(table string, values map[string]any) (int64, error) {
	runner := q.runner()
	if runner == nil {
		return 0, fmt.Errorf("deleteQuery: database not initialized")
	}
	whereSQL, args := buildWhere(values)
	query := fmt.Sprintf("DELETE FROM %s%s", table, whereSQL)
	res, err := runner.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("deleteQuery: %w", err)
	}
	return res.RowsAffected()
}

func buildWhere(conditions map[string]any) (string, []any) {
	if len(conditions) == 0 {
		return "", nil
	}
	keys := sortedKeys(conditions)
	parts := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s = ?", quoteIdent(key)))
		args = append(args, conditions[key])
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

func buildSet(values map[string]any) (string, []any) {
	keys := sortedKeys(values)
	parts := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s = ?", quoteIdent(key)))
		args = append(args, values[key])
	}
	return " SET " + strings.Join(parts, ", "), args
}

func quoteIdent(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return key
	}
	// Escape embedded backticks by doubling them, then wrap in backticks.
	escaped := strings.ReplaceAll(key, "`", "``")
	return "`" + escaped + "`"
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizeRows(values any) []map[string]any {
	switch v := values.(type) {
	case nil:
		return nil
	case []map[string]any:
		return v
	case map[string]any:
		return []map[string]any{v}
	case []any:
		rows := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if row, ok := item.(map[string]any); ok {
				rows = append(rows, row)
			}
		}
		return rows
	default:
		return nil
	}
}

func rowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	results := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		rowMap := make(map[string]any, len(cols))
		for i, col := range cols {
			switch v := values[i].(type) {
			case []byte:
				rowMap[col] = string(v)
			default:
				rowMap[col] = v
			}
		}
		results = append(results, rowMap)
	}
	return results, rows.Err()
}
