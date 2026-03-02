package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

const defaultDBPath = "audicatalog.db"

// OpenSQLite opens SQLite, applies required pragmas, and runs migrations.
func OpenSQLite(ctx context.Context, dbPath string) (*sql.DB, error) {
	return openSQLite(ctx, dbPath, false)
}

// OpenSQLiteReadOnly opens SQLite in read-only mode without applying migrations.
func OpenSQLiteReadOnly(ctx context.Context, dbPath string) (*sql.DB, error) {
	return openSQLite(ctx, dbPath, true)
}

func openSQLite(ctx context.Context, dbPath string, readOnly bool) (*sql.DB, error) {
	path := strings.TrimSpace(dbPath)
	if path == "" {
		path = defaultDBPath
	}

	dsn := path
	if readOnly {
		dsn = fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", url.PathEscape(path))
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}

	if readOnly {
		if err := applyReadOnlyPragmas(ctx, db); err != nil {
			_ = db.Close()
			return nil, err
		}
		return db, nil
	}

	if err := applyPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate sqlite db: %w", err)
	}

	return db, nil
}

func applyPragmas(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA foreign_keys=ON;`,
		`PRAGMA busy_timeout=5000;`,
	}

	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("apply %q: %w", pragma, err)
		}
	}

	return nil
}

func applyReadOnlyPragmas(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		`PRAGMA foreign_keys=ON;`,
		`PRAGMA query_only=ON;`,
	}

	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("apply %q: %w", pragma, err)
		}
	}

	return nil
}
