package books

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"
)

var (
	ErrInvalidFields   = errors.New("invalid book fields")
	ErrNotFound        = errors.New("book not found")
	ErrVersionConflict = errors.New("book version conflict")
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
	changes, err := creationChanges(book)
	if err != nil {
		return Book{}, err
	}
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
		return Book{}, fmt.Errorf("get book for update: %w", err)
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
	changes, err := updatedChanges(before, after)
	if err != nil {
		return Book{}, err
	}
	if err := s.store.UpdateBook(ctx, after, changes); err != nil {
		return Book{}, fmt.Errorf("update book: %w", err)
	}
	return after, nil
}

func validateFields(fields Fields) error {
	if strings.TrimSpace(fields.Title) == "" || utf8.RuneCountInString(fields.Title) > 200 {
		return fmt.Errorf("%w: title must contain 1 to 200 characters", ErrInvalidFields)
	}
	if utf8.RuneCountInString(fields.Description) > 2000 {
		return fmt.Errorf("%w: description exceeds 2000 characters", ErrInvalidFields)
	}

	date, err := time.Parse("2006-01-02", fields.PublicationDate)
	if err != nil || date.Format("2006-01-02") != fields.PublicationDate {
		return fmt.Errorf("%w: publication date must be YYYY-MM-DD", ErrInvalidFields)
	}

	if len(fields.Authors) < 1 || len(fields.Authors) > 20 {
		return fmt.Errorf("%w: provide 1 to 20 authors", ErrInvalidFields)
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

func creationChanges(book Book) ([]Change, error) {
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
		change, err := newChange(book.ID, book.CreatedAt, "created", field.name, nil, field.value, field.description)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func updatedChanges(before, after Book) ([]Change, error) {
	changes := make([]Change, 0, 4)
	if before.Title != after.Title {
		change, err := newChange(after.ID, after.UpdatedAt, "updated", "title", before.Title, after.Title,
			fmt.Sprintf("Title changed from %q to %q", before.Title, after.Title))
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	if before.Description != after.Description {
		change, err := newChange(after.ID, after.UpdatedAt, "updated", "description", before.Description, after.Description,
			fmt.Sprintf("Description changed from %q to %q", before.Description, after.Description))
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	if before.PublicationDate != after.PublicationDate {
		change, err := newChange(after.ID, after.UpdatedAt, "updated", "publication_date", before.PublicationDate, after.PublicationDate,
			fmt.Sprintf("Publication date changed from %q to %q", before.PublicationDate, after.PublicationDate))
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	if !slices.Equal(before.Authors, after.Authors) {
		change, err := newChange(after.ID, after.UpdatedAt, "updated", "authors", before.Authors, after.Authors,
			describeAuthors(before.Authors, after.Authors))
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func newChange(bookID string, at time.Time, kind, field string, oldValue, newValue any, description string) (Change, error) {
	var oldJSON json.RawMessage
	if oldValue != nil {
		value, err := json.Marshal(oldValue)
		if err != nil {
			return Change{}, fmt.Errorf("encode old %s value: %w", field, err)
		}
		oldJSON = value
	}
	newJSON, err := json.Marshal(newValue)
	if err != nil {
		return Change{}, fmt.Errorf("encode new %s value: %w", field, err)
	}
	return Change{
		BookID:      bookID,
		OccurredAt:  at,
		Kind:        kind,
		Field:       field,
		OldValue:    oldJSON,
		NewValue:    newJSON,
		Description: description,
	}, nil
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
