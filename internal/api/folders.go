package api

import (
	"fmt"
	"net/http"
)

func (s *Server) handleListFolders(w http.ResponseWriter, r *http.Request) {
	folders, err := s.store.Folders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("folders: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, folders)
}
