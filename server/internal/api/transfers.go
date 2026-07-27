package api

import (
	"net/http"
)

func (s *Server) handleListTransfers(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"transfers": []interface{}{},
		"items":     []interface{}{},
	})
}

func (s *Server) handleClearCompletedTransfers(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"message": "ok"})
}

func (s *Server) handleTransferAppeal(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"message": "ok"})
}
