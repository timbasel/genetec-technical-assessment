package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	*sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), os.ModePerm); err != nil {
		return nil, err
	}

	dsn := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	pragmas := url.Values{}
	for _, pragma := range []string{"foreign_keys(1)", "busy_timeout(1000)", "journal_mode(WAL)", "synchronous(FULL)"} {
		pragmas.Add("_pragma", pragma)
	}
	dsn.RawQuery = pragmas.Encode()

	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("failed to open store: %w", err)
	}
	store := &Store{DB: db}

	if err := migrate(ctx, store); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate store: %w", err)
	}

	return store, nil
}

//go:embed migrations/*.sql
var migrations embed.FS

func migrate(ctx context.Context, store *Store) error {
	if _, err := store.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY)"); err != nil {
		return err
	}

	files, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return err
	}

	for _, file := range files {
		var applied int
		err := store.QueryRowContext(ctx, "SELECT 1 FROM schema_migrations WHERE name=?", file.Name()).Scan(&applied)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		migration, err := migrations.ReadFile("migrations/" + file.Name())
		if err != nil {
			return err
		}

		tx, err := store.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		if _, err = tx.ExecContext(ctx, string(migration)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to apply migration %s: %w", file.Name(), err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(name) VALUES(?)", file.Name()); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to apply migration %s: %w", file.Name(), err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", file.Name(), err)
		}
	}

	return nil
}

type RowScanner interface {
	Scan(dest ...any) error
}
