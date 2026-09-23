package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
