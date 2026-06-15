package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	mcsmtp "mailcraft/internal/smtp"
	"mailcraft/internal/store"
)

func (s *Server) handleListEmails(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.SearchFilter{
		Query:  q.Get("q"),
		Tag:    q.Get("tag"),
		Folder: q.Get("folder"),
		From:   q.Get("from"),
		To:     q.Get("to"),
		Sort:   q.Get("sort"),
	}

	if v := q.Get("read"); v != "" {
		b := v == "true"
		filter.Read = &b
	}
	if v := q.Get("starred"); v != "" {
		b := v == "true"
		filter.Starred = &b
	}
	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Page = n
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}

	emails, total, err := s.store.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("list emails: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"emails": emails,
		"total":  total,
		"page":   filter.Page,
		"limit":  filter.Limit,
	})
}

func (s *Server) handleGetEmail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	email, err := s.store.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "email not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get email: %v", err))
		return
	}

	if !email.Read {
		email.Read = true
		if err := s.store.Update(r.Context(), email); err == nil {
			s.store.Publish(store.Event{Type: "email.updated", Payload: email})
		}
	}

	writeJSON(w, http.StatusOK, email)
}

func (s *Server) handleGetEmailRaw(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	email, err := s.store.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "email not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get email: %v", err))
		return
	}

	w.Header().Set("Content-Type", "message/rfc822")
	if len(email.RawMessage) > 0 {
		_, _ = w.Write(email.RawMessage)
	} else {
		_, _ = w.Write([]byte("(raw message not available)"))
	}
}

func (s *Server) handleGetEmailHTML(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	email, err := s.store.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "email not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get email: %v", err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if email.HTML != "" {
		_, _ = w.Write([]byte(email.HTML))
	} else {
		_, _ = w.Write([]byte("<pre>" + email.Text + "</pre>"))
	}
}

func (s *Server) handleGetAttachment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	filename := chi.URLParam(r, "filename")

	email, err := s.store.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "email not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get email: %v", err))
		return
	}

	var found *store.Attachment
	for i := range email.Attachments {
		if email.Attachments[i].Filename == filename {
			found = &email.Attachments[i]
			break
		}
	}
	if found == nil {
		writeError(w, http.StatusNotFound, "attachment not found")
		return
	}

	if len(email.RawMessage) == 0 {
		writeError(w, http.StatusInternalServerError, "raw message not available")
		return
	}

	data, ct, err := mcsmtp.ExtractAttachment(email.RawMessage, filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("extract attachment: %v", err))
		return
	}
	if ct == "" {
		ct = found.ContentType
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	_, _ = w.Write(data)
}

func (s *Server) handleDeleteEmail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.Delete(r.Context(), id); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "email not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("delete email: %v", err))
		return
	}
	s.store.Publish(store.Event{Type: "email.deleted", Payload: map[string]string{"id": id}})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteEmails(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	if err := s.store.DeleteAll(r.Context(), body.IDs); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("delete emails: %v", err))
		return
	}
	s.store.Publish(store.Event{Type: "email.deleted", Payload: map[string]any{"ids": body.IDs}})
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handlePatchEmail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	email, err := s.store.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "email not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get email: %v", err))
		return
	}

	var patch struct {
		Read    *bool    `json:"read"`
		Starred *bool    `json:"starred"`
		Tags    []string `json:"tags"`
		Color   *string  `json:"color"`
		Folder  *string  `json:"folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}

	if patch.Read != nil {
		email.Read = *patch.Read
	}
	if patch.Starred != nil {
		email.Starred = *patch.Starred
	}
	if patch.Tags != nil {
		email.Tags = patch.Tags
	}
	if patch.Color != nil {
		email.Color = *patch.Color
	}
	if patch.Folder != nil {
		email.Folder = *patch.Folder
	}

	if err := s.store.Update(r.Context(), email); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("update email: %v", err))
		return
	}

	s.store.Publish(store.Event{Type: "email.updated", Payload: email})
	writeJSON(w, http.StatusOK, email)
}

func (s *Server) handleAddTag(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	email, err := s.store.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "email not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get email: %v", err))
		return
	}

	var body struct {
		Tag string `json:"tag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid body: %v", err))
		return
	}
	if body.Tag == "" {
		writeError(w, http.StatusBadRequest, "tag is required")
		return
	}

	for _, t := range email.Tags {
		if t == body.Tag {
			writeJSON(w, http.StatusOK, email)
			return
		}
	}
	email.Tags = append(email.Tags, body.Tag)

	if err := s.store.Update(r.Context(), email); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("update email: %v", err))
		return
	}

	s.store.Publish(store.Event{Type: "email.updated", Payload: email})
	writeJSON(w, http.StatusOK, email)
}

func (s *Server) handleRemoveTag(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tag := chi.URLParam(r, "tag")

	email, err := s.store.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "email not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get email: %v", err))
		return
	}

	newTags := email.Tags[:0]
	for _, t := range email.Tags {
		if t != tag {
			newTags = append(newTags, t)
		}
	}
	email.Tags = newTags

	if err := s.store.Update(r.Context(), email); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("update email: %v", err))
		return
	}

	s.store.Publish(store.Event{Type: "email.updated", Payload: email})
	writeJSON(w, http.StatusOK, email)
}

func (s *Server) handleGetSMTPLog(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	email, err := s.store.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "email not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get email: %v", err))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	log := email.SMTPLog
	if log == "" {
		log = "(no SMTP log available — email was received before logging was enabled)"
	}
	_, _ = w.Write([]byte(log))
}

func (s *Server) handleExportEmail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	email, err := s.store.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "email not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("get email: %v", err))
		return
	}

	idShort := id
	if len(idShort) > 8 {
		idShort = idShort[:8]
	}
	filename := fmt.Sprintf("mailcraft-%s.eml", idShort)
	w.Header().Set("Content-Type", "message/rfc822")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))

	if len(email.RawMessage) > 0 {
		_, _ = w.Write(email.RawMessage)
	} else {
		fmt.Fprintf(w, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\n\r\n%s",
			email.From,
			strings.Join(email.To, ", "),
			email.Subject,
			email.ReceivedAt.Format(time.RFC1123Z),
			email.Text)
	}
}

func (s *Server) handleExportEmails(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	var emails []*store.Email
	if len(body.IDs) == 0 {
		all, _, err := s.store.List(r.Context(), store.SearchFilter{Limit: 10000})
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("list emails: %v", err))
			return
		}
		emails = all
	} else {
		for _, id := range body.IDs {
			e, err := s.store.Get(r.Context(), id)
			if err == nil {
				emails = append(emails, e)
			}
		}
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="mailcraft-export.zip"`)

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, email := range emails {
		f, err := zw.Create(email.ID + ".eml")
		if err != nil {
			continue
		}
		if len(email.RawMessage) > 0 {
			_, _ = f.Write(email.RawMessage)
		} else {
			fmt.Fprintf(f, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\n\r\n%s",
				email.From,
				strings.Join(email.To, ", "),
				email.Subject,
				email.ReceivedAt.Format(time.RFC1123Z),
				email.Text)
		}
	}
}
