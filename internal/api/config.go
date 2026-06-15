package api

import (
	"encoding/json"
	"net/http"
)

type mutableStore interface {
	SetMaxEmails(n int)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"smtp_addr":  s.cfg.SMTPAddr,
		"http_addr":  s.cfg.HTTPAddr,
		"max_emails": s.cfg.MaxEmails,
		"base_path":  s.cfg.BasePath,
		"log_level":  s.cfg.LogLevel,
	})
}

func (s *Server) handlePatchConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LogLevel  *string `json:"log_level"`
		MaxEmails *int    `json:"max_emails"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	if body.LogLevel != nil {
		switch *body.LogLevel {
		case "debug", "info", "warn", "error":
			s.cfg.SetLogLevel(*body.LogLevel)
		default:
			writeError(w, http.StatusBadRequest, "log_level must be debug, info, warn, or error")
			return
		}
	}

	if body.MaxEmails != nil {
		if *body.MaxEmails < 1 {
			writeError(w, http.StatusBadRequest, "max_emails must be >= 1")
			return
		}
		if ms, ok := s.store.(mutableStore); ok {
			ms.SetMaxEmails(*body.MaxEmails)
		}
		s.cfg.MaxEmails = *body.MaxEmails
	}

	s.handleConfig(w, r)
}
