package db

import "database/sql"

// Open opens a DB connection using the provided driver and dsn.
func Open(driver, dsn string) (*sql.DB, error) {
    return sql.Open(driver, dsn)
}
