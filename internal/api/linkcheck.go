package api

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/html"
)

type LinkResult struct {
	URL        string `json:"url"`
	Type       string `json:"type"`
	Status     int    `json:"status"`
	StatusText string `json:"status_text"`
	RedirectTo string `json:"redirect_to,omitempty"`
	ResponseMs int64  `json:"response_ms"`
	Error      string `json:"error,omitempty"`
}

type linkEntry struct {
	URL  string
	Type string
}

func (s *Server) handleLinkCheck(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	followRedirects := r.URL.Query().Get("follow") == "true"

	email, err := s.store.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "email not found")
		return
	}

	links := extractLinks(email.HTML, email.Text)
	results := checkLinks(r.Context(), links, followRedirects)
	writeJSON(w, http.StatusOK, map[string]any{"links": results, "total": len(results)})
}

var urlRegex = regexp.MustCompile(`https?://[^\s<>"{}|\\^[\]` + "`" + `]+`)

func extractLinks(htmlContent, textContent string) []linkEntry {
	seen := make(map[string]bool)
	var links []linkEntry

	add := func(rawURL, linkType string) {
		rawURL = strings.TrimRight(rawURL, ".,;:!?)")
		u, err := url.Parse(rawURL)
		if err != nil {
			return
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return
		}
		if seen[rawURL] {
			return
		}
		seen[rawURL] = true
		links = append(links, linkEntry{URL: rawURL, Type: linkType})
	}

	if htmlContent != "" {
		doc, err := html.Parse(strings.NewReader(htmlContent))
		if err == nil {
			var walk func(*html.Node)
			walk = func(n *html.Node) {
				if n.Type == html.ElementNode {
					switch n.Data {
					case "a":
						for _, attr := range n.Attr {
							if attr.Key == "href" {
								add(attr.Val, "link")
							}
						}
					case "img":
						for _, attr := range n.Attr {
							if attr.Key == "src" {
								add(attr.Val, "image")
							}
						}
					case "link":
						isStylesheet := false
						href := ""
						for _, attr := range n.Attr {
							if attr.Key == "rel" && strings.Contains(strings.ToLower(attr.Val), "stylesheet") {
								isStylesheet = true
							}
							if attr.Key == "href" {
								href = attr.Val
							}
						}
						if href != "" {
							if isStylesheet {
								add(href, "stylesheet")
							} else {
								add(href, "link")
							}
						}
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
			}
			walk(doc)
		}
	}

	if textContent != "" {
		matches := urlRegex.FindAllString(textContent, -1)
		for _, m := range matches {
			add(m, "link")
		}
	}

	if len(links) > 100 {
		links = links[:100]
	}
	return links
}

func checkLinks(ctx context.Context, links []linkEntry, followRedirects bool) []LinkResult {
	results := make([]LinkResult, len(links))

	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	noRedirectClient := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	followClient := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	client := noRedirectClient
	if followRedirects {
		client = followClient
	}

	for i, link := range links {
		i, link := i, link
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			res := LinkResult{
				URL:  link.URL,
				Type: link.Type,
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, link.URL, nil)
			if err != nil {
				res.Error = err.Error()
				results[i] = res
				return
			}
			req.Header.Set("User-Agent", "MailCraft/1.0 LinkChecker")

			resp, err := client.Do(req)
			res.ResponseMs = time.Since(start).Milliseconds()
			if err != nil {
				res.Error = err.Error()
				results[i] = res
				return
			}
			resp.Body.Close()

			res.Status = resp.StatusCode
			res.StatusText = resp.Status

			if !followRedirects && (resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 307 || resp.StatusCode == 308) {
				res.RedirectTo = resp.Header.Get("Location")
			}

			results[i] = res
		}()
	}

	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].URL < results[j].URL
	})

	return results
}
