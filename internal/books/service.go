package books

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"
)

var (
	ErrInvalidFields       = errors.New("invalid book fields")
	ErrInvalidHistoryQuery = errors.New("invalid history query")
	ErrNotFound            = errors.New("book not found")
	ErrVersionConflict     = errors.New("book version conflict")
)

type Fields struct {
	Title           string
	Description     string
	PublicationDate string
	Authors         []string
}

type Store interface {
	CreateBook(context.Context, Book, []Change) error
	GetBook(context.Context, string) (Book, error)
	UpdateBook(context.Context, Book, []Change) error
	GetBookHistory(context.Context, string, Query) (Page, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) CreateBook(ctx context.Context, fields Fields) (Book, error) {
	if err := validateFields(fields); err != nil {
		return Book{}, err
	}

	now := time.Now().UTC()
	book := Book{
		ID:              uuid.New().String(),
		Title:           fields.Title,
		Description:     fields.Description,
		PublicationDate: fields.PublicationDate,
		Authors:         slices.Clone(fields.Authors),
		Version:         1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	changes := creationChanges(book)
	if err := s.store.CreateBook(ctx, book, changes); err != nil {
		return Book{}, fmt.Errorf("create book: %w", err)
	}
	return book, nil
}

func (s *Service) GetBook(ctx context.Context, id string) (Book, error) {
	return s.store.GetBook(ctx, id)
}

func (s *Service) UpdateBook(ctx context.Context, id string, version int64, fields Fields) (Book, error) {
	if err := validateFields(fields); err != nil {
		return Book{}, err
	}
	if version < 1 {
		return Book{}, fmt.Errorf("%w: version must be positive", ErrInvalidFields)
	}

	before, err := s.store.GetBook(ctx, id)
	if err != nil {
		return Book{}, fmt.Errorf("failed to get book for update: %w", err)
	}
	if before.Version != version {
		return Book{}, ErrVersionConflict
	}

	after := before
	after.Title = fields.Title
	after.Description = fields.Description
	after.PublicationDate = fields.PublicationDate
	after.Authors = slices.Clone(fields.Authors)

	if before.Title == after.Title &&
		before.Description == after.Description &&
		before.PublicationDate == after.PublicationDate &&
		slices.Equal(before.Authors, after.Authors) {
		return before, nil
	}

	after.Version++
	after.UpdatedAt = time.Now().UTC()
	changes := updatedChanges(before, after)
	if err := s.store.UpdateBook(ctx, after, changes); err != nil {
		return Book{}, fmt.Errorf("update book: %w", err)
	}
	return after, nil
}

func (s *Service) GetBookHistory(ctx context.Context, id string, query Query) (Page, error) {
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit < 1 || query.Limit > 100 || query.Offset < 0 {
		return Page{}, fmt.Errorf("%w: limit must be 1 to 100 and offset must be non-negative", ErrInvalidHistoryQuery)
	}
	if query.Order == "" {
		query.Order = "desc"
	}
	if query.Order != "asc" && query.Order != "desc" {
		return Page{}, fmt.Errorf("%w: order must be asc or desc", ErrInvalidHistoryQuery)
	}
	if query.Kind != "" && query.Kind != "created" && query.Kind != "updated" {
		return Page{}, fmt.Errorf("%w: kind must be created or updated", ErrInvalidHistoryQuery)
	}
	switch query.Field {
	case "", "title", "description", "publication_date", "authors":
	default:
		return Page{}, fmt.Errorf("%w: invalid field", ErrInvalidHistoryQuery)
	}

	minTime := time.Unix(0, math.MinInt64)
	maxTime := time.Unix(0, math.MaxInt64)
	if query.From != nil && (query.From.Before(minTime) || query.From.After(maxTime)) {
		return Page{}, fmt.Errorf("%w: from is outside the supported timestamp range", ErrInvalidHistoryQuery)
	}
	if query.To != nil && (query.To.Before(minTime) || query.To.After(maxTime)) {
		return Page{}, fmt.Errorf("%w: to is outside the supported timestamp range", ErrInvalidHistoryQuery)
	}
	if query.From != nil && query.To != nil && query.From.After(*query.To) {
		return Page{}, fmt.Errorf("%w: from must not be after to", ErrInvalidHistoryQuery)
	}

	page, err := s.store.GetBookHistory(ctx, id, query)
	if err != nil {
		return Page{}, fmt.Errorf("get book history: %w", err)
	}
	return page, nil
}

func validateFields(fields Fields) error {
	if strings.TrimSpace(fields.Title) == "" {
		return fmt.Errorf("%w: title must not be empty", ErrInvalidFields)
	}
	if utf8.RuneCountInString(fields.Title) > 200 {
		return fmt.Errorf("%w: title exceeds 200 characters", ErrInvalidFields)
	}

	if utf8.RuneCountInString(fields.Description) > 2000 {
		return fmt.Errorf("%w: description exceeds 2000 characters", ErrInvalidFields)
	}

	date, err := time.Parse("2006-01-02", fields.PublicationDate)
	if err != nil || date.Format("2006-01-02") != fields.PublicationDate {
		return fmt.Errorf("%w: publication date must be YYYY-MM-DD", ErrInvalidFields)
	}

	if len(fields.Authors) < 1 {
		return fmt.Errorf("%w: provide at least 1 author", ErrInvalidFields)
	}
	seen := make(map[string]bool, len(fields.Authors))
	for _, author := range fields.Authors {
		if strings.TrimSpace(author) == "" || author != strings.TrimSpace(author) || utf8.RuneCountInString(author) > 200 {
			return fmt.Errorf("%w: each author must contain 1 to 200 characters without surrounding whitespace", ErrInvalidFields)
		}
		if seen[author] {
			return fmt.Errorf("%w: duplicate author %q", ErrInvalidFields, author)
		}
		seen[author] = true
	}
	return nil
}

func creationChanges(book Book) []Change {
	fields := []struct {
		name        string
		value       any
		description string
	}{
		{"title", book.Title, fmt.Sprintf("Title set to %q", book.Title)},
		{"description", book.Description, fmt.Sprintf("Description set to %q", book.Description)},
		{"publication_date", book.PublicationDate, fmt.Sprintf("Publication date set to %q", book.PublicationDate)},
		{"authors", book.Authors, fmt.Sprintf("Authors set to %q", book.Authors)},
	}

	changes := make([]Change, 0, len(fields))
	for _, field := range fields {
		changes = append(changes, Change{
			BookID:      book.ID,
			OccurredAt:  book.CreatedAt,
			Kind:        "created",
			Field:       field.name,
			NewValue:    field.value,
			Description: field.description,
		})
	}
	return changes
}

func updatedChanges(before, after Book) []Change {
	changes := make([]Change, 0, 4)
	if before.Title != after.Title {
		changes = append(changes, Change{
			BookID:      after.ID,
			OccurredAt:  after.UpdatedAt,
			Kind:        "updated",
			Field:       "title",
			OldValue:    before.Title,
			NewValue:    after.Title,
			Description: fmt.Sprintf("Title changed from %q to %q", before.Title, after.Title),
		})
	}
	if before.Description != after.Description {
		changes = append(changes, Change{
			BookID:      after.ID,
			OccurredAt:  after.UpdatedAt,
			Kind:        "updated",
			Field:       "description",
			OldValue:    before.Description,
			NewValue:    after.Description,
			Description: fmt.Sprintf("Description changed from %q to %q", before.Description, after.Description),
		})
	}
	if before.PublicationDate != after.PublicationDate {
		changes = append(changes, Change{
			BookID:      after.ID,
			OccurredAt:  after.UpdatedAt,
			Kind:        "updated",
			Field:       "publication_date",
			OldValue:    before.PublicationDate,
			NewValue:    after.PublicationDate,
			Description: fmt.Sprintf("Publication date changed from %q to %q", before.PublicationDate, after.PublicationDate),
		})
	}
	if !slices.Equal(before.Authors, after.Authors) {
		changes = append(changes, Change{
			BookID:      after.ID,
			OccurredAt:  after.UpdatedAt,
			Kind:        "updated",
			Field:       "authors",
			OldValue:    before.Authors,
			NewValue:    after.Authors,
			Description: describeAuthors(before.Authors, after.Authors),
		})
	}
	return changes
}

func describeAuthors(before, after []string) string {
	if len(after) == len(before)+1 {
		for i, author := range after {
			if slices.Equal(before[:i], after[:i]) && slices.Equal(before[i:], after[i+1:]) {
				return fmt.Sprintf("Author %q was added", author)
			}
		}
	}
	if len(before) == len(after)+1 {
		for i, author := range before {
			if slices.Equal(before[:i], after[:i]) && slices.Equal(before[i+1:], after[i:]) {
				return fmt.Sprintf("Author %q was removed", author)
			}
		}
	}
	return fmt.Sprintf("Authors changed from %q to %q", before, after)
}
