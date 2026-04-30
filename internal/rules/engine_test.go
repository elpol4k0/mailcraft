package rules_test

import (
	"context"
	"testing"
	"time"

	"mailcraft/internal/rules"
	"mailcraft/internal/store"
)

func makeEmail(id, from, subject, body string) *store.Email {
	return &store.Email{
		ID:         id,
		From:       from,
		To:         []string{"to@example.com"},
		Subject:    subject,
		Text:       body,
		Tags:       []string{},
		Size:       len(body),
		ReceivedAt: time.Now(),
	}
}

func makeRule(logic store.LogicOp, conds []store.Condition, actions []store.Action) *store.Rule {
	return &store.Rule{
		ID:         "rule1",
		Name:       "Test Rule",
		Enabled:    true,
		Priority:   1,
		Logic:      logic,
		Conditions: conds,
		Actions:    actions,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func TestRuleContains(t *testing.T) {
	e := makeEmail("1", "alice@example.com", "Invoice Payment", "Please pay the invoice")
	rule := makeRule(store.LogicAND, []store.Condition{
		{Field: store.FieldSubject, Operator: store.OpContains, Value: "Invoice"},
	}, []store.Action{
		{Type: store.ActionTag, Value: "billing"},
	})

	eng := rules.NewEngine()
	eng.SetRules([]*store.Rule{rule})

	ms := store.NewMemoryStore(100)
	eng.Apply(context.Background(), e, ms)

	found := false
	for _, t2 := range e.Tags {
		if t2 == "billing" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tag 'billing', got %v", e.Tags)
	}
}

func TestRuleRegex(t *testing.T) {
	e := makeEmail("1", "noreply@github.com", "PR #123 opened", "")
	rule := makeRule(store.LogicAND, []store.Condition{
		{Field: store.FieldFrom, Operator: store.OpRegex, Value: `.*@github\.com`},
	}, []store.Action{
		{Type: store.ActionTag, Value: "github"},
	})

	eng := rules.NewEngine()
	eng.SetRules([]*store.Rule{rule})

	ms := store.NewMemoryStore(100)
	eng.Apply(context.Background(), e, ms)

	found := false
	for _, tag := range e.Tags {
		if tag == "github" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tag 'github', got %v", e.Tags)
	}
}

func TestRuleANDLogic(t *testing.T) {
	e := makeEmail("1", "alice@example.com", "Hello World", "important content")
	rule := makeRule(store.LogicAND, []store.Condition{
		{Field: store.FieldSubject, Operator: store.OpContains, Value: "Hello"},
		{Field: store.FieldFrom, Operator: store.OpContains, Value: "example.com"},
	}, []store.Action{
		{Type: store.ActionTag, Value: "matched"},
	})

	eng := rules.NewEngine()
	eng.SetRules([]*store.Rule{rule})

	ms := store.NewMemoryStore(100)
	eng.Apply(context.Background(), e, ms)

	found := false
	for _, tag := range e.Tags {
		if tag == "matched" {
			found = true
		}
	}
	if !found {
		t.Errorf("AND logic: expected tag 'matched', got %v", e.Tags)
	}

	// Test AND logic fails if one condition doesn't match
	e2 := makeEmail("2", "bob@other.com", "Hello World", "content")
	e2Tags := e2.Tags
	eng.Apply(context.Background(), e2, ms)
	for _, tag := range e2.Tags {
		if tag == "matched" {
			t.Errorf("AND logic: should not match when one condition fails, got tags %v", e2Tags)
		}
	}
}

func TestRuleORLogic(t *testing.T) {
	e := makeEmail("1", "bob@other.com", "Hello World", "body text")
	rule := makeRule(store.LogicOR, []store.Condition{
		{Field: store.FieldSubject, Operator: store.OpContains, Value: "Invoice"},
		{Field: store.FieldSubject, Operator: store.OpContains, Value: "Hello"},
	}, []store.Action{
		{Type: store.ActionTag, Value: "or-matched"},
	})

	eng := rules.NewEngine()
	eng.SetRules([]*store.Rule{rule})

	ms := store.NewMemoryStore(100)
	eng.Apply(context.Background(), e, ms)

	found := false
	for _, tag := range e.Tags {
		if tag == "or-matched" {
			found = true
		}
	}
	if !found {
		t.Errorf("OR logic: expected tag 'or-matched', got %v", e.Tags)
	}
}

func TestRulePriority(t *testing.T) {
	e := makeEmail("1", "test@example.com", "Test", "body")

	rule1 := &store.Rule{
		ID:      "low",
		Name:    "Low Priority",
		Enabled: true,
		Priority: 10,
		Logic:   store.LogicAND,
		Conditions: []store.Condition{
			{Field: store.FieldSubject, Operator: store.OpContains, Value: "Test"},
		},
		Actions: []store.Action{
			{Type: store.ActionColor, Value: "blue"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	rule2 := &store.Rule{
		ID:      "high",
		Name:    "High Priority",
		Enabled: true,
		Priority: 1,
		Logic:   store.LogicAND,
		Conditions: []store.Condition{
			{Field: store.FieldSubject, Operator: store.OpContains, Value: "Test"},
		},
		Actions: []store.Action{
			{Type: store.ActionColor, Value: "red"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	eng := rules.NewEngine()
	eng.SetRules([]*store.Rule{rule1, rule2})

	ms := store.NewMemoryStore(100)
	eng.Apply(context.Background(), e, ms)

	// Both rules match but both set color; last applied color should be "blue" (low priority runs after high)
	if e.Color != "blue" {
		t.Errorf("expected color 'blue' (both rules fire, last wins), got %q", e.Color)
	}
}

func TestRuleActionTag(t *testing.T) {
	e := makeEmail("1", "test@example.com", "Payment Received", "")
	rule := makeRule(store.LogicAND, []store.Condition{
		{Field: store.FieldSubject, Operator: store.OpContains, Value: "Payment"},
	}, []store.Action{
		{Type: store.ActionTag, Value: "finance"},
	})

	eng := rules.NewEngine()
	eng.SetRules([]*store.Rule{rule})

	ms := store.NewMemoryStore(100)
	eng.Apply(context.Background(), e, ms)

	if len(e.Tags) == 0 || e.Tags[0] != "finance" {
		t.Errorf("expected tag 'finance', got %v", e.Tags)
	}
}

func TestRuleActionColor(t *testing.T) {
	e := makeEmail("1", "alert@monitoring.com", "CRITICAL: Server down", "")
	rule := makeRule(store.LogicAND, []store.Condition{
		{Field: store.FieldSubject, Operator: store.OpContains, Value: "CRITICAL"},
	}, []store.Action{
		{Type: store.ActionColor, Value: "red"},
	})

	eng := rules.NewEngine()
	eng.SetRules([]*store.Rule{rule})

	ms := store.NewMemoryStore(100)
	eng.Apply(context.Background(), e, ms)

	if e.Color != "red" {
		t.Errorf("expected color 'red', got %q", e.Color)
	}
}

func TestRuleActionDelete(t *testing.T) {
	e := makeEmail("1", "spam@example.com", "You won a prize!", "")
	rule := makeRule(store.LogicAND, []store.Condition{
		{Field: store.FieldSubject, Operator: store.OpContains, Value: "won a prize"},
	}, []store.Action{
		{Type: store.ActionDelete},
	})

	eng := rules.NewEngine()
	eng.SetRules([]*store.Rule{rule})

	ms := store.NewMemoryStore(100)
	deleted := eng.Apply(context.Background(), e, ms)

	if !deleted {
		t.Error("expected email to be marked for deletion")
	}
}
