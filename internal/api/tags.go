package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.store.Tags(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("list tags: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.store.DeleteTag(r.Context(), name); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("delete tag: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleRenameTag(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var body struct {
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if body.NewName == "" {
		writeError(w, http.StatusBadRequest, "new_name is required")
		return
	}
	if err := s.store.RenameTag(r.Context(), name, body.NewName); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("rename tag: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "renamed"})
}
