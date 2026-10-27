package store

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestMemory_SaveAndGet(t *testing.T) {
	m := NewMemory()

	if err := m.Save("abc123", "https://example.com"); err != nil {
		t.Fatalf("Save: unexpected error: %v", err)
	}

	got, err := m.Get("abc123")
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if got != "https://example.com" {
		t.Errorf("Get = %q, want %q", got, "https://example.com")
	}
}

func TestMemory_GetMissing(t *testing.T) {
	m := NewMemory()

	_, err := m.Get("nope")
	// errors.Is (not ==) keeps the test correct even if Get later wraps
	// the sentinel with extra context via fmt.Errorf("...: %w", err).
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get error = %v, want ErrNotFound", err)
	}
}

func TestMemory_SaveDuplicateDoesNotOverwrite(t *testing.T) {
	m := NewMemory()

	if err := m.Save("abc123", "https://first.example"); err != nil {
		t.Fatalf("first Save: unexpected error: %v", err)
	}
	if err := m.Save("abc123", "https://second.example"); !errors.Is(err, ErrCodeExists) {
		t.Fatalf("second Save error = %v, want ErrCodeExists", err)
	}

	got, _ := m.Get("abc123")
	if got != "https://first.example" {
		t.Errorf("Get = %q, want original URL to be kept", got)
	}
}

// TestMemory_Concurrent is mainly useful under `go test -race`: it hammers
// the store from many goroutines so a missing lock shows up as a data race.
func TestMemory_Concurrent(t *testing.T) {
	m := NewMemory()
	const n = 100

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code := fmt.Sprintf("c%d", i)
			if err := m.Save(code, "https://example.com"); err != nil {
				t.Errorf("Save(%q): %v", code, err)
			}
			if _, err := m.Get(code); err != nil {
				t.Errorf("Get(%q): %v", code, err)
			}
		}()
	}
	wg.Wait()
}
