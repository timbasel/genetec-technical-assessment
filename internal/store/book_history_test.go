package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/timbasel/genetec-technical-assessment/internal/books"
)

func seedHistory(t *testing.T) (*Store, books.Book, []books.Change) {
	t.Helper()
	ctx := context.Background()
	store := newTestStore(t)
	at := time.Date(2026, time.September, 25, 10, 20, 30, 0, time.UTC)
	book := books.Book{
		ID:              "018f47a6-28c9-7e24-9879-4b71c576d0e1",
		Title:           "The Hobbit",
		Description:     "An adventure",
		PublicationDate: "1937-09-21",
		Authors:         []string{"J.R.R. Tolkien"},
		Version:         1,
		CreatedAt:       at,
		UpdatedAt:       at,
	}
	changes := []books.Change{
		{
			ID: 1, BookID: book.ID, OccurredAt: at, Kind: "created", Field: "title",
			NewValue: book.Title, Description: "Title set",
		},
		{
			ID: 2, BookID: book.ID, OccurredAt: at, Kind: "created", Field: "authors",
			NewValue: book.Authors, Description: "Authors set",
		},
		{
			ID: 3, BookID: book.ID, OccurredAt: at.Add(100 * time.Millisecond), Kind: "updated", Field: "title",
			OldValue: book.Title, NewValue: "The Hobbit, revised", Description: "Title changed",
		},
		{
			ID: 4, BookID: book.ID, OccurredAt: at.Add(500 * time.Millisecond), Kind: "updated", Field: "description",
			OldValue: book.Description, NewValue: "Revised", Description: "Description changed",
		},
		{
			ID: 5, BookID: book.ID, OccurredAt: at.Add(time.Second), Kind: "updated", Field: "authors",
			OldValue: book.Authors, NewValue: []string{"J.R.R. Tolkien", "Other Author"}, Description: "Author added",
		},
	}
	if err := store.CreateBook(ctx, book, changes[:2]); err != nil {
		t.Fatal(err)
	}
	updated := book
	updated.Version++
	updated.UpdatedAt = at.Add(time.Second)
	if err := store.UpdateBook(ctx, updated, changes[2:]); err != nil {
		t.Fatal(err)
	}
	return store, book, changes
}

func TestGetBookHistoryPaginationAndOrdering(t *testing.T) {
	store, book, changes := seedHistory(t)
	ctx := context.Background()

	page, err := store.GetBookHistory(ctx, book.ID, books.Query{Limit: 2, Offset: 1, Order: "asc"})
	if err != nil {
		t.Fatal(err)
	}
	want := books.Page{Items: changes[1:3], Total: 5, Limit: 2, Offset: 1}
	if !reflect.DeepEqual(page, want) {
		t.Fatalf("ascending page = %+v, want %+v", page, want)
	}

	page, err = store.GetBookHistory(ctx, book.ID, books.Query{Limit: 2, Order: "desc"})
	if err != nil {
		t.Fatal(err)
	}
	want = books.Page{Items: []books.Change{changes[4], changes[3]}, Total: 5, Limit: 2, Offset: 0}
	if !reflect.DeepEqual(page, want) {
		t.Fatalf("descending page = %+v, want %+v", page, want)
	}

	page, err = store.GetBookHistory(ctx, book.ID, books.Query{Limit: 2, Offset: 20, Order: "asc"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 5 || page.Limit != 2 || page.Offset != 20 || page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("out-of-range page = %+v", page)
	}
}

func TestGetBookHistoryFilters(t *testing.T) {
	store, book, changes := seedHistory(t)
	at := changes[0].OccurredAt
	tests := []struct {
		name  string
		query books.Query
		want  []books.Change
	}{
		{"kind", books.Query{Limit: 20, Kind: "created", Order: "asc"}, changes[:2]},
		{"field", books.Query{Limit: 20, Field: "title", Order: "asc"}, []books.Change{changes[0], changes[2]}},
		{"combined", books.Query{Limit: 20, Kind: "updated", Field: "authors", Order: "asc"}, []books.Change{changes[4]}},
		{"from inclusive", books.Query{Limit: 20, From: &at, Order: "asc"}, changes},
		{"to inclusive", books.Query{Limit: 20, To: &at, Order: "asc"}, changes[:2]},
		{
			"range",
			books.Query{Limit: 20, From: timePtr(at.Add(100 * time.Millisecond)), To: timePtr(at.Add(500 * time.Millisecond)), Order: "asc"},
			changes[2:4],
		},
		{
			"nanosecond boundary",
			books.Query{Limit: 20, From: timePtr(at.Add(100*time.Millisecond + time.Nanosecond)), Order: "asc"},
			changes[3:],
		},
		{
			"non-UTC boundary",
			books.Query{Limit: 20, To: timePtr(at.In(time.FixedZone("EST", -5*60*60))), Order: "asc"},
			changes[:2],
		},
		{
			"no matches",
			books.Query{Limit: 20, Kind: "created", From: timePtr(at.Add(time.Second)), Order: "asc"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := store.GetBookHistory(context.Background(), book.ID, tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if page.Total != len(tt.want) || !reflect.DeepEqual(page.Items, append([]books.Change{}, tt.want...)) {
				t.Fatalf("page = %+v, want items %+v", page, tt.want)
			}
		})
	}
}

func timePtr(at time.Time) *time.Time { return &at }

func TestGetBookHistoryMissingBook(t *testing.T) {
	store := newTestStore(t)
	_, err := store.GetBookHistory(context.Background(), "missing", books.Query{Limit: 20, Order: "desc"})
	if !errors.Is(err, books.ErrNotFound) {
		t.Fatalf("GetBookHistory() error = %v, want ErrNotFound", err)
	}
}

func TestGetBookHistoryRejectsInvalidPaginationAndOrder(t *testing.T) {
	store, book, _ := seedHistory(t)
	for _, query := range []books.Query{
		{Limit: 0, Order: "desc"},
		{Limit: 101, Order: "desc"},
		{Limit: 20, Offset: -1, Order: "desc"},
		{Limit: 20, Order: "desc; DROP TABLE books"},
	} {
		_, err := store.GetBookHistory(context.Background(), book.ID, query)
		if !errors.Is(err, books.ErrInvalidFields) {
			t.Fatalf("GetBookHistory(%+v) error = %v, want ErrInvalidFields", query, err)
		}
	}
}
