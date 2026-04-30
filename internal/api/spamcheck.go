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

type SpamCheckResult struct {
	Score  float64         `json:"score"`
	Level  string          `json:"level"`
	Checks []SpamCheckItem `json:"checks"`
}

type SpamCheckItem struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
	Pass        bool    `json:"pass"`
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

func runSpamCheck(email *store.Email) SpamCheckResult {
	var checks []SpamCheckItem

	subject := email.Subject
	subjectLower := strings.ToLower(subject)

	spamWords := []string{
		"free", "winner", "congratulations", "urgent", "act now", "limited time",
		"click here", "buy now", "earn money", "make money", "cash prize",
		"you won", "lottery", "million", "guaranteed", "no risk",
	}

	shorteners := []string{"bit.ly", "tinyurl.com", "t.co", "goo.gl", "ow.ly", "short.link", "tiny.cc"}

	dkimPass := len(email.Headers["Dkim-Signature"]) > 0 || len(email.Headers["DKIM-Signature"]) > 0
	if dkimPass {
		checks = append(checks, SpamCheckItem{Name: "DKIM_PRESENT", Score: -0.1, Description: "DKIM signature found", Pass: true})
	} else {
		checks = append(checks, SpamCheckItem{Name: "DKIM_PRESENT", Score: 1.5, Description: "No DKIM signature found", Pass: false})
	}

	spfPass := false
	for _, v := range email.Headers["Received-Spf"] {
		if strings.Contains(strings.ToLower(v), "spf=pass") {
			spfPass = true
			break
		}
	}
	if !spfPass {
		for _, v := range email.Headers["Authentication-Results"] {
			if strings.Contains(strings.ToLower(v), "spf=pass") {
				spfPass = true
				break
			}
		}
	}
	if spfPass {
		checks = append(checks, SpamCheckItem{Name: "SPF_PRESENT", Score: -0.1, Description: "SPF pass found", Pass: true})
	} else {
		checks = append(checks, SpamCheckItem{Name: "SPF_PRESENT", Score: 1.0, Description: "No SPF pass result", Pass: false})
	}

	dmarcPass := false
	for _, v := range email.Headers["Authentication-Results"] {
		if strings.Contains(strings.ToLower(v), "dmarc=pass") {
			dmarcPass = true
			break
		}
	}
	if dmarcPass {
		checks = append(checks, SpamCheckItem{Name: "DMARC_PRESENT", Score: -0.1, Description: "DMARC pass found", Pass: true})
	} else {
		checks = append(checks, SpamCheckItem{Name: "DMARC_PRESENT", Score: 0.5, Description: "No DMARC result", Pass: false})
	}

	if len(subject) > 5 {
		letters := 0
		upper := 0
		for _, c := range subject {
			if unicode.IsLetter(c) {
				letters++
				if unicode.IsUpper(c) {
					upper++
				}
			}
		}
		if letters > 0 && float64(upper)/float64(letters) > 0.5 {
			checks = append(checks, SpamCheckItem{Name: "SUBJECT_ALL_CAPS", Score: 2.0, Description: "Subject is mostly uppercase", Pass: false})
		} else {
			checks = append(checks, SpamCheckItem{Name: "SUBJECT_ALL_CAPS", Score: 0, Description: "Subject case looks normal", Pass: true})
		}
	} else {
		checks = append(checks, SpamCheckItem{Name: "SUBJECT_ALL_CAPS", Score: 0, Description: "Subject case looks normal", Pass: true})
	}

	subjectMatches := 0
	for _, word := range spamWords {
		if strings.Contains(subjectLower, word) {
			subjectMatches++
		}
	}
	subjectSpamScore := float64(subjectMatches) * 1.0
	if subjectSpamScore > 3.0 {
		subjectSpamScore = 3.0
	}
	if subjectMatches == 0 {
		checks = append(checks, SpamCheckItem{Name: "SUBJECT_SPAM_WORDS", Score: 0, Description: "No spam keywords in subject", Pass: true})
	} else {
		checks = append(checks, SpamCheckItem{Name: "SUBJECT_SPAM_WORDS", Score: subjectSpamScore, Description: "Spam keywords found in subject", Pass: false})
	}

	bodyLower := strings.ToLower(email.Text)
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
		checks = append(checks, SpamCheckItem{Name: "BODY_SPAM_WORDS", Score: 0, Description: "No spam keywords in body", Pass: true})
	} else {
		checks = append(checks, SpamCheckItem{Name: "BODY_SPAM_WORDS", Score: bodySpamScore, Description: "Spam keywords in message body", Pass: false})
	}

	if email.HTML != "" && email.Text == "" {
		checks = append(checks, SpamCheckItem{Name: "MISSING_TEXT_PART", Score: 0.5, Description: "No plain text alternative", Pass: false})
	} else {
		checks = append(checks, SpamCheckItem{Name: "MISSING_TEXT_PART", Score: 0, Description: "Plain text part present", Pass: true})
	}

	linkCount := strings.Count(email.HTML, "<a href")
	if linkCount > 10 {
		checks = append(checks, SpamCheckItem{Name: "EXCESSIVE_LINKS", Score: 1.0, Description: fmt.Sprintf("Excessive links: %d found", linkCount), Pass: false})
	} else {
		checks = append(checks, SpamCheckItem{Name: "EXCESSIVE_LINKS", Score: 0, Description: "Link count looks normal", Pass: true})
	}

	hasUnsubscribe := len(email.Headers["List-Unsubscribe"]) > 0
	if hasUnsubscribe {
		checks = append(checks, SpamCheckItem{Name: "LIST_UNSUBSCRIBE", Score: -0.5, Description: "List-Unsubscribe header present", Pass: true})
	} else {
		checks = append(checks, SpamCheckItem{Name: "LIST_UNSUBSCRIBE", Score: 0.3, Description: "No List-Unsubscribe header", Pass: false})
	}

	if strings.Contains(email.From, "<") {
		checks = append(checks, SpamCheckItem{Name: "MISSING_FROM_NAME", Score: 0, Description: "From address has display name", Pass: true})
	} else {
		checks = append(checks, SpamCheckItem{Name: "MISSING_FROM_NAME", Score: 0.3, Description: "From has no display name", Pass: false})
	}

	if email.Subject == "" {
		checks = append(checks, SpamCheckItem{Name: "SUBJECT_EMPTY", Score: 1.0, Description: "Missing subject", Pass: false})
	} else {
		checks = append(checks, SpamCheckItem{Name: "SUBJECT_EMPTY", Score: 0, Description: "Subject is present", Pass: true})
	}

	combined := email.HTML + email.Text
	combinedLower := strings.ToLower(combined)
	hasShortener := false
	for _, domain := range shorteners {
		if strings.Contains(combinedLower, domain) {
			hasShortener = true
			break
		}
	}
	if hasShortener {
		checks = append(checks, SpamCheckItem{Name: "URL_SHORTENER", Score: 1.5, Description: "URL shortener detected (may be tracking link)", Pass: false})
	} else {
		checks = append(checks, SpamCheckItem{Name: "URL_SHORTENER", Score: 0, Description: "No URL shorteners found", Pass: true})
	}

	excessivePunct := regexp.MustCompile(`[!?]{3,}`)
	if excessivePunct.MatchString(subject) {
		checks = append(checks, SpamCheckItem{Name: "EXCESSIVE_PUNCTUATION", Score: 0.5, Description: "Excessive punctuation in subject", Pass: false})
	} else {
		checks = append(checks, SpamCheckItem{Name: "EXCESSIVE_PUNCTUATION", Score: 0, Description: "Subject punctuation looks normal", Pass: true})
	}

	if len(email.Attachments) > 0 {
		checks = append(checks, SpamCheckItem{Name: "HAS_ATTACHMENT", Score: 0.3, Description: fmt.Sprintf("Email has %d attachment(s)", len(email.Attachments)), Pass: false})
	} else {
		checks = append(checks, SpamCheckItem{Name: "HAS_ATTACHMENT", Score: 0, Description: "No attachments", Pass: true})
	}

	var totalScore float64
	for _, c := range checks {
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
