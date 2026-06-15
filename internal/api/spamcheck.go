package api

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"

	"mailcraft/internal/store"
)

// SpamCheckResult is a content-focused analysis of an email message.
// It is NOT a deliverability/spam-probability prediction: things like IP and
// domain reputation, real DKIM/SPF/DMARC validation, and recipient behaviour
// cannot be evaluated by a local mail catcher. The score only reflects
// message-level content heuristics a developer can actually act on.
type SpamCheckResult struct {
	Score  float64         `json:"score"`
	Level  string          `json:"level"`
	Checks []SpamCheckItem `json:"checks"`
}

type SpamCheckItem struct {
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
	Pass        bool    `json:"pass"`
	// Info marks a check as informational only: it is shown to the user but
	// does NOT contribute to the content score (e.g. auth headers that are
	// added by the sending MTA, not by the message content).
	Info bool `json:"info"`
}

func (s *Server) handleSpamCheck(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	email, err := s.store.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "email not found")
		return
	}
	result := runSpamCheck(email)
	writeJSON(w, http.StatusOK, result)
}

var tagStripper = regexp.MustCompile(`<[^>]+>`)
var excessivePunct = regexp.MustCompile(`[!?]{3,}`)

func runSpamCheck(email *store.Email) SpamCheckResult {
	var checks []SpamCheckItem
	add := func(name, category string, score float64, desc string, pass bool) {
		checks = append(checks, SpamCheckItem{
			Name: name, Category: category, Score: score, Description: desc, Pass: pass,
		})
	}
	info := func(name, desc string, pass bool) {
		checks = append(checks, SpamCheckItem{
			Name: name, Category: "authentication", Score: 0, Description: desc, Pass: pass, Info: true,
		})
	}

	subject := email.Subject
	subjectLower := strings.ToLower(subject)

	// Trigger words commonly flagged by content filters.
	spamWords := []string{
		"free", "winner", "congratulations", "urgent", "act now", "limited time",
		"click here", "buy now", "earn money", "make money", "cash prize",
		"you won", "lottery", "million", "guaranteed", "no risk", "risk-free",
		"100% free", "order now", "discount", "special offer", "best price",
		"satisfaction guaranteed", "double your", "extra income", "viagra",
	}
	shorteners := []string{"bit.ly", "tinyurl.com", "t.co", "goo.gl", "ow.ly", "short.link", "tiny.cc"}

	// ---- Subject -----------------------------------------------------------
	if email.Subject == "" {
		add("SUBJECT_EMPTY", "subject", 1.0, "Missing subject line", false)
	} else {
		add("SUBJECT_EMPTY", "subject", 0, "Subject is present", true)
	}

	if len(subject) > 5 {
		letters, upper := 0, 0
		for _, c := range subject {
			if unicode.IsLetter(c) {
				letters++
				if unicode.IsUpper(c) {
					upper++
				}
			}
		}
		if letters > 0 && float64(upper)/float64(letters) > 0.5 {
			add("SUBJECT_ALL_CAPS", "subject", 2.0, "Subject is mostly uppercase — looks shouty", false)
		} else {
			add("SUBJECT_ALL_CAPS", "subject", 0, "Subject capitalisation looks normal", true)
		}
	} else {
		add("SUBJECT_ALL_CAPS", "subject", 0, "Subject capitalisation looks normal", true)
	}

	// Overly long subjects get clipped in most clients and look spammy.
	if len([]rune(subject)) > 150 {
		add("SUBJECT_TOO_LONG", "subject", 0.5, fmt.Sprintf("Subject is very long (%d chars) — clients will clip it", len([]rune(subject))), false)
	} else {
		add("SUBJECT_TOO_LONG", "subject", 0, "Subject length looks reasonable", true)
	}

	if excessivePunct.MatchString(subject) {
		add("SUBJECT_EXCESSIVE_PUNCTUATION", "subject", 0.5, "Excessive punctuation in subject (!!! / ???)", false)
	} else {
		add("SUBJECT_EXCESSIVE_PUNCTUATION", "subject", 0, "Subject punctuation looks normal", true)
	}

	subjectMatches := 0
	for _, word := range spamWords {
		if strings.Contains(subjectLower, word) {
			subjectMatches++
		}
	}
	subjectSpamScore := float64(subjectMatches)
	if subjectSpamScore > 3.0 {
		subjectSpamScore = 3.0
	}
	if subjectMatches == 0 {
		add("SUBJECT_SPAM_WORDS", "content", 0, "No trigger words in subject", true)
	} else {
		add("SUBJECT_SPAM_WORDS", "content", subjectSpamScore, fmt.Sprintf("%d trigger word(s) in subject", subjectMatches), false)
	}

	// ---- Body content ------------------------------------------------------
	bodyLower := strings.ToLower(email.Text + " " + email.HTML)
	bodyMatches := 0
	for _, word := range spamWords {
		if strings.Contains(bodyLower, word) {
			bodyMatches++
		}
	}
	bodySpamScore := float64(bodyMatches) * 0.5
	if bodySpamScore > 2.0 {
		bodySpamScore = 2.0
	}
	if bodyMatches == 0 {
		add("BODY_SPAM_WORDS", "content", 0, "No trigger words in body", true)
	} else {
		add("BODY_SPAM_WORDS", "content", bodySpamScore, fmt.Sprintf("%d trigger word(s) in body", bodyMatches), false)
	}

	// ---- Structure ---------------------------------------------------------
	if email.HTML != "" && strings.TrimSpace(email.Text) == "" {
		add("MISSING_TEXT_PART", "structure", 1.0, "HTML email has no plain-text alternative — hurts rendering & deliverability", false)
	} else {
		add("MISSING_TEXT_PART", "structure", 0, "Plain-text alternative present", true)
	}

	// Image-heavy with little real text is a classic spam pattern.
	if email.HTML != "" {
		imgCount := strings.Count(strings.ToLower(email.HTML), "<img")
		visibleText := strings.TrimSpace(tagStripper.ReplaceAllString(email.HTML, " "))
		if email.Text != "" {
			visibleText = strings.TrimSpace(email.Text)
		}
		if imgCount >= 1 && len(visibleText) < 100 {
			add("IMAGE_HEAVY", "structure", 1.0, fmt.Sprintf("Mostly images, little text (%d image(s), %d chars of text)", imgCount, len(visibleText)), false)
		} else {
			add("IMAGE_HEAVY", "structure", 0, "Healthy text-to-image balance", true)
		}
	} else {
		add("IMAGE_HEAVY", "structure", 0, "Healthy text-to-image balance", true)
	}

	// ---- Links -------------------------------------------------------------
	linkCount := strings.Count(strings.ToLower(email.HTML), "<a href")
	if linkCount > 10 {
		add("EXCESSIVE_LINKS", "links", 1.0, fmt.Sprintf("Excessive links: %d found", linkCount), false)
	} else {
		add("EXCESSIVE_LINKS", "links", 0, "Link count looks normal", true)
	}

	combinedLower := strings.ToLower(email.HTML + email.Text)
	hasShortener := false
	for _, domain := range shorteners {
		if strings.Contains(combinedLower, domain) {
			hasShortener = true
			break
		}
	}
	if hasShortener {
		add("URL_SHORTENER", "links", 1.5, "URL shortener detected — filters distrust hidden destinations", false)
	} else {
		add("URL_SHORTENER", "links", 0, "No URL shorteners found", true)
	}

	// ---- Hygiene / best practices -----------------------------------------
	if len(email.Headers["List-Unsubscribe"]) > 0 {
		add("LIST_UNSUBSCRIBE", "hygiene", -0.5, "List-Unsubscribe header present — good for bulk mail", true)
	} else {
		add("LIST_UNSUBSCRIBE", "hygiene", 0.5, "No List-Unsubscribe header — recommended for marketing/bulk mail", false)
	}

	if strings.Contains(email.From, "<") {
		add("MISSING_FROM_NAME", "hygiene", 0, "From address has a display name", true)
	} else {
		add("MISSING_FROM_NAME", "hygiene", 0.3, "From has no display name", false)
	}

	if len(email.Attachments) > 0 {
		add("HAS_ATTACHMENT", "hygiene", 0.3, fmt.Sprintf("Email has %d attachment(s) — bulk mail with attachments is filtered harder", len(email.Attachments)), false)
	} else {
		add("HAS_ATTACHMENT", "hygiene", 0, "No attachments", true)
	}

	// ---- Authentication (informational only) -------------------------------
	// These are added by the sending mail server, not by the message itself,
	// so a local catcher can only report presence — never validate them. They
	// do not affect the content score.
	dkimPass := len(email.Headers["Dkim-Signature"]) > 0 || len(email.Headers["DKIM-Signature"]) > 0
	if dkimPass {
		info("DKIM_PRESENT", "DKIM signature header present (not validated locally)", true)
	} else {
		info("DKIM_PRESENT", "No DKIM signature — normally added by your mail server, not the message", false)
	}

	spfPass := headerContains(email, "Received-Spf", "spf=pass") || headerContains(email, "Authentication-Results", "spf=pass")
	if spfPass {
		info("SPF_PRESENT", "SPF pass header present (not validated locally)", true)
	} else {
		info("SPF_PRESENT", "No SPF result — added by your mail server during sending, not the message", false)
	}

	dmarcPass := headerContains(email, "Authentication-Results", "dmarc=pass")
	if dmarcPass {
		info("DMARC_PRESENT", "DMARC pass header present (not validated locally)", true)
	} else {
		info("DMARC_PRESENT", "No DMARC result — policy lives in DNS, not in the message", false)
	}

	// ---- Total (content checks only) --------------------------------------
	var totalScore float64
	for _, c := range checks {
		if c.Info {
			continue
		}
		totalScore += c.Score
	}
	if totalScore < 0 {
		totalScore = 0
	}
	if totalScore > 10 {
		totalScore = 10
	}

	level := "ham"
	if totalScore >= 6 {
		level = "spam"
	} else if totalScore >= 3 {
		level = "maybe"
	}

	return SpamCheckResult{
		Score:  totalScore,
		Level:  level,
		Checks: checks,
	}
}

func headerContains(email *store.Email, header, needle string) bool {
	for _, v := range email.Headers[header] {
		if strings.Contains(strings.ToLower(v), needle) {
			return true
		}
	}
	return false
}
