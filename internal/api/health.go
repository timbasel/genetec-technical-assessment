package api

import "net/http"

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	if err := s.health.GetHealth(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
