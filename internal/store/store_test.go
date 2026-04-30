package store_test

import (
	"context"
	"testing"
	"time"

	"mailcraft/internal/store"
)

func makeEmail(id, from, subject string) *store.Email {
	return &store.Email{
		ID:         id,
		From:       from,
		To:         []string{"to@example.com"},
		Subject:    subject,
		Text:       "hello body",
		Tags:       []string{},
		ReceivedAt: time.Now(),
		Size:       100,
	}
}

func TestStoreAddAndGet(t *testing.T) {
	s := store.NewMemoryStore(100)
	ctx := context.Background()

	e := makeEmail("id1", "alice@example.com", "Hello World")
	if err := s.Add(ctx, e); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := s.Get(ctx, "id1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.From != "alice@example.com" {
		t.Errorf("From = %q, want %q", got.From, "alice@example.com")
	}
	if got.Subject != "Hello World" {
		t.Errorf("Subject = %q, want %q", got.Subject, "Hello World")
	}
}

func TestStoreSearch(t *testing.T) {
	s := store.NewMemoryStore(100)
	ctx := context.Background()

	emails := []*store.Email{
		makeEmail("1", "alice@example.com", "Invoice for services"),
		makeEmail("2", "bob@example.com", "Meeting tomorrow"),
		makeEmail("3", "charlie@example.com", "Invoice follow-up"),
	}
	emails[0].Tags = []string{"billing"}
	emails[1].Read = true

	for _, e := range emails {
		if err := s.Add(ctx, e); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	// Search by query
	results, total, err := s.List(ctx, store.SearchFilter{Query: "Invoice"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	_ = results

	// Filter by tag
	results, total, err = s.List(ctx, store.SearchFilter{Tag: "billing"})
	if err != nil {
		t.Fatalf("List by tag: %v", err)
	}
	if total != 1 {
		t.Errorf("tag total = %d, want 1", total)
	}

	// Filter by read
	readTrue := true
	results, total, err = s.List(ctx, store.SearchFilter{Read: &readTrue})
	if err != nil {
		t.Fatalf("List by read: %v", err)
	}
	if total != 1 {
		t.Errorf("read total = %d, want 1", total)
	}
	_ = results
}

func TestStoreMaxEmails(t *testing.T) {
	s := store.NewMemoryStore(3)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		id := string(rune('a' + i))
		e := makeEmail(id, "sender@example.com", "Email "+id)
		if err := s.Add(ctx, e); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	_, total, err := s.List(ctx, store.SearchFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}

	// Oldest should be gone
	_, err = s.Get(ctx, "a")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for evicted email, got %v", err)
	}

	// Newest should exist
	_, err = s.Get(ctx, "e")
	if err != nil {
		t.Errorf("Get newest: %v", err)
	}
}

func TestStoreSubscribe(t *testing.T) {
	s := store.NewMemoryStore(100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, unsub := s.Subscribe(ctx)
	defer unsub()

	e := makeEmail("sub1", "test@example.com", "Subscription Test")
	s.Publish(store.Event{Type: "email.new", Payload: e})

	select {
	case evt := <-ch:
		if evt.Type != "email.new" {
			t.Errorf("event type = %q, want %q", evt.Type, "email.new")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}
