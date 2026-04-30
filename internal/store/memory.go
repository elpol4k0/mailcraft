package store

import (
	"context"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu        sync.RWMutex
	emails    []*Email
	index     map[string]*Email
	maxEmails int

	rulesMu  sync.RWMutex
	rules    []*Rule
	rulesIdx map[string]*Rule

	subsMu sync.RWMutex
	subs   map[chan Event]struct{}
}

func NewMemoryStore(maxEmails int) *MemoryStore {
	if maxEmails <= 0 {
		maxEmails = 5000
	}
	return &MemoryStore{
		emails:    make([]*Email, 0),
		index:     make(map[string]*Email),
		maxEmails: maxEmails,
		rules:     make([]*Rule, 0),
		rulesIdx:  make(map[string]*Rule),
		subs:      make(map[chan Event]struct{}),
	}
}

func (s *MemoryStore) Add(_ context.Context, email *Email) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.emails) >= s.maxEmails {
		oldest := s.emails[0]
		s.emails = s.emails[1:]
		delete(s.index, oldest.ID)
	}

	clone := emailClone(email)
	s.emails = append(s.emails, clone)
	s.index[clone.ID] = clone
	return nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (*Email, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.index[id]
	if !ok {
		return nil, ErrNotFound
	}
	clone := emailClone(e)
	return clone, nil
}

func (s *MemoryStore) List(_ context.Context, filter SearchFilter) ([]*Email, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matched := make([]*Email, 0, len(s.emails))
	for _, e := range s.emails {
		if matchesFilter(e, filter) {
			clone := emailClone(e)
			matched = append(matched, clone)
		}
	}

	total := len(matched)

	sort := filter.Sort
	if sort == "" || sort == "date" || sort == "-date" {
		for i, j := 0, len(matched)-1; i < j; i, j = i+1, j-1 {
			matched[i], matched[j] = matched[j], matched[i]
		}
	}

	page := filter.Page
	limit := filter.Limit
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}
	start := (page - 1) * limit
	if start >= len(matched) {
		return []*Email{}, total, nil
	}
	end := start + limit
	if end > len(matched) {
		end = len(matched)
	}
	return matched[start:end], total, nil
}

func (s *MemoryStore) Update(_ context.Context, email *Email) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.index[email.ID]
	if !ok {
		return ErrNotFound
	}

	clone := emailClone(email)
	s.index[email.ID] = clone
	for i, e := range s.emails {
		if e.ID == email.ID {
			if len(clone.RawMessage) == 0 {
				clone.RawMessage = existing.RawMessage
			}
			for ai := range clone.Attachments {
				if len(clone.Attachments[ai].Data) == 0 && ai < len(existing.Attachments) {
					clone.Attachments[ai].Data = existing.Attachments[ai].Data
				}
			}
			s.emails[i] = clone
			s.index[email.ID] = clone
			break
		}
	}
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.index[id]; !ok {
		return ErrNotFound
	}
	delete(s.index, id)
	for i, e := range s.emails {
		if e.ID == id {
			s.emails = append(s.emails[:i], s.emails[i+1:]...)
			break
		}
	}
	return nil
}

func (s *MemoryStore) DeleteAll(_ context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(ids) == 0 {
		s.emails = s.emails[:0]
		s.index = make(map[string]*Email)
		return nil
	}

	del := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		del[id] = struct{}{}
	}
	remaining := s.emails[:0]
	for _, e := range s.emails {
		if _, ok := del[e.ID]; ok {
			delete(s.index, e.ID)
		} else {
			remaining = append(remaining, e)
		}
	}
	s.emails = remaining
	return nil
}

func (s *MemoryStore) Stats(_ context.Context) (Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var st Stats
	st.Total = len(s.emails)
	for _, e := range s.emails {
		if !e.Read {
			st.Unread++
		}
		if e.Starred {
			st.Starred++
		}
		st.SizeBytes += int64(e.Size)
	}
	return st, nil
}

func (s *MemoryStore) Tags(_ context.Context) (map[string]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := make(map[string]int)
	for _, e := range s.emails {
		for _, t := range e.Tags {
			counts[t]++
		}
	}
	return counts, nil
}

func (s *MemoryStore) Folders(_ context.Context) (map[string]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := make(map[string]int)
	for _, e := range s.emails {
		if e.Folder != "" {
			counts[e.Folder]++
		}
	}
	return counts, nil
}

func (s *MemoryStore) RenameTag(_ context.Context, oldName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, e := range s.emails {
		for i, t := range e.Tags {
			if t == oldName {
				e.Tags[i] = newName
			}
		}
	}
	return nil
}

func (s *MemoryStore) DeleteTag(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, e := range s.emails {
		newTags := e.Tags[:0]
		for _, t := range e.Tags {
			if t != name {
				newTags = append(newTags, t)
			}
		}
		e.Tags = newTags
	}
	return nil
}

func (s *MemoryStore) ListRules(_ context.Context) ([]*Rule, error) {
	s.rulesMu.RLock()
	defer s.rulesMu.RUnlock()
	out := make([]*Rule, len(s.rules))
	copy(out, s.rules)
	return out, nil
}

