package api

import "net/http"

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	if s.db == nil || s.db.PingContext(r.Context()) != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
