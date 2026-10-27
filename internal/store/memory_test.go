package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestMemory_SaveAndGet(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()

	if err := m.Save(ctx, "abc123", "https://example.com"); err != nil {
		t.Fatalf("Save: unexpected error: %v", err)
	}

	got, err := m.Get(ctx, "abc123")
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if got != "https://example.com" {
		t.Errorf("Get = %q, want %q", got, "https://example.com")
	}
}

func TestMemory_GetMissing(t *testing.T) {
	m := NewMemory()

	_, err := m.Get(context.Background(), "nope")
	// errors.Is (not ==) keeps the test correct even if Get later wraps
	// the sentinel with extra context via fmt.Errorf("...: %w", err).
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get error = %v, want ErrNotFound", err)
	}
}

func TestMemory_SaveDuplicateDoesNotOverwrite(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()

	if err := m.Save(ctx, "abc123", "https://first.example"); err != nil {
		t.Fatalf("first Save: unexpected error: %v", err)
	}
	if err := m.Save(ctx, "abc123", "https://second.example"); !errors.Is(err, ErrCodeExists) {
		t.Fatalf("second Save error = %v, want ErrCodeExists", err)
	}

	got, _ := m.Get(ctx, "abc123")
	if got != "https://first.example" {
		t.Errorf("Get = %q, want original URL to be kept", got)
	}
}

func TestMemory_CancelledContext(t *testing.T) {
	m := NewMemory()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately to simulate a client that already left

	if err := m.Save(ctx, "abc123", "https://example.com"); !errors.Is(err, context.Canceled) {
		t.Errorf("Save error = %v, want context.Canceled", err)
	}
	if _, err := m.Get(ctx, "abc123"); !errors.Is(err, context.Canceled) {
		t.Errorf("Get error = %v, want context.Canceled", err)
	}

	// The cancelled Save must not have written anything.
	if _, err := m.Get(context.Background(), "abc123"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after cancelled Save error = %v, want ErrNotFound", err)
	}
}

// TestMemory_Concurrent is mainly useful under `go test -race`: it hammers
// the store from many goroutines so a missing lock shows up as a data race.
func TestMemory_Concurrent(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	const n = 100

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code := fmt.Sprintf("c%d", i)
			if err := m.Save(ctx, code, "https://example.com"); err != nil {
				t.Errorf("Save(%q): %v", code, err)
			}
			if _, err := m.Get(ctx, code); err != nil {
				t.Errorf("Get(%q): %v", code, err)
			}
		}()
	}
	wg.Wait()
}
