package api

import (
	"embed"
	"net/http"
)

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

func registerDocs(mux *http.ServeMux) {
	mux.Handle("GET /docs/", http.FileServerFS(docs))
}
