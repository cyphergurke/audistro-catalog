package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate applies all embedded SQL migrations in version order.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	type migration struct {
		version int
		name    string
	}
	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".sql" {
			continue
		}

		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			return fmt.Errorf("invalid migration name: %s", name)
		}
		version, convErr := strconv.Atoi(parts[0])
		if convErr != nil {
			return fmt.Errorf("invalid migration version %q: %w", parts[0], convErr)
		}
		migrations = append(migrations, migration{version: version, name: name})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	for _, m := range migrations {
		applied, err := isMigrationApplied(ctx, db, m.version)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", m.version, err)
		}
		if applied {
			continue
		}

		sqlBytes, err := migrationFiles.ReadFile(filepath.Join("migrations", m.name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", m.name, err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", m.name, err)
		}

		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}

		stmt, err := tx.PrepareContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("prepare insert migration %s: %w", m.name, err)
		}
		_, err = stmt.ExecContext(ctx, m.version, time.Now().Unix())
		_ = stmt.Close()
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert migration version %d: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.name, err)
		}
	}

	return nil
}

func isMigrationApplied(ctx context.Context, db *sql.DB, version int) (bool, error) {
	stmt, err := db.PrepareContext(ctx, `SELECT 1 FROM schema_migrations WHERE version = ?`)
	if err != nil {
		return false, fmt.Errorf("prepare migration lookup: %w", err)
	}
	defer stmt.Close()

	var marker int
	err = stmt.QueryRowContext(ctx, version).Scan(&marker)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query migration lookup: %w", err)
	}
	return true, nil
}
