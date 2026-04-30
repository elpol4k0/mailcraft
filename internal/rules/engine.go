package rules

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"mailcraft/internal/store"
)

type Engine struct {
	mu    sync.RWMutex
	rules []*store.Rule
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) SetRules(rules []*store.Rule) {
	sorted := make([]*store.Rule, len(rules))
	copy(sorted, rules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})
	e.mu.Lock()
	e.rules = sorted
	e.mu.Unlock()
}

func (e *Engine) Apply(ctx context.Context, email *store.Email, st store.Store) (deleted bool) {
	e.mu.RLock()
	rules := make([]*store.Rule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if !matchRule(r, email) {
			continue
		}
		now := time.Now()
		r.Stats.MatchCount++
		r.Stats.LastMatchAt = &now
		_ = st.UpdateRule(ctx, r)

		for _, action := range r.Actions {
			switch action.Type {
			case store.ActionTag:
				if !containsStr(email.Tags, action.Value) {
					email.Tags = append(email.Tags, action.Value)
				}
			case store.ActionRemoveTag:
				email.Tags = removeStr(email.Tags, action.Value)
			case store.ActionColor:
				email.Color = action.Value
			case store.ActionMarkRead:
				email.Read = true
			case store.ActionStar:
				email.Starred = true
			case store.ActionDelete:
				deleted = true
			case store.ActionWebhook:
				go sendWebhook(action.Value, email)
			case store.ActionFolder:
				if action.Value != "" {
					email.Folder = action.Value
				}
			}
		}

		if deleted {
			return true
		}
	}
	return false
}

func TestRule(rule *store.Rule, emails []*store.Email) []string {
	var ids []string
	for _, e := range emails {
		if matchRule(rule, e) {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

func matchRule(r *store.Rule, e *store.Email) bool {
	if len(r.Conditions) == 0 {
		return false
	}
	for _, c := range r.Conditions {
		result := evalCondition(c, e)
		if r.Logic == store.LogicOR && result {
			return true
		}
		if r.Logic == store.LogicAND && !result {
			return false
		}
	}
	return r.Logic == store.LogicAND
}

func evalCondition(c store.Condition, e *store.Email) bool {
	var fieldVal string
	switch c.Field {
	case store.FieldFrom:
		fieldVal = e.From
	case store.FieldTo:
		fieldVal = strings.Join(e.To, " ")
	case store.FieldCC:
		fieldVal = strings.Join(e.CC, " ")
	case store.FieldSubject:
		fieldVal = e.Subject
	case store.FieldBody:
		fieldVal = e.Text + " " + e.HTML
	case store.FieldHeader:
		if c.HeaderKey != "" {
			vals := e.Headers[c.HeaderKey]
			fieldVal = strings.Join(vals, " ")
		}
	case store.FieldTag:
		fieldVal = strings.Join(e.Tags, " ")
	case store.FieldSize:
		return evalNumericOp(c.Operator, e.Size, c.Value)
	case store.FieldHasAttachment:
		has := len(e.Attachments) > 0
		if c.Operator == store.OpExists {
			return strconv.FormatBool(has) == c.Value
		}
		return false
	}

	return evalStringOp(c.Operator, fieldVal, c.Value)
}

func evalStringOp(op store.ConditionOperator, fieldVal, condVal string) bool {
	switch op {
	case store.OpContains:
		return strings.Contains(strings.ToLower(fieldVal), strings.ToLower(condVal))
	case store.OpNotContains:
		return !strings.Contains(strings.ToLower(fieldVal), strings.ToLower(condVal))
	case store.OpEquals:
		return strings.EqualFold(fieldVal, condVal)
	case store.OpNotEquals:
		return !strings.EqualFold(fieldVal, condVal)
	case store.OpStartsWith:
		return strings.HasPrefix(strings.ToLower(fieldVal), strings.ToLower(condVal))
	case store.OpEndsWith:
		return strings.HasSuffix(strings.ToLower(fieldVal), strings.ToLower(condVal))
	case store.OpRegex:
		re, err := regexp.Compile(condVal)
		if err != nil {
			return false
		}
		return re.MatchString(fieldVal)
	case store.OpExists:
		return fieldVal != ""
	}
	return false
}

func evalNumericOp(op store.ConditionOperator, size int, condVal string) bool {
	n, err := strconv.Atoi(condVal)
	if err != nil {
		return false
	}
	switch op {
	case store.OpGT:
		return size > n
	case store.OpLT:
		return size < n
	case store.OpEquals:
		return size == n
	}
	return false
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func removeStr(ss []string, s string) []string {
	out := ss[:0]
	for _, v := range ss {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

func sendWebhook(url string, email *store.Email) {
	body := fmt.Sprintf(`{"id":%q,"subject":%q,"from":%q}`, email.ID, email.Subject, email.From)
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(body))
	if err != nil {
		return
	}
	resp.Body.Close()
}
