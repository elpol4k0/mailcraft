package smtp_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	mcsmtp "mailcraft/internal/smtp"
	"mailcraft/internal/store"
)

type noopEngine struct{}

func (n *noopEngine) Apply(_ context.Context, _ *store.Email, _ store.Store) bool {
	return false
}

func startTestServer(t *testing.T, st store.Store) string {
	t.Helper()
	eng := &noopEngine{}
	handler := mcsmtp.DefaultHandler(st, eng)
	srv := mcsmtp.NewServer("127.0.0.1:0", 26214400, 100, handler)
	if err := srv.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	})
	return srv.Addr()
}

func sendRawSMTP(t *testing.T, addr, from, to, data string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	smtpExchange(t, conn, from, []string{to}, data)
}

func smtpExchange(t *testing.T, conn net.Conn, from string, to []string, data string) {
	t.Helper()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	readLine := func() string {
		buf := make([]byte, 4096)
		var result strings.Builder
		for {
			n, err := conn.Read(buf)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			result.Write(buf[:n])
			s := result.String()
			if strings.HasSuffix(strings.TrimRight(s, "\r\n"), "\r\n") || strings.Count(s, "\n") > 0 {
				lines := strings.Split(strings.TrimRight(s, "\r\n"), "\n")
				lastLine := strings.TrimRight(lines[len(lines)-1], "\r")
				if len(lastLine) > 3 && lastLine[3] == ' ' {
					return s
				}
				if len(lastLine) > 3 && lastLine[3] == '-' {
					continue
				}
				return s
			}
		}
	}

	sendLine := func(line string) {
		fmt.Fprintf(conn, "%s\r\n", line)
	}

	readLine() // banner
	sendLine("EHLO test")
	readLine()
	sendLine("MAIL FROM:<" + from + ">")
	readLine()
	for _, rcpt := range to {
		sendLine("RCPT TO:<" + rcpt + ">")
		readLine()
	}
	sendLine("DATA")
	readLine()
	// Send data line by line
	lines := strings.Split(data, "\n")
	for _, l := range lines {
		l = strings.TrimRight(l, "\r")
		if l == "." {
			l = ".."
		}
		fmt.Fprintf(conn, "%s\r\n", l)
	}
	sendLine(".")
	readLine()
	sendLine("QUIT")
}

