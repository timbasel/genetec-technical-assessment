package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timbasel/genetec-technical-assessment/internal/api"
	"github.com/timbasel/genetec-technical-assessment/internal/books"
	"github.com/timbasel/genetec-technical-assessment/internal/health"
	"github.com/timbasel/genetec-technical-assessment/internal/store"
)

const validBook = `{"title":"The Hobbitt","description":"An adventure","publication_date":"1937-09-21","authors":["J.R.R. Tolkien"]}`

func bookHandler(t *testing.T) http.Handler {
	t.Helper()
	store, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "books.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return api.NewServer(books.NewService(store), health.NewService(store)).Handler()
}

func apiRequest(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func requireStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, want, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", response.Header().Get("Content-Type"))
	}
}

func TestBookRoutes(t *testing.T) {
	handler := bookHandler(t)
	created := apiRequest(t, handler, http.MethodPost, "/api/books", validBook)
	requireStatus(t, created, http.StatusCreated)
	var book api.Book
	if err := json.Unmarshal(created.Body.Bytes(), &book); err != nil {
		t.Fatal(err)
	}
	id := book.Id.String()
	if book.Title != "The Hobbitt" || book.PublicationDate.String() != "1937-09-21" || book.Version != 1 ||
		book.CreatedAt.IsZero() || created.Header().Get("Location") != "/books/"+id {
		t.Fatalf("created book = %+v; Location = %q", book, created.Header().Get("Location"))
	}
	var body map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["publication_date"]; !ok {
		t.Fatalf("response lacks publication_date: %s", created.Body.String())
	}

	got := apiRequest(t, handler, http.MethodGet, "/api/books/"+id, "")
	requireStatus(t, got, http.StatusOK)
	if got.Body.String() != created.Body.String() {
		t.Fatalf("GET book = %s, want %s", got.Body.String(), created.Body.String())
	}

	initial := apiRequest(t, handler, http.MethodGet, "/api/books/"+id+"/history", "")
	requireStatus(t, initial, http.StatusOK)
	var page api.HistoryPage
	if err := json.Unmarshal(initial.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 4 || page.Limit != 20 || page.Offset != 0 || len(page.Items) != 4 ||
		page.Items[0].Field != api.HistoryEntryFieldAuthors || page.Items[0].Old != nil {
		t.Fatalf("initial history = %+v", page)
	}

	updated := apiRequest(t, handler, http.MethodPut, "/api/books/"+id,
		`{"title":"The Hobbit","description":"An adventure","publication_date":"1937-09-21","authors":["J.R.R. Tolkien","Second Author"],"version":1}`)
	requireStatus(t, updated, http.StatusOK)
	if err := json.Unmarshal(updated.Body.Bytes(), &book); err != nil {
		t.Fatal(err)
	}
	if book.Version != 2 || book.Title != "The Hobbit" || len(book.Authors) != 2 {
		t.Fatalf("updated book = %+v", book)
	}

	filtered := apiRequest(t, handler, http.MethodGet, "/api/books/"+id+"/history?kind=updated&field=title&order=asc&limit=1", "")
	requireStatus(t, filtered, http.StatusOK)
	if err := json.Unmarshal(filtered.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Limit != 1 || len(page.Items) != 1 || page.Items[0].Old == nil ||
		page.Items[0].Description != `Title changed from "The Hobbitt" to "The Hobbit"` {
		t.Fatalf("filtered history = %+v", page)
	}
	oldTitle, err := page.Items[0].Old.AsBookTitle()
	if err != nil || oldTitle != "The Hobbitt" {
		t.Fatalf("old title = %q, error = %v", oldTitle, err)
	}
	newTitle, err := page.Items[0].New.AsBookTitle()
	if err != nil || newTitle != "The Hobbit" {
		t.Fatalf("new title = %q, error = %v", newTitle, err)
	}

	authorHistory := apiRequest(t, handler, http.MethodGet, "/api/books/"+id+"/history?kind=updated&field=authors", "")
	requireStatus(t, authorHistory, http.StatusOK)
	if err := json.Unmarshal(authorHistory.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("author history = %+v", page)
	}
	authors, err := page.Items[0].New.AsAuthors()
	if err != nil || len(authors) != 2 || authors[1] != "Second Author" {
		t.Fatalf("new authors = %q, error = %v", authors, err)
	}

	all := apiRequest(t, handler, http.MethodGet, "/api/books/"+id+"/history?limit=2&offset=1", "")
	requireStatus(t, all, http.StatusOK)
	if err := json.Unmarshal(all.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 6 || page.Limit != 2 || page.Offset != 1 || len(page.Items) != 2 {
		t.Fatalf("paginated history = %+v", page)
	}

	stale := apiRequest(t, handler, http.MethodPut, "/api/books/"+id,
		`{"title":"Wrong","description":"An adventure","publication_date":"1937-09-21","authors":["J.R.R. Tolkien"],"version":1}`)
	requireStatus(t, stale, http.StatusConflict)
}

func TestBookRouteErrors(t *testing.T) {
	handler := bookHandler(t)
	missing := "00000000-0000-0000-0000-000000000001"
	tests := []struct {
		name, method, path, body string
		status                   int
	}{
		{"missing title", http.MethodPost, "/api/books", `{"description":"A","publication_date":"1937-09-21","authors":["Tolkien"]}`, 400},
		{"missing description", http.MethodPost, "/api/books", `{"title":"The Hobbit","publication_date":"1937-09-21","authors":["Tolkien"]}`, 400},
		{"null description", http.MethodPost, "/api/books", `{"title":"The Hobbit","description":null,"publication_date":"1937-09-21","authors":["Tolkien"]}`, 400},
		{"unknown field", http.MethodPost, "/api/books", `{"title":"The Hobbit","description":"A","publication_date":"1937-09-21","authors":["Tolkien"],"extra":1}`, 400},
		{"bad date", http.MethodPost, "/api/books", `{"title":"The Hobbit","description":"A","publication_date":"1937-02-30","authors":["Tolkien"]}`, 400},
		{"duplicate authors", http.MethodPost, "/api/books", `{"title":"The Hobbit","description":"A","publication_date":"1937-09-21","authors":["Tolkien","Tolkien"]}`, 400},
		{"multiple objects", http.MethodPost, "/api/books", validBook + validBook, 400},
		{"empty body", http.MethodPost, "/api/books", "", 400},
		{"too large", http.MethodPost, "/api/books", validBook + strings.Repeat(" ", 1<<20), 413},
		{"invalid id", http.MethodGet, "/api/books/not-a-uuid", "", 400},
		{"missing book", http.MethodGet, "/api/books/" + missing, "", 404},
		{"missing update version", http.MethodPut, "/api/books/" + missing, validBook, 400},
		{"missing update book", http.MethodPut, "/api/books/" + missing, `{"title":"A","description":"","publication_date":"1937-09-21","authors":["Tolkien"],"version":1}`, 404},
		{"missing history", http.MethodGet, "/api/books/" + missing + "/history", "", 404},
		{"invalid limit", http.MethodGet, "/api/books/" + missing + "/history?limit=0", "", 400},
		{"invalid order", http.MethodGet, "/api/books/" + missing + "/history?order=sideways", "", 400},
		{"empty kind", http.MethodGet, "/api/books/" + missing + "/history?kind=", "", 400},
		{"empty field", http.MethodGet, "/api/books/" + missing + "/history?field=", "", 400},
		{"empty order", http.MethodGet, "/api/books/" + missing + "/history?order=", "", 400},
		{"invalid timestamp", http.MethodGet, "/api/books/" + missing + "/history?from=nope", "", 400},
		{"empty timestamp", http.MethodGet, "/api/books/" + missing + "/history?from=", "", 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := apiRequest(t, handler, tt.method, tt.path, tt.body)
			requireStatus(t, response, tt.status)
			var responseBody api.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &responseBody); err != nil || responseBody.Error.Code == "" || responseBody.Error.Message == "" {
				t.Fatalf("error response = %q, decode error = %v", response.Body.String(), err)
			}
		})
	}
}
