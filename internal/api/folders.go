package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"mailcraft/internal/store"
)

func (s *Server) handleListFolders(w http.ResponseWriter, r *http.Request) {
	folders, err := s.store.Folders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("folders: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, folders)
}

func (s *Server) handleRenameFolder(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var body struct {
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NewName == "" {
		writeError(w, http.StatusBadRequest, "new_name required")
		return
	}
	if err := s.store.RenameFolder(r.Context(), name, body.NewName); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("rename folder: %v", err))
		return
	}
	s.store.Publish(store.Event{Type: "folders.updated"})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.store.DeleteFolder(r.Context(), name); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("delete folder: %v", err))
		return
	}
	s.store.Publish(store.Event{Type: "folders.updated"})
	w.WriteHeader(http.StatusNoContent)
}
