package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/timbasel/genetec-technical-assessment/internal/books"
)

func (s *Store) GetBookHistory(ctx context.Context, bookID string, query books.Query) (books.Page, error) {
	if query.Limit < 1 || query.Limit > 100 {
		return books.Page{}, fmt.Errorf("%w: `limit` must be between 1 and 100", books.ErrInvalidHistoryQuery)
	}
	if query.Offset < 0 {
		return books.Page{}, fmt.Errorf("%w: `offset` must be non-negative", books.ErrInvalidHistoryQuery)
	}
	if query.Order != "asc" && query.Order != "desc" {
		return books.Page{}, fmt.Errorf("%w: `order` must be asc or desc", books.ErrInvalidHistoryQuery)
	}

	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return books.Page{}, fmt.Errorf("begin history transaction: %w", err)
	}
	defer tx.Rollback()

	var found int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM books WHERE id = ?", bookID).Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return books.Page{}, fmt.Errorf("get book history: %w", books.ErrNotFound)
		}
		return books.Page{}, fmt.Errorf("check book for history: %w", err)
	}

	var filters strings.Builder
	filters.WriteString(" FROM books_history WHERE book_id = ?")
	args := []any{bookID}
	if query.Kind != "" {
		filters.WriteString(" AND kind = ?")
		args = append(args, query.Kind)
	}
	if query.Field != "" {
		filters.WriteString(" AND field = ?")
		args = append(args, query.Field)
	}
	if query.From != nil {
		filters.WriteString(" AND occurred_at >= ?")
		args = append(args, query.From.UnixNano())
	}
	if query.To != nil {
		filters.WriteString(" AND occurred_at <= ?")
		args = append(args, query.To.UnixNano())
	}

	page := books.Page{Items: []books.Change{}, Limit: query.Limit, Offset: query.Offset}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*)"+filters.String(), args...).Scan(&page.Total); err != nil {
		return books.Page{}, fmt.Errorf("count book history: %w", err)
	}

	order := "ASC"
	if query.Order == "desc" {
		order = "DESC"
	}
	statement := `SELECT id, book_id, occurred_at, kind, field, old_value, new_value, description` +
		filters.String() + ` ORDER BY occurred_at ` + order + `, id ` + order + ` LIMIT ? OFFSET ?`
	rows, err := tx.QueryContext(ctx, statement, append(args, query.Limit, query.Offset)...)
	if err != nil {
		return books.Page{}, fmt.Errorf("query book history: %w", err)
	}
	for rows.Next() {
		var change books.Change
		if err := scanBookChange(rows, &change); err != nil {
			rows.Close()
			return books.Page{}, fmt.Errorf("scan book history: %w", err)
		}
		page.Items = append(page.Items, change)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return books.Page{}, fmt.Errorf("read book history: %w", err)
	}
	if err := rows.Close(); err != nil {
		return books.Page{}, fmt.Errorf("close book history rows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return books.Page{}, fmt.Errorf("commit history transaction: %w", err)
	}
	return page, nil
}

func insertBookChange(ctx context.Context, tx *sql.Tx, bookID string, change books.Change) error {
	var oldValue any
	if change.OldValue != nil {
		encoded, err := json.Marshal(change.OldValue)
		if err != nil {
			return fmt.Errorf("encode old `%s` value: %w", change.Field, err)
		}
		oldValue = encoded
	}

	newValue, err := json.Marshal(change.NewValue)
	if err != nil {
		return fmt.Errorf("encode new `%s` value: %w", change.Field, err)
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
		change.OldValue, err = decodeBookChangeValue(change.Field, []byte(oldValue.String))
		if err != nil {
			return fmt.Errorf("decode old `%s` value: %w", change.Field, err)
		}
	}
	change.NewValue, err = decodeBookChangeValue(change.Field, []byte(newValue))
	if err != nil {
		return fmt.Errorf("decode new `%s` value: %w", change.Field, err)
	}

	return nil
}

func decodeBookChangeValue(field string, value []byte) (any, error) {
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
		return nil, fmt.Errorf("unknown `field` value %q", field)
	}
}
