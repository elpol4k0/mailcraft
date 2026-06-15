package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mailcraft/internal/api"
	"mailcraft/internal/config"
	"mailcraft/internal/rules"
	mcsmtp "mailcraft/internal/smtp"
	"mailcraft/internal/store"
	"mailcraft/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LevelVar})))

	var tlsCfg *tls.Config
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			slog.Error("tls: failed to load cert/key", "err", err)
			os.Exit(1)
		}
		tlsCfg = &tls.Config{Certificates: []tls.Certificate{cert}}
		slog.Info("tls: STARTTLS enabled")
	}

	st := store.NewMemoryStore(cfg.MaxEmails)
	defer st.Close()

	eng := rules.NewEngine()
	ruleList, err := st.ListRules(context.Background())
	if err == nil && len(ruleList) > 0 {
		eng.SetRules(ruleList)
	}

	smtpHandler := mcsmtp.DefaultHandler(st, eng)
	smtpSrv := mcsmtp.NewServer(cfg.SMTPAddr, cfg.MaxSize, 100, tlsCfg, smtpHandler)
	if err := smtpSrv.Start(); err != nil {
		slog.Error("smtp server error", "err", err)
		os.Exit(1)
	}

	httpSrv := api.NewServer(cfg, st, eng, ui.Assets)
	if err := httpSrv.Start(); err != nil {
		slog.Error("http server error", "err", err)
		os.Exit(1)
	}

	slog.Info("mailcraft started", "smtp", cfg.SMTPAddr, "http", cfg.HTTPAddr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := smtpSrv.Shutdown(ctx); err != nil {
		slog.Error("smtp shutdown error", "err", err)
	}
	if err := httpSrv.Shutdown(ctx); err != nil {
		slog.Error("http shutdown error", "err", err)
	}
}
