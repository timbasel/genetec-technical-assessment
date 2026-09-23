package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/timbasel/genetec-technical-assessment/internal/books"
)

func (s *Store) CreateBook(ctx context.Context, book books.Book, changes []books.Change) error {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create book transaction: %w", err)
	}
	defer tx.Rollback()

	if err := insertBook(ctx, tx, book); err != nil {
		return fmt.Errorf("insert book: %w", err)
	}
	for _, change := range changes {
		if err := insertChange(ctx, tx, book.ID, change); err != nil {
			return fmt.Errorf("insert book change: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create book transaction: %w", err)
	}

	return nil
}

var ErrVersionConflict = books.ErrVersionConflict

func (s *Store) GetBook(ctx context.Context, id string) (books.Book, error) {
	var book books.Book
	err := scanBook(s.QueryRowContext(ctx, `
		SELECT id, title, description, publication_date, authors, version, created_at, updated_at
		FROM books WHERE id = ?`, id), &book)
	if errors.Is(err, sql.ErrNoRows) {
		return books.Book{}, fmt.Errorf("get book: %w", books.ErrNotFound)
	}
	if err != nil {
		return books.Book{}, fmt.Errorf("get book: %w", err)
	}
	return book, nil
}

func (s *Store) UpdateBook(ctx context.Context, book books.Book, changes []books.Change) error {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update book transaction: %w", err)
	}
	defer tx.Rollback()

	if err := updateBook(ctx, tx, book); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("update book: %w", books.ErrNotFound)
		}
		return fmt.Errorf("update book: %w", err)
	}
	for _, change := range changes {
		if err := insertChange(ctx, tx, book.ID, change); err != nil {
			return fmt.Errorf("insert book change: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update book transaction: %w", err)
	}
	return nil
}

func updateBook(ctx context.Context, tx *sql.Tx, book books.Book) error {
	authors, err := json.Marshal(book.Authors)
	if err != nil {
		return fmt.Errorf("encode authors: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE books
		SET title = ?, description = ?, publication_date = ?, authors = ?, version = ?, updated_at = ?
		WHERE id = ? AND version = ?`,
		book.Title,
		book.Description,
		book.PublicationDate,
		authors,
		book.Version,
		book.UpdatedAt.UnixNano(),
		book.ID,
		book.Version-1,
	)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 0 {
		return nil
	}

	var found int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM books WHERE id = ?", book.ID).Scan(&found)
	if err != nil {
		return err
	}
	return ErrVersionConflict
}

func insertBook(ctx context.Context, tx *sql.Tx, book books.Book) error {
	authors, err := json.Marshal(book.Authors)
	if err != nil {
		return fmt.Errorf("encode authors: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO books(id, title, description, publication_date, authors, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		book.ID,
		book.Title,
		book.Description,
		book.PublicationDate,
		authors,
		book.Version,
		book.CreatedAt.UnixNano(),
		book.UpdatedAt.UnixNano(),
	)
	return err
}

func insertChange(ctx context.Context, tx *sql.Tx, bookID string, change books.Change) error {
	var oldValue any
	if change.OldValue != nil {
		encoded, err := json.Marshal(change.OldValue)
		if err != nil {
			return fmt.Errorf("encode old %s value: %w", change.Field, err)
		}
		oldValue = encoded
	}

	newValue, err := json.Marshal(change.NewValue)
	if err != nil {
		return fmt.Errorf("encode new %s value: %w", change.Field, err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO books_history(book_id, occurred_at, kind, field, old_value, new_value, description)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		bookID,
		change.OccurredAt.UnixNano(),
		change.Kind,
		change.Field,
		oldValue,
		newValue,
		change.Description,
	)
	return err
}

func scanBook(row RowScanner, book *books.Book) error {
	var authors []byte
	var createdAt int64
	var updatedAt int64

	if err := row.Scan(
		&book.ID,
		&book.Title,
		&book.Description,
		&book.PublicationDate,
		&authors,
		&book.Version,
		&createdAt,
		&updatedAt,
	); err != nil {
		return err
	}

	var err error
	err = json.Unmarshal(authors, &book.Authors)
	if err != nil {
		return fmt.Errorf("failed to decode authors: %w", err)
	}
	book.CreatedAt = time.Unix(0, createdAt).UTC()
	book.UpdatedAt = time.Unix(0, updatedAt).UTC()

	return nil
}

func scanBookChange(row RowScanner, change *books.Change) error {
	var occurredAt int64
	var oldValue sql.NullString
	var newValue string

	if err := row.Scan(
		&change.ID,
		&change.BookID,
		&occurredAt,
		&change.Kind,
		&change.Field,
		&oldValue,
		&newValue,
		&change.Description,
	); err != nil {
		return err
	}

	change.OccurredAt = time.Unix(0, occurredAt).UTC()
	var err error
	if oldValue.Valid {
		change.OldValue, err = decodeChangeValue(change.Field, []byte(oldValue.String))
		if err != nil {
			return fmt.Errorf("decode old %s value: %w", change.Field, err)
		}
	}
	change.NewValue, err = decodeChangeValue(change.Field, []byte(newValue))
	if err != nil {
		return fmt.Errorf("decode new %s value: %w", change.Field, err)
	}

	return nil
}

func decodeChangeValue(field string, value []byte) (any, error) {
	switch field {
	case "title", "description", "publication_date":
		var decoded string
		if err := json.Unmarshal(value, &decoded); err != nil {
			return nil, err
		}
		return decoded, nil
	case "authors":
		var decoded []string
		if err := json.Unmarshal(value, &decoded); err != nil {
			return nil, err
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("unknown change field %q", field)
	}
}
