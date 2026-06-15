package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

var Version = "dev"

type Config struct {
	SMTPAddr  string
	HTTPAddr  string
	MaxEmails int
	MaxSize   int64
	BasePath  string
	LogLevel  string
	TLSCert   string
	TLSKey    string

	LevelVar *slog.LevelVar
}

func Load() (*Config, error) {
	cfg := &Config{LevelVar: &slog.LevelVar{}}

	fs := flag.NewFlagSet("mailcraft", flag.ContinueOnError)

	smtpAddr := fs.String("smtp-addr", envOr("MC_SMTP_ADDR", ":1025"), "SMTP bind address")
	httpAddr := fs.String("http-addr", envOr("MC_HTTP_ADDR", ":8025"), "HTTP bind address")
	maxEmails := fs.Int("max-emails", envIntOr("MC_MAX_EMAILS", 5000), "maximum number of emails to keep")
	maxSize := fs.Int64("max-size", envInt64Or("MC_MAX_SIZE", 26214400), "maximum message size in bytes")
	basePath := fs.String("base-path", envOr("MC_BASE_PATH", "/"), "base path for reverse proxy")
	logLevel := fs.String("log-level", envOr("MC_LOG_LEVEL", "info"), "log level: debug, info, warn, error")
	tlsCert := fs.String("tls-cert", envOr("MC_TLS_CERT", ""), "path to TLS certificate file (enables STARTTLS)")
	tlsKey := fs.String("tls-key", envOr("MC_TLS_KEY", ""), "path to TLS key file (enables STARTTLS)")
	version := fs.Bool("version", false, "print version and exit")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, fmt.Errorf("config: parse flags: %w", err)
	}

	if *version {
		fmt.Printf("mailcraft %s\n", Version)
		os.Exit(0)
	}

	cfg.SMTPAddr = *smtpAddr
	cfg.HTTPAddr = *httpAddr
	cfg.MaxEmails = *maxEmails
	cfg.MaxSize = *maxSize
	cfg.BasePath = *basePath
	cfg.TLSCert = *tlsCert
	cfg.TLSKey = *tlsKey
	cfg.SetLogLevel(*logLevel)

	return cfg, nil
}

func (c *Config) SetLogLevel(level string) {
	c.LogLevel = level
	switch level {
	case "debug":
		c.LevelVar.Set(slog.LevelDebug)
	case "warn":
		c.LevelVar.Set(slog.LevelWarn)
	case "error":
		c.LevelVar.Set(slog.LevelError)
	default:
		c.LevelVar.Set(slog.LevelInfo)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return def
}

func envInt64Or(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return n
		}
	}
	return def
}
