package api

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"mailcraft/internal/config"
	"mailcraft/internal/rules"
	"mailcraft/internal/store"
)

type Server struct {
	cfg        *config.Config
	store      store.Store
	engine     *rules.Engine
	assets     fs.FS
	httpServer *http.Server
	startTime  time.Time
}

func NewServer(cfg *config.Config, st store.Store, eng *rules.Engine, assets fs.FS) *Server {
	return &Server{
		cfg:       cfg,
		store:     st,
		engine:    eng,
		assets:    assets,
		startTime: time.Now(),
	}
}

func (s *Server) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		r.Get("/stats", s.handleStats)
		r.Get("/events", s.handleEvents)
		r.Get("/config", s.handleConfig)

		r.Get("/emails", s.handleListEmails)
		r.Delete("/emails", s.handleDeleteEmails)
		r.Get("/emails/{id}", s.handleGetEmail)
		r.Delete("/emails/{id}", s.handleDeleteEmail)
		r.Patch("/emails/{id}", s.handlePatchEmail)
		r.Get("/emails/{id}/raw", s.handleGetEmailRaw)
		r.Get("/emails/{id}/html", s.handleGetEmailHTML)
		r.Get("/emails/{id}/attachments/{filename}", s.handleGetAttachment)
		r.Post("/emails/{id}/tags", s.handleAddTag)
		r.Delete("/emails/{id}/tags/{tag}", s.handleRemoveTag)
		r.Get("/emails/{id}/smtplog", s.handleGetSMTPLog)
		r.Get("/emails/{id}/linkcheck", s.handleLinkCheck)
		r.Post("/emails/{id}/linkcheck", s.handleLinkCheck)
		r.Get("/emails/{id}/htmlcheck", s.handleHTMLCheck)
		r.Get("/emails/{id}/spamcheck", s.handleSpamCheck)
		r.Get("/emails/{id}/export", s.handleExportEmail)
		r.Post("/emails/export", s.handleExportEmails)

		r.Get("/rules", s.handleListRules)
		r.Post("/rules", s.handleCreateRule)
		r.Post("/rules/reorder", s.handleReorderRules)
		r.Get("/rules/{id}", s.handleGetRule)
		r.Put("/rules/{id}", s.handleUpdateRule)
		r.Patch("/rules/{id}", s.handlePatchRule)
		r.Delete("/rules/{id}", s.handleDeleteRule)
		r.Post("/rules/{id}/test", s.handleTestRule)

		r.Get("/tags", s.handleListTags)
		r.Delete("/tags/{name}", s.handleDeleteTag)
		r.Put("/tags/{name}", s.handleRenameTag)

		r.Get("/folders", s.handleListFolders)
		r.Put("/folders/{name}", s.handleRenameFolder)
		r.Delete("/folders/{name}", s.handleDeleteFolder)

		r.Patch("/config", s.handlePatchConfig)
	})
}

func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		if _, err := fsys.Open(path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		r2 := r.Clone(r.Context())
		r2.URL.Path = "/index.html"
		fileServer.ServeHTTP(w, r2)
	})
}

func (s *Server) Start() error {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(loggingMiddleware)
	r.Use(corsMiddleware)

	s.RegisterRoutes(r)

	r.Handle("/*", spaHandler(s.assets))

	s.httpServer = &http.Server{
		Addr:    s.cfg.HTTPAddr,
		Handler: r,
	}

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("http server error: %v\n", err)
		}
	}()

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