func (s *MemoryStore) AddRule(_ context.Context, rule *Rule) error {
	s.rulesMu.Lock()
	defer s.rulesMu.Unlock()
	s.rules = append(s.rules, rule)
	s.rulesIdx[rule.ID] = rule
	return nil
}

func (s *MemoryStore) GetRule(_ context.Context, id string) (*Rule, error) {
	s.rulesMu.RLock()
	defer s.rulesMu.RUnlock()
	r, ok := s.rulesIdx[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

func (s *MemoryStore) UpdateRule(_ context.Context, rule *Rule) error {
	s.rulesMu.Lock()
	defer s.rulesMu.Unlock()
	if _, ok := s.rulesIdx[rule.ID]; !ok {
		return ErrNotFound
	}
	s.rulesIdx[rule.ID] = rule
	for i, r := range s.rules {
		if r.ID == rule.ID {
			s.rules[i] = rule
			break
		}
	}
	return nil
}

func (s *MemoryStore) DeleteRule(_ context.Context, id string) error {
	s.rulesMu.Lock()
	defer s.rulesMu.Unlock()
	if _, ok := s.rulesIdx[id]; !ok {
		return ErrNotFound
	}
	delete(s.rulesIdx, id)
	for i, r := range s.rules {
		if r.ID == id {
			s.rules = append(s.rules[:i], s.rules[i+1:]...)
			break
		}
	}
	return nil
}

func (s *MemoryStore) Subscribe(ctx context.Context) (<-chan Event, func()) {
	ch := make(chan Event, 32)
	s.subsMu.Lock()
	s.subs[ch] = struct{}{}
	s.subsMu.Unlock()

	var once sync.Once
	doCancel := func() {
		once.Do(func() {
			s.subsMu.Lock()
			delete(s.subs, ch)
			s.subsMu.Unlock()
			close(ch)
		})
	}

	go func() {
		<-ctx.Done()
		doCancel()
	}()

	return ch, doCancel
}

func (s *MemoryStore) Publish(event Event) {
	s.subsMu.RLock()
	defer s.subsMu.RUnlock()

	for ch := range s.subs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *MemoryStore) Close() error {
	return nil
}

func matchesFilter(e *Email, f SearchFilter) bool {
	if f.Query != "" {
		q := strings.ToLower(f.Query)
		if !strings.Contains(strings.ToLower(e.From), q) &&
			!strings.Contains(strings.ToLower(e.Subject), q) &&
			!strings.Contains(strings.ToLower(e.Text), q) &&
			!strings.Contains(strings.ToLower(e.HTML), q) &&
			!containsAnyLower(e.To, q) {
			return false
		}
	}
	if f.Tag != "" {
		found := false
		for _, t := range e.Tags {
			if t == f.Tag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if f.Read != nil && e.Read != *f.Read {
		return false
	}
	if f.Starred != nil && e.Starred != *f.Starred {
		return false
	}
	if f.Folder != "" && e.Folder != f.Folder {
		return false
	}
	if f.From != "" && !strings.Contains(strings.ToLower(e.From), strings.ToLower(f.From)) {
		return false
	}
	if f.To != "" {
		found := false
		for _, to := range e.To {
			if strings.Contains(strings.ToLower(to), strings.ToLower(f.To)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func containsAnyLower(ss []string, q string) bool {
	for _, s := range ss {
		if strings.Contains(strings.ToLower(s), q) {
			return true
		}
	}
	return false
}

func emailClone(e *Email) *Email {
	clone := *e
	clone.To = copyStrings(e.To)
	clone.CC = copyStrings(e.CC)
	clone.BCC = copyStrings(e.BCC)
	clone.Tags = copyStrings(e.Tags)
	clone.Headers = copyHeaders(e.Headers)
	clone.Attachments = copyAttachments(e.Attachments)
	if len(e.RawMessage) > 0 {
		clone.RawMessage = make([]byte, len(e.RawMessage))
		copy(clone.RawMessage, e.RawMessage)
	}
	return &clone
}

func copyStrings(ss []string) []string {
	if ss == nil {
		return nil
	}
	out := make([]string, len(ss))
	copy(out, ss)
	return out
}

func copyHeaders(h map[string][]string) map[string][]string {
	if h == nil {
		return nil
	}
	out := make(map[string][]string, len(h))
	for k, v := range h {
		out[k] = copyStrings(v)
	}
	return out
}

func copyAttachments(atts []Attachment) []Attachment {
	if atts == nil {
		return nil
	}
	out := make([]Attachment, len(atts))
	for i, a := range atts {
		out[i] = a
		if len(a.Data) > 0 {
			out[i].Data = make([]byte, len(a.Data))
			copy(out[i].Data, a.Data)
		}
	}
	return out
}

var ErrNotFound = storeError("not found")

type storeError string

func (e storeError) Error() string { return string(e) }

func GetRaw(e *Email) []byte { return e.RawMessage }

func TimePtr(t time.Time) *time.Time { return &t }
