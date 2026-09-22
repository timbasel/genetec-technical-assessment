package books_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/timbasel/genetec-technical-assessment/internal/books"
	"github.com/timbasel/genetec-technical-assessment/internal/store"
)

func testService(t *testing.T) (*books.Service, *store.Store) {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return books.NewService(db), db
}

func testFields() books.Fields {
	return books.Fields{
		Title:           "The Hobbitt",
		Description:     "An adventure",
		PublicationDate: "1937-09-21",
		Authors:         []string{"J.R.R. Tolkien"},
	}
}

type storedChange struct {
	field       string
	oldValue    sql.NullString
	newValue    string
	description string
}

func readChanges(t *testing.T, db *store.Store, id string) []storedChange {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), `
		SELECT field, old_value, new_value, description
		FROM books_history WHERE book_id = ? ORDER BY id`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var changes []storedChange
	for rows.Next() {
		var change storedChange
		if err := rows.Scan(&change.field, &change.oldValue, &change.newValue, &change.description); err != nil {
			t.Fatal(err)
		}
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return changes
}

func TestServiceCreateBook(t *testing.T) {
	service, db := testService(t)
	fields := testFields()
	book, err := service.CreateBook(context.Background(), fields)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(book.ID); err != nil {
		t.Fatalf("invalid book ID %q: %v", book.ID, err)
	}
	if book.Version != 1 || book.CreatedAt.IsZero() || !book.CreatedAt.Equal(book.UpdatedAt) || book.CreatedAt.Location() != time.UTC {
		t.Fatalf("unexpected book metadata: %+v", book)
	}
	fields.Authors[0] = "mutated by caller"
	got, err := service.GetBook(context.Background(), book.ID)
	if err != nil || !reflect.DeepEqual(got, book) {
		t.Fatalf("GetBook() = %+v, %v; want %+v", got, err, book)
	}
	want := []storedChange{
		{field: "title", newValue: `"The Hobbitt"`, description: `Title set to "The Hobbitt"`},
		{field: "description", newValue: `"An adventure"`, description: `Description set to "An adventure"`},
		{field: "publication_date", newValue: `"1937-09-21"`, description: `Publication date set to "1937-09-21"`},
		{field: "authors", newValue: `["J.R.R. Tolkien"]`, description: `Authors set to ["J.R.R. Tolkien"]`},
	}
	if got := readChanges(t, db, book.ID); !reflect.DeepEqual(got, want) {
		t.Fatalf("creation changes = %+v, want %+v", got, want)
	}
}

func TestServiceUpdateBook(t *testing.T) {
	service, db := testService(t)
	original, err := service.CreateBook(context.Background(), testFields())
	if err != nil {
		t.Fatal(err)
	}
	fields := testFields()
	fields.Title = "The Hobbit"
	fields.Authors = []string{"J.R.R. Tolkien", "Another Author"}
	updated, err := service.UpdateBook(context.Background(), original.ID, original.Version, fields)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || updated.Title != "The Hobbit" || !reflect.DeepEqual(updated.Authors, fields.Authors) || !updated.CreatedAt.Equal(original.CreatedAt) || updated.UpdatedAt.Before(original.UpdatedAt) {
		t.Fatalf("unexpected updated book: %+v", updated)
	}
	got, err := service.GetBook(context.Background(), original.ID)
	if err != nil || !reflect.DeepEqual(got, updated) {
		t.Fatalf("GetBook() = %+v, %v; want %+v", got, err, updated)
	}
	changes := readChanges(t, db, original.ID)
	want := []storedChange{
		{field: "title", newValue: `"The Hobbitt"`, description: `Title set to "The Hobbitt"`},
		{field: "description", newValue: `"An adventure"`, description: `Description set to "An adventure"`},
		{field: "publication_date", newValue: `"1937-09-21"`, description: `Publication date set to "1937-09-21"`},
		{field: "authors", newValue: `["J.R.R. Tolkien"]`, description: `Authors set to ["J.R.R. Tolkien"]`},
		{field: "title", oldValue: sql.NullString{String: `"The Hobbitt"`, Valid: true}, newValue: `"The Hobbit"`, description: `Title changed from "The Hobbitt" to "The Hobbit"`},
		{field: "authors", oldValue: sql.NullString{String: `["J.R.R. Tolkien"]`, Valid: true}, newValue: `["J.R.R. Tolkien","Another Author"]`, description: `Author "Another Author" was added`},
	}
	if !reflect.DeepEqual(changes, want) {
		t.Fatalf("changes = %+v, want %+v", changes, want)
	}
}

