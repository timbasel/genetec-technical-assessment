//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package api -generate models,std-http-server -o server.generated.go ./openapi.yaml

package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/timbasel/genetec-technical-assessment/internal/books"
)

type Server struct {
	db    *sql.DB
	books *books.Service
}

func NewServer(db *sql.DB, booksService *books.Service) *Server {
	return &Server{db: db, books: booksService}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	registerDocs(mux)
	return HandlerWithOptions(s, StdHTTPServerOptions{
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		},
	})
}

func writeRequestError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds 1 MiB")
		return
	}
	writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, books.ErrInvalidFields), errors.Is(err, books.ErrInvalidHistoryQuery):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, books.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "book not found")
	case errors.Is(err, books.ErrVersionConflict):
		writeError(w, http.StatusConflict, "version_conflict", "book version conflict")
	default:
		log.Printf("API error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	response := ErrorResponse{}
	response.Error.Code = code
	response.Error.Message = message
	writeJSON(w, status, response)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		log.Printf("encode API response: %v", err)
		w.Header().Del("Location")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write(append([]byte(`{"error":{"code":"internal_error","message":"internal server error"}}`), '\n')); err != nil {
			log.Printf("write API error response: %v", err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(append(body, '\n')); err != nil {
		log.Printf("write API response: %v", err)
	}
}
