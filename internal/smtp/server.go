package smtp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mailcraft/internal/store"
)

type Handler func(ctx context.Context, raw []byte, from string, to []string, smtpLog string) error

type Server struct {
	addr       string
	maxSize    int64
	maxConns   int
	tlsCfg     *tls.Config
	handler    Handler
	listener   net.Listener
	wg         sync.WaitGroup
	connCount  atomic.Int64
	shutdownCh chan struct{}
}

func NewServer(addr string, maxSize int64, maxConns int, tlsCfg *tls.Config, handler Handler) *Server {
	if maxSize <= 0 {
		maxSize = 26214400
	}
	if maxConns <= 0 {
		maxConns = 100
	}
	return &Server{
		addr:       addr,
		maxSize:    maxSize,
		maxConns:   maxConns,
		tlsCfg:     tlsCfg,
		handler:    handler,
		shutdownCh: make(chan struct{}),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("smtp: listen %s: %w", s.addr, err)
	}
	s.listener = ln
	slog.Info("smtp server listening", "addr", s.addr)
	go s.acceptLoop()
	return nil
}

func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.addr
}

func (s *Server) Shutdown(ctx context.Context) error {
	close(s.shutdownCh)
	if s.listener != nil {
		s.listener.Close()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdownCh:
				return
			default:
				slog.Error("smtp: accept error", "err", err)
				continue
			}
		}

		if s.connCount.Load() >= int64(s.maxConns) {
			conn.Close()
			continue
		}

		s.wg.Add(1)
		s.connCount.Add(1)
		go func() {
			defer s.wg.Done()
			defer s.connCount.Add(-1)
			s.handleConn(conn)
		}()
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Minute))

	sess := &session{
		conn:    conn,
		rw:      bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
		maxSize: s.maxSize,
		tlsCfg:  s.tlsCfg,
	}
	sess.run(context.Background(), s.handler)
}

type session struct {
	conn    net.Conn
	rw      *bufio.ReadWriter
	maxSize int64
	tlsCfg  *tls.Config
	isTLS   bool

	from    string
	to      []string
	data    []byte
	smtpLog strings.Builder
}

func (s *session) writeLine(line string) {
	s.smtpLog.WriteString("S: " + line + "\n")
	_, _ = s.rw.WriteString(line + "\r\n")
	s.rw.Flush()
}

func (s *session) logClient(line string) {
	s.smtpLog.WriteString("C: " + line + "\n")
}

func (s *session) run(ctx context.Context, handler Handler) {
	s.writeLine("220 mailcraft ESMTP ready")
	s.loop(ctx, handler)
}

func (s *session) loop(ctx context.Context, handler Handler) {
	scanner := bufio.NewScanner(s.rw.Reader)
	for scanner.Scan() {
		line := scanner.Text()
		s.logClient(line)
		cmd := strings.ToUpper(strings.TrimSpace(line))

		switch {
		case cmd == "QUIT":
			s.writeLine("221 Bye")
			return
		case cmd == "NOOP":
			s.writeLine("250 OK")
		case cmd == "RSET":
			s.from = ""
			s.to = nil
			s.data = nil
			s.writeLine("250 OK")
		case strings.HasPrefix(cmd, "EHLO") || strings.HasPrefix(cmd, "HELO"):
			s.writeLine("250-mailcraft")
			s.writeLine("250-SIZE " + fmt.Sprintf("%d", s.maxSize))
			s.writeLine("250-8BITMIME")
			if s.tlsCfg != nil && !s.isTLS {
				s.writeLine("250-STARTTLS")
			}
			s.writeLine("250 AUTH PLAIN LOGIN")
		case strings.HasPrefix(cmd, "STARTTLS"):
			if s.tlsCfg == nil || s.isTLS {
				s.writeLine("454 TLS not available")
				continue
			}
			s.writeLine("220 Ready to start TLS")
			tlsConn := tls.Server(s.conn, s.tlsCfg)
			if err := tlsConn.Handshake(); err != nil {
				slog.Debug("smtp: tls handshake failed", "err", err)
				return
			}
			s.conn = tlsConn
			s.rw = bufio.NewReadWriter(bufio.NewReader(tlsConn), bufio.NewWriter(tlsConn))
			s.isTLS = true
			s.from = ""
			s.to = nil
			s.loop(ctx, handler)
			return
		case strings.HasPrefix(cmd, "AUTH"):
			s.handleAuth(line)
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			s.from = extractAngle(line[len("MAIL FROM:"):])
			s.writeLine("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			rcpt := extractAngle(line[len("RCPT TO:"):])
			s.to = append(s.to, rcpt)
			s.writeLine("250 OK")
		case cmd == "DATA":
			if err := s.handleData(ctx, handler); err != nil {
				slog.Error("smtp: data error", "err", err)
			}
		default:
			s.writeLine("500 Unrecognized command")
		}
	}
}

func (s *session) handleAuth(line string) {
	upper := strings.ToUpper(line)
	if strings.Contains(upper, "PLAIN") {
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			s.writeLine("235 Authentication successful")
			return
		}
		s.writeLine("334 ")
		_, _ = s.rw.ReadString('\n')
		s.writeLine("235 Authentication successful")
		return
	}
	if strings.Contains(upper, "LOGIN") {
		s.writeLine("334 VXNlcm5hbWU6")
		_, _ = s.rw.ReadString('\n')
		s.writeLine("334 UGFzc3dvcmQ6")
		_, _ = s.rw.ReadString('\n')
		s.writeLine("235 Authentication successful")
		return
	}
	s.writeLine("504 Authentication mechanism not supported")
}

