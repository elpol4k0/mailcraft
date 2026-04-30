package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mailcraft/internal/rules"
	"mailcraft/internal/store"
)

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	ruleList, err := s.store.ListRules(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("list rules: %v", err))
		return
	}
	if ruleList == nil {
		ruleList = []*store.Rule{}
	}
	writeJSON(w, http.StatusOK, ruleList)
}

func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var rule store.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	rule.ID = uuid.NewString()
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	if rule.Logic == "" {
		rule.Logic = store.LogicAND
	}

	if err := s.store.AddRule(r.Context(), &rule); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create rule: %v", err))
		return
	}

	s.refreshEngine(r)
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) handleGetRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rule, err := s.store.GetRule(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get rule: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := s.store.GetRule(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get rule: %v", err))
		return
	}

	var rule store.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	rule.Stats = existing.Stats

	if err := s.store.UpdateRule(r.Context(), &rule); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("update rule: %v", err))
		return
	}

	s.refreshEngine(r)
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) handlePatchRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rule, err := s.store.GetRule(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get rule: %v", err))
		return
	}

	var patch struct {
		Enabled  *bool `json:"enabled"`
		Priority *int  `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}

	if patch.Enabled != nil {
		rule.Enabled = *patch.Enabled
	}
	if patch.Priority != nil {
		rule.Priority = *patch.Priority
	}
	rule.UpdatedAt = time.Now()

	if err := s.store.UpdateRule(r.Context(), rule); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("update rule: %v", err))
		return
	}

	s.refreshEngine(r)
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteRule(r.Context(), id); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("delete rule: %v", err))
		return
	}
	s.refreshEngine(r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleTestRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rule, err := s.store.GetRule(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get rule: %v", err))
		return
	}

	emails, _, err := s.store.List(r.Context(), store.SearchFilter{Limit: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("list emails: %v", err))
		return
	}

	matchIDs := rules.TestRule(rule, emails)
	writeJSON(w, http.StatusOK, map[string]any{
		"match_count": len(matchIDs),
		"match_ids":   matchIDs,
	})
}

func (s *Server) handleReorderRules(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}

	for i, id := range body.IDs {
		rule, err := s.store.GetRule(r.Context(), id)
		if err != nil {
			continue
		}
		rule.Priority = i
		rule.UpdatedAt = time.Now()
		_ = s.store.UpdateRule(r.Context(), rule)
	}

	s.refreshEngine(r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) refreshEngine(r *http.Request) {
	ruleList, err := s.store.ListRules(r.Context())
	if err != nil {
		return
	}
	s.engine.SetRules(ruleList)
}
