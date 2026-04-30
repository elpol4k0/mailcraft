package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	rc := http.NewResponseController(w)
	if err := rc.Flush(); err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ch, cancel := s.store.Subscribe(r.Context())
	defer cancel()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	flush := func() { rc.Flush() }

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, data)
			flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"version":  "dev",
		"uptime_s": int64(time.Since(s.startTime).Seconds()),
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("stats: %v", err))
		return
	}

	rules, err := s.store.ListRules(r.Context())
	if err == nil {
		stats.RulesCount = len(rules)
	}

	writeJSON(w, http.StatusOK, stats)
}
