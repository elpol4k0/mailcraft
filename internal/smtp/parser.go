package smtp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"

	"mailcraft/internal/store"
)

func newBase64Decoder(r io.Reader) io.Reader {
	data, err := io.ReadAll(r)
	if err != nil {
		return strings.NewReader("")
	}
	stripped := strings.ReplaceAll(string(data), "\r\n", "")
	stripped = strings.ReplaceAll(stripped, "\n", "")
	stripped = strings.ReplaceAll(stripped, "\r", "")
	return base64.NewDecoder(base64.StdEncoding, strings.NewReader(stripped))
}

var sanitizer = bluemonday.UGCPolicy()

func ParseEmail(raw []byte) (*store.Email, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("smtp: parse message: %w", err)
	}

	e := &store.Email{
		ID:         uuid.NewString(),
		ReceivedAt: time.Now(),
		RawMessage: raw,
		Size:       len(raw),
		Headers:    make(map[string][]string),
		Tags:       []string{},
	}

	for k, v := range msg.Header {
		e.Headers[k] = v
	}

	e.MessageID = decodeHeader(msg.Header.Get("Message-Id"))
	e.Subject = decodeHeader(msg.Header.Get("Subject"))

	from, err := parseAddress(msg.Header.Get("From"))
	if err == nil {
		e.From = formatAddress(from)
	} else {
		e.From = msg.Header.Get("From")
	}

	e.To = parseAddressList(msg.Header.Get("To"))
	e.CC = parseAddressList(msg.Header.Get("Cc"))
	e.BCC = parseAddressList(msg.Header.Get("Bcc"))

	ct := msg.Header.Get("Content-Type")
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return nil, fmt.Errorf("smtp: read body: %w", err)
	}

	if err := parsePart(ct, msg.Header.Get("Content-Transfer-Encoding"), body, e); err != nil {
		return nil, fmt.Errorf("smtp: parse body: %w", err)
	}

	e.HTML = sanitizer.Sanitize(e.HTML)

	return e, nil
}

func parsePart(ct, cte string, body []byte, e *store.Email) error {
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		e.Text = decodeTransferEncoding(cte, body)
		return nil
	}

	switch {
	case strings.HasPrefix(mediaType, "multipart/"):
		boundary := params["boundary"]
		if boundary == "" {
			return fmt.Errorf("smtp: multipart missing boundary")
		}
		return parseMultipart(mediaType, boundary, body, e)
	case mediaType == "text/plain":
		e.Text = decodeTransferEncoding(cte, body)
	case mediaType == "text/html":
		e.HTML = decodeTransferEncoding(cte, body)
	default:
		att := parseAttachment(mediaType, params, cte, body)
		if att != nil {
			e.Attachments = append(e.Attachments, *att)
		}
	}
	return nil
}

func parseMultipart(mediaType, boundary string, body []byte, e *store.Email) error {
	mr := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("smtp: read multipart: %w", err)
		}

		partBody, err := io.ReadAll(part)
		if err != nil {
			return fmt.Errorf("smtp: read part: %w", err)
		}

		partCT := part.Header.Get("Content-Type")
		partCTE := part.Header.Get("Content-Transfer-Encoding")

		partMediaType, partParams, parseErr := mime.ParseMediaType(partCT)
		if parseErr != nil {
			if e.Text == "" {
				e.Text = decodeTransferEncoding(partCTE, partBody)
			}
			continue
		}

		switch {
		case strings.HasPrefix(partMediaType, "multipart/"):
			subBoundary := partParams["boundary"]
			if subBoundary != "" {
				if err := parseMultipart(partMediaType, subBoundary, partBody, e); err != nil {
					return err
				}
			}
		case partMediaType == "text/plain" && e.Text == "" && !isAttachment(part):
			e.Text = decodeTransferEncoding(partCTE, partBody)
		case partMediaType == "text/html" && e.HTML == "" && !isAttachment(part):
			e.HTML = decodeTransferEncoding(partCTE, partBody)
		default:
			if isAttachment(part) || (!strings.HasPrefix(partMediaType, "text/")) {
				att := parseAttachment(partMediaType, partParams, partCTE, partBody)
				if att != nil {
					att.Filename = getFilename(part, partParams)
					if att.Filename == "" {
						att.Filename = "attachment"
					}
					e.Attachments = append(e.Attachments, *att)
				}
			}
		}
	}

	_ = mediaType
	return nil
}

func isAttachment(part *multipart.Part) bool {
	cd := part.Header.Get("Content-Disposition")
	return strings.HasPrefix(strings.ToLower(cd), "attachment")
}

func getFilename(part *multipart.Part, params map[string]string) string {
	cd := part.Header.Get("Content-Disposition")
	if cd != "" {
		_, cdParams, err := mime.ParseMediaType(cd)
		if err == nil {
			if fn := cdParams["filename"]; fn != "" {
				return decodeHeader(fn)
			}
		}
	}
	if fn := params["name"]; fn != "" {
		return decodeHeader(fn)
	}
	return ""
}

func parseAttachment(mediaType string, params map[string]string, cte string, body []byte) *store.Attachment {
	data := decodeTransferEncodingBytes(cte, body)
	filename := ""
	if n, ok := params["name"]; ok {
		filename = decodeHeader(n)
	}
	if filename == "" {
		filename = "attachment"
	}
	return &store.Attachment{
		Filename:    filename,
		ContentType: mediaType,
		Size:        len(data),
		Data:        data,
	}
}

func decodeTransferEncoding(cte string, body []byte) string {
	return string(decodeTransferEncodingBytes(cte, body))
}

func decodeTransferEncodingBytes(cte string, body []byte) []byte {
	switch strings.ToLower(strings.TrimSpace(cte)) {
	case "quoted-printable":
		decoded, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(body)))
		if err != nil {
			return body
		}
		return decoded
	case "base64":
		decoded, err := io.ReadAll(
			newBase64Decoder(bytes.NewReader(body)),
		)
		if err != nil {
			return body
		}
		return decoded
	}
	return body
}

func decodeHeader(h string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(h)
	if err != nil {
		return h
	}
	return decoded
}

func parseAddress(s string) (*mail.Address, error) {
	if s == "" {
		return nil, fmt.Errorf("empty address")
	}
	return mail.ParseAddress(s)
}

func formatAddress(addr *mail.Address) string {
	if addr.Name != "" {
		return addr.Name + " <" + addr.Address + ">"
	}
	return addr.Address
}

func parseAddressList(s string) []string {
	if s == "" {
		return nil
	}
	addrs, err := mail.ParseAddressList(s)
	if err != nil {
		if s != "" {
			return []string{s}
		}
		return nil
	}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, formatAddress(a))
	}
	return out
}
