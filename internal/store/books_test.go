package store

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/timbasel/genetec-technical-assessment/internal/books"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func seedTestBook(t *testing.T) (*Store, books.Book) {
	t.Helper()
	store := newTestStore(t)
	now := time.Date(2026, time.September, 25, 10, 20, 30, 0, time.UTC)
	book := books.Book{
		ID:              "018f47a6-28c9-7e24-9879-4b71c576d0e1",
		Title:           "The Hobbitt",
		Description:     "An adventure",
		PublicationDate: "1937-09-21",
		Authors:         []string{"J.R.R. Tolkien"},
		Version:         1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := store.CreateBook(context.Background(), book, nil); err != nil {
		t.Fatal(err)
	}
	return store, book
}

func assertStoredChanges(t *testing.T, store *Store, bookID string, changes []books.Change) {
	t.Helper()
	rows, err := store.QueryContext(context.Background(), `SELECT * FROM books_history WHERE book_id = ? ORDER BY id`, bookID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for i, change := range changes {
		if !rows.Next() {
			t.Fatalf("history row %d is missing", i)
		}
		var result books.Change
		if err := scanBookChange(rows, &result); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(change, result) {
			t.Fatalf("book_history - expected: %+v; got: %+v", change, result)
		}
	}
	if rows.Next() {
		t.Fatal("Unexpected additional history row")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateBook(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, time.September, 22, 10, 20, 30, 0, time.UTC)
	uuid := "018f47a6-28c9-7e24-9879-4b71c576d0e1"

	book := books.Book{
		ID:              uuid,
		Title:           "The Hobbit",
		Description:     "An adventure in Middle-earth",
		PublicationDate: "1937-09-21",
		Authors:         []string{"J.R.R. Tolkin"},
		Version:         1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	changes := []books.Change{
		{
			ID: 1, BookID: book.ID, OccurredAt: now, Kind: "created", Field: "title", NewValue: "The Hobbit", Description: `Title set to "The Hobbit"`,
		},
		{
			ID: 2, BookID: book.ID, OccurredAt: now, Kind: "created", Field: "description", NewValue: "An adventure in Middle-earth", Description: `Description set to "An adventure in Middle-earth"`,
		},
		{
			ID: 3, BookID: book.ID, OccurredAt: now, Kind: "created", Field: "publication_date", NewValue: "1937-09-21", Description: `Publication date set to "1937-09-21"`,
		},
		{
			ID: 4, BookID: book.ID, OccurredAt: now, Kind: "created", Field: "authors", NewValue: []string{"J.R.R. Tolkien"}, Description: `Authors set to ["J.R.R. Tolkien"]`,
		},
	}

	if err := store.CreateBook(ctx, book, changes); err != nil {
		t.Fatal(err)
	}

	var result books.Book
	if err := scanBook(store.QueryRowContext(ctx, `SELECT * FROM books WHERE id = ?`, book.ID), &result); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(book, result) {
		t.Fatalf("book - expected: %+v; got: %+v", book, result)
	}

	assertStoredChanges(t, store, book.ID, changes)
}

func TestGetBook(t *testing.T) {
	store, want := seedTestBook(t)
	got, err := store.GetBook(context.Background(), want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetBook() = %+v, want %+v", got, want)
	}
}

func TestGetBookMissing(t *testing.T) {
	store := newTestStore(t)
	_, err := store.GetBook(context.Background(), "missing")
	if !errors.Is(err, books.ErrNotFound) {
		t.Fatalf("GetBook() error = %v, want ErrNotFound", err)
	}
}

func TestUpdateBook(t *testing.T) {
	store, original := seedTestBook(t)
	updated := original
	updated.Title = "The Hobbit"
	updated.Authors = []string{"J.R.R. Tolkien", "Another Author"}
	updated.Version = 2
	updated.UpdatedAt = original.UpdatedAt.Add(time.Hour)

	changes := []books.Change{
		{
			ID: 1, BookID: original.ID, OccurredAt: updated.UpdatedAt, Kind: "updated", Field: "title", OldValue: "The Hobbitt", NewValue: "The Hobbit", Description: `Title changed from "The Hobbitt" to "The Hobbit"`,
		},
		{
			ID: 2, BookID: original.ID, OccurredAt: updated.UpdatedAt, Kind: "updated", Field: "authors", OldValue: []string{"J.R.R. Tolkien"}, NewValue: []string{"J.R.R. Tolkien", "Another Author"}, Description: `Author "Another Author" was added`,
		},
	}

	if err := store.UpdateBook(context.Background(), updated, changes); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetBook(context.Background(), original.ID)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, updated) {
		t.Fatalf("updated book = %+v, want %+v", got, updated)
	}
	assertStoredChanges(t, store, original.ID, changes)
}

func TestUpdateBookRejectsStaleVersion(t *testing.T) {
	store, original := seedTestBook(t)
	updated := original
	updated.Title = "The Hobbit"
	updated.Version = 2
	updated.UpdatedAt = original.UpdatedAt.Add(time.Hour)

	if err := store.UpdateBook(context.Background(), updated, nil); err != nil {
		t.Fatal(err)
	}

	stale := updated
	stale.Title = "Wrong title"
	changes := []books.Change{
		{
			Kind: "updated", Field: "title", OccurredAt: updated.UpdatedAt, NewValue: "Wrong title", Description: "Wrong title",
		},
	}

	if err := store.UpdateBook(context.Background(), stale, changes); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("UpdateBook() error = %v, want ErrVersionConflict", err)
	}
	got, err := store.GetBook(context.Background(), original.ID)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, updated) {
		t.Fatalf("book after stale update = %+v, want %+v", got, updated)
	}
	assertStoredChanges(t, store, original.ID, nil)
}

func TestUpdateBookMissing(t *testing.T) {
	store := newTestStore(t)
	book := books.Book{ID: "missing", Version: 2}

	if err := store.UpdateBook(context.Background(), book, nil); !errors.Is(err, books.ErrNotFound) {
		t.Fatalf("UpdateBook() error = %v, want ErrNotFound", err)
	}
}

func TestUpdateBookRollsBackWhenChangeInsertFails(t *testing.T) {
	store, original := seedTestBook(t)
	updated := original
	updated.Title = "The Hobbit"
	updated.Version = 2
	updated.UpdatedAt = original.UpdatedAt.Add(time.Hour)

	changes := []books.Change{
		{
			Kind: "updated", Field: "title", OccurredAt: updated.UpdatedAt, NewValue: "The Hobbit", Description: "Title changed",
		},
		{
			Kind: "updated", Field: "description", OccurredAt: updated.UpdatedAt, NewValue: func() {}, Description: "Invalid change",
		},
	}

	if err := store.UpdateBook(context.Background(), updated, changes); err == nil {
		t.Fatal("UpdateBook() error = nil")
	}
	got, err := store.GetBook(context.Background(), original.ID)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, original) {
		t.Fatalf("book after failed update = %+v, want %+v", got, original)
	}
	assertStoredChanges(t, store, original.ID, nil)
}