func waitForEmail(t *testing.T, st store.Store, timeout time.Duration) *store.Email {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		emails, total, err := st.List(context.Background(), store.SearchFilter{})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if total > 0 {
			return emails[0]
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for email")
	return nil
}

func TestSMTPBasicReceive(t *testing.T) {
	st := store.NewMemoryStore(100)
	addr := startTestServer(t, st)

	msg := "From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: Test Email\r\n\r\nHello, this is a test email.\r\n"
	sendRawSMTP(t, addr, "sender@example.com", "recipient@example.com", msg)

	email := waitForEmail(t, st, 5*time.Second)
	if email.Subject != "Test Email" {
		t.Errorf("subject = %q, want %q", email.Subject, "Test Email")
	}
	if !strings.Contains(email.From, "sender@example.com") {
		t.Errorf("from = %q, want to contain sender@example.com", email.From)
	}
}

func TestSMTPMIMEMultipart(t *testing.T) {
	st := store.NewMemoryStore(100)
	addr := startTestServer(t, st)

	boundary := "==boundary123=="
	attachContent := "Hello attachment"
	attachB64 := base64.StdEncoding.EncodeToString([]byte(attachContent))

	msg := strings.Join([]string{
		"From: sender@example.com",
		"To: recipient@example.com",
		"Subject: Multipart Test",
		"MIME-Version: 1.0",
		"Content-Type: multipart/mixed; boundary=\"" + boundary + "\"",
		"",
		"--" + boundary,
		"Content-Type: multipart/alternative; boundary=\"alt123\"",
		"",
		"--alt123",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Plain text body",
		"--alt123",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>HTML body</p>",
		"--alt123--",
		"--" + boundary,
		"Content-Type: application/octet-stream; name=\"test.txt\"",
		"Content-Disposition: attachment; filename=\"test.txt\"",
		"Content-Transfer-Encoding: base64",
		"",
		attachB64,
		"--" + boundary + "--",
	}, "\r\n")

	sendRawSMTP(t, addr, "sender@example.com", "recipient@example.com", msg)

	email := waitForEmail(t, st, 5*time.Second)
	if email.Subject != "Multipart Test" {
		t.Errorf("subject = %q, want %q", email.Subject, "Multipart Test")
	}
	if !strings.Contains(email.Text, "Plain text body") {
		t.Errorf("text = %q, want to contain 'Plain text body'", email.Text)
	}
	if !strings.Contains(email.HTML, "HTML body") {
		t.Errorf("html = %q, want to contain 'HTML body'", email.HTML)
	}
	if len(email.Attachments) == 0 {
		t.Errorf("expected 1 attachment, got 0")
	} else {
		att := email.Attachments[0]
		if att.Filename != "test.txt" {
			t.Errorf("attachment filename = %q, want %q", att.Filename, "test.txt")
		}
	}
}

func TestSMTPAuthAccepted(t *testing.T) {
	st := store.NewMemoryStore(100)
	addr := startTestServer(t, st)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(10 * time.Second))

	br := bufio.NewReader(conn)

	readSMTPResp := func() string {
		var sb strings.Builder
		for {
			line, err := br.ReadString('\n')
			sb.WriteString(line)
			if err != nil {
				break
			}
			// Multi-line SMTP: "250-..." continues, "250 ..." ends
			if len(line) >= 4 && line[3] == ' ' {
				break
			}
			if len(line) < 4 {
				break
			}
		}
		return sb.String()
	}

	sendLine := func(l string) {
		fmt.Fprintf(conn, "%s\r\n", l)
	}

	readSMTPResp() // banner

	sendLine("EHLO test")
	resp := readSMTPResp()
	if !strings.Contains(resp, "250") {
		t.Errorf("EHLO failed: %q", resp)
	}

	// AUTH PLAIN with credentials inline
	creds := base64.StdEncoding.EncodeToString([]byte("\x00user\x00password"))
	sendLine("AUTH PLAIN " + creds)
	resp = readSMTPResp()
	if !strings.Contains(resp, "235") {
		t.Errorf("AUTH PLAIN failed: %q", resp)
	}

	sendLine("QUIT")
}

func TestSMTPMaxSize(t *testing.T) {
	st := store.NewMemoryStore(100)
	eng := &noopEngine{}
	handler := mcsmtp.DefaultHandler(st, eng)
	srv := mcsmtp.NewServer("127.0.0.1:0", 1024, 100, handler)
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	addr := srv.Addr()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(10 * time.Second))

	buf := make([]byte, 4096)
	readResp := func() string {
		var result strings.Builder
		for {
			n, err := conn.Read(buf)
			if err != nil {
				break
			}
			result.Write(buf[:n])
			s := result.String()
			if strings.Contains(s, "\n") {
				lines := strings.Split(strings.TrimRight(s, "\r\n"), "\n")
				last := strings.TrimRight(lines[len(lines)-1], "\r")
				if len(last) >= 4 && last[3] == ' ' {
					return s
				}
				if len(last) >= 4 && last[3] == '-' {
					continue
				}
				return s
			}
		}
		return result.String()
	}

	sendLine := func(l string) { fmt.Fprintf(conn, "%s\r\n", l) }

	readResp()
	sendLine("EHLO test")
	readResp()
	sendLine("MAIL FROM:<big@example.com>")
	readResp()
	sendLine("RCPT TO:<recv@example.com>")
	readResp()
	sendLine("DATA")
	readResp()

	// Send a body larger than 1024 bytes
	bigLine := strings.Repeat("X", 100)
	sendLine("Subject: big\r\n")
	for i := 0; i < 20; i++ {
		sendLine(bigLine)
	}
	sendLine(".")
	resp := readResp()

	if !strings.Contains(resp, "552") {
		t.Errorf("expected 552 too large, got: %q", resp)
	}

	// No email should be stored
	_, total, _ := st.List(context.Background(), store.SearchFilter{})
	if total != 0 {
		t.Errorf("expected 0 emails stored, got %d", total)
	}

	_ = bytes.Compare
}
