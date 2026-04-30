package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/html"
)

type HTMLCheckWarning struct {
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Count       int     `json:"count"`
	Support     string  `json:"support"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
	Clients     string  `json:"clients"`
}

type HTMLCheckResult struct {
	Score    int                `json:"score"`
	Warnings []HTMLCheckWarning `json:"warnings"`
	Total    int                `json:"total"`
}

type elementRule struct {
	support     string
	description string
	clients     string
}

type cssRule struct {
	pattern     string
	support     string
	description string
	clients     string
}

type attrRule struct {
	support     string
	description string
	clients     string
	minCount    int
}

var elementRules = map[string]elementRule{
	"script":   {"none", "JavaScript is not supported in any email client", "All"},
	"video":    {"none", "Video is not supported in most email clients", "Gmail, Outlook, Yahoo"},
	"audio":    {"none", "Audio is not supported in most email clients", "Gmail, Outlook, Yahoo"},
	"canvas":   {"none", "Canvas is not supported in email clients", "All"},
	"svg":      {"none", "SVG has very limited support in email clients", "Gmail, Outlook"},
	"form":     {"none", "Forms are not supported in most email clients", "Gmail, Outlook, Yahoo"},
	"input":    {"none", "Input elements are stripped by most email clients", "Gmail, Outlook"},
	"textarea": {"none", "Textarea is stripped by most email clients", "All"},
	"select":   {"none", "Select is stripped by most email clients", "All"},
	"button":   {"none", "Button elements are not supported in most email clients", "Gmail, Outlook"},
	"object":   {"none", "Object embeds are not supported in email clients", "All"},
	"embed":    {"none", "Embed is not supported in email clients", "All"},
	"iframe":   {"none", "iframes are blocked by email clients", "All"},
	"picture":  {"partial", "Picture element has partial support in email clients", "Outlook 2007-2019"},
	"details":  {"partial", "Details/summary elements not supported in most clients", "Gmail, Outlook"},
	"summary":  {"partial", "Summary element not supported in most clients", "Gmail, Outlook"},
}

var cssRules = []cssRule{
	{"display: flex", "partial", "Flexbox has limited support, not supported in Outlook", "Outlook 2007-2019"},
	{"display:flex", "partial", "Flexbox has limited support, not supported in Outlook", "Outlook 2007-2019"},
	{"display: grid", "none", "CSS Grid is not supported in most email clients", "Gmail, Outlook, Yahoo"},
	{"display:grid", "none", "CSS Grid is not supported in most email clients", "Gmail, Outlook, Yahoo"},
	{"position: fixed", "none", "Fixed positioning is not supported in email clients", "All"},
	{"position: sticky", "none", "Sticky positioning is not supported in email clients", "All"},
	{"transform:", "partial", "CSS transforms have limited support", "Outlook 2007-2019"},
	{"animation:", "partial", "CSS animations not supported in Gmail/Outlook", "Gmail, Outlook"},
	{"@keyframes", "partial", "CSS animations not supported in Gmail/Outlook", "Gmail, Outlook"},
	{"transition:", "partial", "CSS transitions not supported in Outlook", "Outlook 2007-2019"},
	{"calc(", "partial", "CSS calc() has limited support in email clients", "Outlook 2007-2019"},
	{"var(", "partial", "CSS variables (custom properties) not supported in most email clients", "Gmail, Outlook, Yahoo"},
	{"@media", "partial", "Media queries not supported in Gmail (non-responsive)", "Gmail"},
	{"filter:", "partial", "CSS filters have very limited support in email clients", "Most clients"},
}

var attrRules = map[string]attrRule{
	"class": {"partial", "class attributes are stripped by Gmail", "Gmail", 5},
	"id":    {"partial", "id attributes may be stripped or prefixed by email clients", "Gmail, Yahoo", 0},
}

func (s *Server) handleHTMLCheck(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	email, err := s.store.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "email not found")
		return
	}
	if email.HTML == "" {
		writeJSON(w, http.StatusOK, HTMLCheckResult{Score: 100, Warnings: []HTMLCheckWarning{}, Total: 0})
		return
	}
	result := checkHTML(email.HTML)
	writeJSON(w, http.StatusOK, result)
}

func checkHTML(htmlContent string) HTMLCheckResult {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return HTMLCheckResult{Score: 100, Warnings: []HTMLCheckWarning{}, Total: 0}
	}

	elementCounts := make(map[string]int)
	attrCounts := make(map[string]int)
	totalElements := 0

	var cssTexts []string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			totalElements++
			tag := strings.ToLower(n.Data)
			elementCounts[tag]++

			for _, attr := range n.Attr {
				key := strings.ToLower(attr.Key)
				if key == "style" {
					cssTexts = append(cssTexts, strings.ToLower(attr.Val))
				}
				if _, ok := attrRules[key]; ok {
					attrCounts[key]++
				}
			}

			if tag == "style" {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						cssTexts = append(cssTexts, strings.ToLower(c.Data))
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	combinedCSS := strings.Join(cssTexts, "\n")

	type cssKey struct{ description string }
	cssMatchCounts := make(map[cssKey]struct {
		count   int
		rule    cssRule
		matched bool
	})

	for _, cr := range cssRules {
		key := cssKey{cr.description}
		existing := cssMatchCounts[key]
		if strings.Contains(combinedCSS, cr.pattern) {
			count := strings.Count(combinedCSS, cr.pattern)
			if count > existing.count || !existing.matched {
				existing.count += count
				existing.rule = cr
				existing.matched = true
			}
			cssMatchCounts[key] = existing
		}
	}

	var warnings []HTMLCheckWarning

	for tag, rule := range elementRules {
		count, ok := elementCounts[tag]
		if !ok || count == 0 {
			continue
		}
		var score float64
		if totalElements > 0 {
			score = float64(count) / float64(totalElements)
		}
		warnings = append(warnings, HTMLCheckWarning{
			Type:        "element",
			Name:        "<" + tag + ">",
			Count:       count,
			Support:     rule.support,
			Score:       score,
			Description: rule.description,
			Clients:     rule.clients,
		})
	}

	for key, m := range cssMatchCounts {
		if !m.matched {
			continue
		}
		_ = key
		var score float64
		if totalElements > 0 {
			score = float64(m.count) / float64(totalElements)
		}
		warnings = append(warnings, HTMLCheckWarning{
			Type:        "css",
			Name:        m.rule.pattern,
			Count:       m.count,
			Support:     m.rule.support,
			Score:       score,
			Description: m.rule.description,
			Clients:     m.rule.clients,
		})
	}

	for attr, rule := range attrRules {
		count, ok := attrCounts[attr]
		if !ok || count == 0 || count <= rule.minCount {
			continue
		}
		var score float64
		if totalElements > 0 {
			score = float64(count) / float64(totalElements)
		}
		warnings = append(warnings, HTMLCheckWarning{
			Type:        "attribute",
			Name:        attr + "=",
			Count:       count,
			Support:     rule.support,
			Score:       score,
			Description: rule.description,
			Clients:     rule.clients,
		})
	}

	finalScore := 100.0
	for _, w := range warnings {
		var reduction float64
		if totalElements > 0 {
			ratio := float64(w.Count) / float64(totalElements)
			if w.Support == "none" {
				reduction = ratio * 30
				if reduction > 30 {
					reduction = 30
				}
			} else if w.Support == "partial" {
				reduction = ratio * 15
				if reduction > 15 {
					reduction = 15
				}
			}
		}
		finalScore -= reduction
	}
	if finalScore < 0 {
		finalScore = 0
	}
	if finalScore > 100 {
		finalScore = 100
	}

	sort.Slice(warnings, func(i, j int) bool {
		si := warnings[i].Support
		sj := warnings[j].Support
		if si != sj {
			if si == "none" {
				return true
			}
			if sj == "none" {
				return false
			}
		}
		return warnings[i].Count > warnings[j].Count
	})

	if warnings == nil {
		warnings = []HTMLCheckWarning{}
	}

	return HTMLCheckResult{
		Score:    int(finalScore),
		Warnings: warnings,
		Total:    totalElements,
	}
}
