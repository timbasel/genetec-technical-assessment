package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/timbasel/genetec-technical-assessment/internal/books"
	"github.com/timbasel/genetec-technical-assessment/internal/utils"
)

func (s *Server) CreateBook(w http.ResponseWriter, r *http.Request) {
	request, err := utils.DecodeJSON[BookFields](w, r, "title", "description", "publication_date", "authors")
	if err != nil {
		writeRequestError(w, err)
		return
	}
	book, err := s.books.CreateBook(r.Context(), bookFields(request))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response, err := bookResponse(book)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Location", "/books/"+book.ID)
	writeJSON(w, http.StatusCreated, response)
}

func (s *Server) GetBook(w http.ResponseWriter, r *http.Request, id BookID) {
	book, err := s.books.GetBook(r.Context(), id.String())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response, err := bookResponse(book)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) UpdateBook(w http.ResponseWriter, r *http.Request, id BookID) {
	request, err := utils.DecodeJSON[UpdateBookRequest](w, r, "title", "description", "publication_date", "authors", "version")
	if err != nil {
		writeRequestError(w, err)
		return
	}
	book, err := s.books.UpdateBook(r.Context(), id.String(), request.Version, books.Fields{
		Title:           request.Title,
		Description:     request.Description,
		PublicationDate: request.PublicationDate.String(),
		Authors:         request.Authors,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response, err := bookResponse(book)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) GetBookHistory(w http.ResponseWriter, r *http.Request, id BookID, params GetBookHistoryParams) {
	for _, name := range []string{"limit", "offset", "kind", "field", "from", "to", "order"} {
		for _, value := range r.URL.Query()[name] {
			if value == "" {
				writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("`%s` must not be empty", name))
				return
			}
		}
	}
	query := books.Query{From: params.From, To: params.To}
	if params.Limit != nil {
		if *params.Limit == 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "`limit` must be between 1 and 100")
			return
		}
		query.Limit = *params.Limit
	}
	if params.Offset != nil {
		query.Offset = *params.Offset
	}
	if params.Kind != nil {
		query.Kind = string(*params.Kind)
	}
	if params.Field != nil {
		query.Field = string(*params.Field)
	}
	if params.Order != nil {
		query.Order = string(*params.Order)
	}

	page, err := s.books.GetBookHistory(r.Context(), id.String(), query)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response := HistoryPage{Items: make([]HistoryEntry, 0, len(page.Items)), Total: page.Total, Limit: page.Limit, Offset: page.Offset}
	for _, change := range page.Items {
		entry, err := historyEntry(change)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		response.Items = append(response.Items, entry)
	}
	writeJSON(w, http.StatusOK, response)
}

func bookFields(request BookFields) books.Fields {
	return books.Fields{
		Title:           request.Title,
		Description:     request.Description,
		PublicationDate: request.PublicationDate.String(),
		Authors:         request.Authors,
	}
}

func bookResponse(book books.Book) (Book, error) {
	id, err := uuid.Parse(book.ID)
	if err != nil {
		return Book{}, fmt.Errorf("parse book ID: %w", err)
	}
	date, err := time.Parse("2006-01-02", book.PublicationDate)
	if err != nil {
		return Book{}, fmt.Errorf("parse publication date: %w", err)
	}
	return Book{
		Id:              id,
		Title:           book.Title,
		Description:     book.Description,
		PublicationDate: PublicationDate{Time: date},
		Authors:         book.Authors,
		Version:         book.Version,
		CreatedAt:       book.CreatedAt,
		UpdatedAt:       book.UpdatedAt,
	}, nil
}

func historyEntry(change books.Change) (HistoryEntry, error) {
	id, err := uuid.Parse(change.BookID)
	if err != nil {
		return HistoryEntry{}, fmt.Errorf("parse history book ID: %w", err)
	}
	newValue, err := historyValue(change.NewValue)
	if err != nil {
		return HistoryEntry{}, fmt.Errorf("encode new history value: %w", err)
	}
	entry := HistoryEntry{
		Id: change.ID, BookId: id, OccurredAt: change.OccurredAt,
		Kind: HistoryEntryKind(change.Kind), Field: HistoryEntryField(change.Field),
		New: newValue, Description: change.Description,
	}
	if change.OldValue != nil {
		oldValue, err := historyValue(change.OldValue)
		if err != nil {
			return HistoryEntry{}, fmt.Errorf("encode old history value: %w", err)
		}
		entry.Old = &oldValue
	}
	return entry, nil
}

func historyValue(value any) (HistoryValue, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return HistoryValue{}, err
	}
	var result HistoryValue
	if err := result.UnmarshalJSON(data); err != nil {
		return HistoryValue{}, err
	}
	return result, nil
}
