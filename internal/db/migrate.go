package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// RunMigrations executes SQL migrations in order from a directory.
func RunMigrations(ctx context.Context, conn *DB, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read migrations dir: %w", err)
	}

	if err := ensureMigrationsTable(ctx, conn.Conn); err != nil {
		return err
	}

	applied, err := loadAppliedMigrations(ctx, conn.Conn)
	if err != nil {
		return err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".sql") {
			files = append(files, filepath.Join(dir, name))
		}
	}

	if len(files) == 0 {
		return nil
	}

	sort.Strings(files)

	for _, path := range files {
		name := filepath.Base(path)
		if applied[name] {
			continue
		}
		if err := runMigrationFile(ctx, conn, path); err != nil {
			// When pointing PoracleGo at an existing PoracleJS database, early migrations may have already
			// been applied. MySQL/MariaDB then error with duplicate column/table/index messages.
			// Treat these as "already applied" and move on so later migrations can still run.
			if !isMySQLAlreadyAppliedError(err) {
				return err
			}
		}
		if err := recordMigration(ctx, conn.Conn, name); err != nil {
			return err
		}
	}

	return nil
}

func isMySQLAlreadyAppliedError(err error) bool {
	if err == nil {
		return false
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1050: // Table already exists
			return true
		case 1060: // Duplicate column name
			return true
		case 1061: // Duplicate key name (index)
			return true
		case 1091: // Can't DROP ...; check that it exists
			return true
		}
	}
	return false
}

func runMigrationFile(ctx context.Context, conn *DB, path string) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", path, err)
	}

	if strings.TrimSpace(string(payload)) == "" {
		return nil
	}

	tx, err := conn.Conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}

	if _, err := tx.ExecContext(ctx, string(payload)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("exec migration %s: %w", path, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", path, err)
	}

	return nil
}

// EnsureMigrationsDir keeps the migrations folder present even before SQL is ported.
func EnsureMigrationsDir(dir string) error {
	return os.MkdirAll(dir, fs.ModePerm)
}

func ensureMigrationsTable(ctx context.Context, conn *sql.DB) error {
	_, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS migrations (
			name VARCHAR(255) NOT NULL PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}
	return nil
}

func loadAppliedMigrations(ctx context.Context, conn *sql.DB) (map[string]bool, error) {
	rows, err := conn.QueryContext(ctx, "SELECT name FROM migrations")
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan migration: %w", err)
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

func recordMigration(ctx context.Context, conn *sql.DB, name string) error {
	_, err := conn.ExecContext(ctx, "INSERT INTO migrations (name) VALUES (?)", name)
	if err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	return nil
}