func TestServiceAuthorDescriptions(t *testing.T) {
	tests := []struct {
		name        string
		authors     []string
		description string
	}{
		{"removed", []string{"Second Author"}, `Author "First Author" was removed`},
		{"reordered", []string{"Second Author", "First Author"}, `Authors changed from ["First Author" "Second Author"] to ["Second Author" "First Author"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, db := testService(t)
			fields := testFields()
			fields.Authors = []string{"First Author", "Second Author"}
			book, err := service.CreateBook(context.Background(), fields)
			if err != nil {
				t.Fatal(err)
			}
			fields.Authors = tt.authors
			if _, err := service.UpdateBook(context.Background(), book.ID, book.Version, fields); err != nil {
				t.Fatal(err)
			}
			changes := readChanges(t, db, book.ID)
			if len(changes) != 5 || changes[4].field != "authors" || changes[4].description != tt.description {
				t.Fatalf("author changes = %+v, want one entry with description %q", changes, tt.description)
			}
		})
	}
}

func TestServiceNoOpAndStaleVersion(t *testing.T) {
	service, db := testService(t)
	book, err := service.CreateBook(context.Background(), testFields())
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := service.UpdateBook(context.Background(), book.ID, book.Version, testFields())
	if err != nil || !reflect.DeepEqual(unchanged, book) {
		t.Fatalf("no-op update = %+v, %v; want %+v", unchanged, err, book)
	}
	if len(readChanges(t, db, book.ID)) != 4 {
		t.Fatal("no-op update added history")
	}
	_, err = service.UpdateBook(context.Background(), book.ID, book.Version+1, testFields())
	if !errors.Is(err, books.ErrVersionConflict) {
		t.Fatalf("stale no-op error = %v, want ErrVersionConflict", err)
	}
	fields := testFields()
	fields.Title = "The Hobbit"
	_, err = service.UpdateBook(context.Background(), book.ID, book.Version+1, fields)
	if !errors.Is(err, books.ErrVersionConflict) {
		t.Fatalf("stale update error = %v, want ErrVersionConflict", err)
	}
	if len(readChanges(t, db, book.ID)) != 4 {
		t.Fatal("stale update added history")
	}
}

func TestServiceMissingBook(t *testing.T) {
	service, _ := testService(t)
	_, err := service.GetBook(context.Background(), "missing")
	if !errors.Is(err, books.ErrNotFound) {
		t.Fatalf("GetBook() error = %v, want ErrNotFound", err)
	}
	_, err = service.UpdateBook(context.Background(), "missing", 1, testFields())
	if !errors.Is(err, books.ErrNotFound) {
		t.Fatalf("UpdateBook() error = %v, want ErrNotFound", err)
	}
}

func TestServiceRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name string
		edit func(*books.Fields)
	}{
		{"blank title", func(f *books.Fields) { f.Title = "  " }},
		{"long title", func(f *books.Fields) { f.Title = strings.Repeat("a", 201) }},
		{"long description", func(f *books.Fields) { f.Description = strings.Repeat("a", 2001) }},
		{"bad date", func(f *books.Fields) { f.PublicationDate = "1937-02-30" }},
		{"no authors", func(f *books.Fields) { f.Authors = nil }},
		{"duplicate authors", func(f *books.Fields) { f.Authors = []string{"Tolkien", "Tolkien"} }},
		{"author whitespace", func(f *books.Fields) { f.Authors = []string{" Tolkien"} }},
		{"trailing author whitespace", func(f *books.Fields) { f.Authors = []string{"Tolkien "} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, db := testService(t)
			fields := testFields()
			tt.edit(&fields)
			if _, err := service.CreateBook(context.Background(), fields); !errors.Is(err, books.ErrInvalidFields) {
				t.Fatalf("CreateBook() error = %v, want ErrInvalidFields", err)
			}
			var count int
			if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM books").Scan(&count); err != nil || count != 0 {
				t.Fatalf("books count = %d, error = %v; want 0", count, err)
			}
		})
	}
}
