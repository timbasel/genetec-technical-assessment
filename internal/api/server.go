//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package api -generate models,std-http-server -o server.generated.go ./openapi.yaml

package api

import (
	"database/sql"
	_ "embed"
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

func (Server) GetBook(w http.ResponseWriter, r *http.Request, id BookID) {
	log.Fatal("Unimplemented")
}

func (Server) CreateBook(w http.ResponseWriter, r *http.Request) {
	log.Fatal("Unimplemented")
}

func (Server) UpdateBook(w http.ResponseWriter, r *http.Request, id BookID) {
	log.Fatal("Unimplemented")
}

func (Server) GetBookHistory(w http.ResponseWriter, r *http.Request, id BookID, history GetBookHistoryParams) {
	log.Fatal("Unimplemented")
}

func (Server) GetDocs(w http.ResponseWriter, r *http.Request) {
	log.Fatal("Unimplemented")
}

//go:embed openapi.yaml
var OpenAPISpec []byte

func (Server) GetOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Write(OpenAPISpec)
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	if s.db == nil || s.db.PingContext(r.Context()) != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