func (s *session) handleData(ctx context.Context, handler Handler) error {
	s.writeLine("354 Start mail input; end with <CRLF>.<CRLF>")

	var buf bytes.Buffer
	var totalSize int64

	for {
		line, err := s.rw.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("smtp: read data: %w", err)
		}

		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "." {
			break
		}

		if strings.HasPrefix(trimmed, "..") {
			trimmed = trimmed[1:]
		}

		totalSize += int64(len(line))
		if totalSize > s.maxSize {
			s.writeLine("552 Message too large")
			for {
				l, err := s.rw.ReadString('\n')
				if err != nil || strings.TrimRight(l, "\r\n") == "." {
					break
				}
			}
			return nil
		}

		buf.WriteString(trimmed + "\r\n")
	}

	raw := buf.Bytes()
	if handler != nil {
		smtpLog := s.smtpLog.String()
		if err := handler(ctx, raw, s.from, s.to, smtpLog); err != nil {
			s.writeLine("451 Requested action aborted")
			return nil
		}
	}

	s.writeLine("250 OK")
	s.from = ""
	s.to = nil
	s.data = nil
	return nil
}

func extractAngle(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, " "); idx != -1 {
		s = s[:idx]
	}
	s = strings.Trim(s, "<>")
	return strings.TrimSpace(s)
}

func DefaultHandler(st store.Store, engine interface {
	Apply(ctx context.Context, email *store.Email, st store.Store) bool
}) Handler {
	return func(ctx context.Context, raw []byte, from string, to []string, smtpLog string) error {
		email, err := ParseEmail(raw)
		if err != nil {
			slog.Warn("smtp: failed to parse email", "err", err, "from", from)
			email = &store.Email{
				ID:         fmt.Sprintf("err-%d", time.Now().UnixNano()),
				From:       from,
				To:         to,
				Subject:    "(unparseable)",
				Text:       string(raw),
				RawMessage: raw,
				Size:       len(raw),
				ReceivedAt: time.Now(),
				Tags:       []string{},
				Headers:    make(map[string][]string),
			}
		}

		if from != "" && email.From == "" {
			email.From = from
		}
		if len(to) > 0 && len(email.To) == 0 {
			email.To = to
		}

		email.SMTPLog = smtpLog

		for i := range email.Attachments {
			email.Attachments[i].Data = nil
		}

		deleted := engine.Apply(ctx, email, st)
		if deleted {
			slog.Info("smtp: email deleted by rule", "from", email.From, "subject", email.Subject)
			return nil
		}

		if err := st.Add(ctx, email); err != nil {
			return fmt.Errorf("smtp: store email: %w", err)
		}

		st.Publish(store.Event{Type: "email.new", Payload: email})
		slog.Info("smtp: email received", "from", email.From, "subject", email.Subject, "id", email.ID)
		return nil
	}
}
