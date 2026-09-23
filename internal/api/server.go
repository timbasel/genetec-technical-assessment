//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -package api -generate models,std-http-server -o server.generated.go ./openapi.yaml

package api

import (
	"database/sql"
	"embed"
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

func (s *Server) GetBook(w http.ResponseWriter, r *http.Request, id BookID) {
	log.Fatal("Unimplemented")
}

func (s *Server) CreateBook(w http.ResponseWriter, r *http.Request) {
	log.Fatal("Unimplemented")
}

func (s *Server) UpdateBook(w http.ResponseWriter, r *http.Request, id BookID) {
	log.Fatal("Unimplemented")
}

func (s *Server) GetBookHistory(w http.ResponseWriter, r *http.Request, id BookID, history GetBookHistoryParams) {
	log.Fatal("Unimplemented")
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	if s.db == nil || s.db.PingContext(r.Context()) != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

//go:embed openapi.yaml
var OpenAPISpec []byte

func (s *Server) GetOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Write(OpenAPISpec)
}

//go:embed docs/*
var docs embed.FS

func (s *Server) GetDocs(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/docs/", http.StatusMovedPermanently)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /docs/", http.FileServerFS(docs))
	return HandlerFromMux(s, mux)
}
