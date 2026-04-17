package db

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open opens a PostgreSQL connection using the pgx driver via database/sql.
func Open(dsn string) (*sql.DB, error) {
	return sql.Open("pgx", dsn)
}
