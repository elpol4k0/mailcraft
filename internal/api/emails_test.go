package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"testing/fstest"

	"github.com/go-chi/chi/v5"

	"mailcraft/internal/api"
	"mailcraft/internal/config"
	"mailcraft/internal/rules"
	"mailcraft/internal/store"
)

func setupServer(t *testing.T) (*api.Server, store.Store) {
	t.Helper()
	st := store.NewMemoryStore(100)
	eng := rules.NewEngine()
	cfg := &config.Config{
		HTTPAddr: "127.0.0.1:0",
	}
	srv := api.NewServer(cfg, st, eng, fstest.MapFS{})
	return srv, st
}

func addTestEmail(t *testing.T, st store.Store, id, from, subject string) *store.Email {
	t.Helper()
	e := &store.Email{
		ID:         id,
		From:       from,
		To:         []string{"to@example.com"},
		Subject:    subject,
		Text:       "test body",
		Tags:       []string{},
		ReceivedAt: time.Now(),
		Size:       100,
	}
	if err := st.Add(context.Background(), e); err != nil {
		t.Fatalf("add email: %v", err)
	}
	return e
}

func doRequest(t *testing.T, srv *api.Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody bytes.Buffer
	if body != nil {
		json.NewEncoder(&reqBody).Encode(body)
	}
	req := httptest.NewRequest(method, path, &reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Use chi router directly via the server's handler
	r := buildTestRouter(srv)
	r.ServeHTTP(w, req)
	return w
}

func buildTestRouter(srv *api.Server) http.Handler {
	r := chi.NewRouter()
	srv.RegisterRoutes(r)
	return r
}

func TestAPIListEmails(t *testing.T) {
	srv, st := setupServer(t)
	addTestEmail(t, st, "id1", "alice@example.com", "Hello")
	addTestEmail(t, st, "id2", "bob@example.com", "World")

	w := doRequest(t, srv, "GET", "/api/v1/emails", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Emails []store.Email `json:"emails"`
		Total  int           `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Total)
	}
}

func TestAPIGetEmail(t *testing.T) {
	srv, st := setupServer(t)
	addTestEmail(t, st, "id1", "alice@example.com", "Test Subject")

	w := doRequest(t, srv, "GET", "/api/v1/emails/id1", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var email store.Email
	if err := json.NewDecoder(w.Body).Decode(&email); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if email.Subject != "Test Subject" {
		t.Errorf("subject = %q, want %q", email.Subject, "Test Subject")
	}
	if !email.Read {
		t.Error("expected email to be marked as read after GET")
	}
}

func TestAPIDeleteEmail(t *testing.T) {
	srv, st := setupServer(t)
	addTestEmail(t, st, "del1", "sender@example.com", "Delete Me")

	w := doRequest(t, srv, "DELETE", "/api/v1/emails/del1", nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}

	_, err := st.Get(context.Background(), "del1")
	if err != store.ErrNotFound {
		t.Error("expected email to be deleted")
	}
}

func TestAPIPatchTags(t *testing.T) {
	srv, st := setupServer(t)
	addTestEmail(t, st, "tag1", "sender@example.com", "Tag Me")

	// Add a tag via POST
	w := doRequest(t, srv, "POST", "/api/v1/emails/tag1/tags", map[string]string{"tag": "important"})
	if w.Code != http.StatusOK {
		t.Errorf("add tag status = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	email, _ := st.Get(context.Background(), "tag1")
	found := false
	for _, t2 := range email.Tags {
		if t2 == "important" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tag 'important', got %v", email.Tags)
	}

	// Remove the tag
	w = doRequest(t, srv, "DELETE", "/api/v1/emails/tag1/tags/important", nil)
	if w.Code != http.StatusOK {
		t.Errorf("remove tag status = %d, want 200", w.Code)
	}

	email, _ = st.Get(context.Background(), "tag1")
	for _, t2 := range email.Tags {
		if t2 == "important" {
			t.Error("tag 'important' should have been removed")
		}
	}
}

func TestAPISearchEmails(t *testing.T) {
	srv, st := setupServer(t)
	addTestEmail(t, st, "s1", "alice@example.com", "Invoice for March")
	addTestEmail(t, st, "s2", "bob@example.com", "Meeting notes")
	addTestEmail(t, st, "s3", "charlie@example.com", "Invoice for April")

	w := doRequest(t, srv, "GET", "/api/v1/emails?q=Invoice", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp struct {
		Total int `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Total)
	}
}
