package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"poraclego/internal/config"
)

// DB wraps the SQL connection pool.
type DB struct {
	Conn *sql.DB
}

// New opens a database connection based on config.database settings.
func New(cfg *config.Config) (*DB, error) {
	client, _ := cfg.GetString("database.client")
	if client == "" {
		client = "mysql"
	}
	if client != "mysql" {
		return nil, fmt.Errorf("unsupported database client: %s", client)
	}

	dsn, err := mysqlDSN(cfg)
	if err != nil {
		return nil, err
	}

	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	maxConns, ok := cfg.GetInt("tuning.maxDatabaseConnections")
	if ok && maxConns > 0 {
		conn.SetMaxOpenConns(maxConns)
		conn.SetMaxIdleConns(maxConns)
	}

	// Connection lifetime defaults prevent stale connections from accumulating.
	connMaxLifetime := 30 * time.Minute
	if minutes, ok := cfg.GetInt("tuning.connMaxLifetimeMinutes"); ok && minutes > 0 {
		connMaxLifetime = time.Duration(minutes) * time.Minute
	}
	conn.SetConnMaxLifetime(connMaxLifetime)

	connMaxIdleTime := 10 * time.Minute
	if minutes, ok := cfg.GetInt("tuning.connMaxIdleTimeMinutes"); ok && minutes > 0 {
		connMaxIdleTime = time.Duration(minutes) * time.Minute
	}
	conn.SetConnMaxIdleTime(connMaxIdleTime)

	return &DB{Conn: conn}, nil
}

// Close shuts down the database pool.
func (db *DB) Close() error {
	if db == nil || db.Conn == nil {
		return nil
	}
	return db.Conn.Close()
}

func mysqlDSN(cfg *config.Config) (string, error) {
	user, _ := cfg.GetString("database.conn.user")
	pass, _ := cfg.GetString("database.conn.password")
	host, _ := cfg.GetString("database.conn.host")
	name, _ := cfg.GetString("database.conn.database")
	port, ok := cfg.GetInt("database.conn.port")
	if !ok {
		port = 3306
	}
	if host == "" || name == "" {
		return "", fmt.Errorf("database.conn.host and database.conn.database are required")
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&loc=Local&multiStatements=true", user, pass, host, port, name), nil
}
