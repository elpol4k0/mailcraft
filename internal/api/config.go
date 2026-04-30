package api

import "net/http"

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"smtp_addr":  s.cfg.SMTPAddr,
		"http_addr":  s.cfg.HTTPAddr,
		"max_emails": s.cfg.MaxEmails,
		"base_path":  s.cfg.BasePath,
		"log_level":  s.cfg.LogLevel,
	})
}
