package db

import (
	"database/sql"
	"os"
	"strconv"
	"time"

	// pgx stdlib driver — registers the "pgx" driver name with database/sql.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open opens a PostgreSQL connection pool using the pgx driver.
//
// Pool size is controlled via environment variables:
//
//	DB_MAX_OPEN_CONNS    (default 10)
//	DB_MAX_IDLE_CONNS    (default 5)
//	DB_CONN_MAX_LIFETIME (default 5m, Go duration string)
func Open(dsn string) (*sql.DB, error) {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(envInt("DB_MAX_OPEN_CONNS", 10))
	sqlDB.SetMaxIdleConns(envInt("DB_MAX_IDLE_CONNS", 5))
	sqlDB.SetConnMaxLifetime(envDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute))

	return sqlDB, nil
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
