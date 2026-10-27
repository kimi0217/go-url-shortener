package store

import (
	"context"
	"sync"
)

// Compile-time check that *Memory satisfies Store. If a method signature
// drifts, the build fails here rather than at some distant call site.
var _ Store = (*Memory)(nil)

// Memory is an in-memory Store. Data is lost when the process exits,
// which is acceptable for this project's scope.
type Memory struct {
	// RWMutex rather than Mutex: redirects (reads) vastly outnumber
	// shortens (writes), and RLock lets many readers proceed in parallel.
	mu   sync.RWMutex
	urls map[string]string
}

// NewMemory returns a ready-to-use Memory. A constructor is needed because
// the zero value's nil map would panic on the first write.
func NewMemory() *Memory {
	return &Memory{urls: make(map[string]string)}
}

func (m *Memory) Save(ctx context.Context, code, url string) error {
	// Map access never blocks, so there is nothing to interrupt midway.
	// Checking up front still honours the contract: a caller that has
	// already given up should not cause a write.
	if err := ctx.Err(); err != nil {
		return err
	}

	// The existence check and the write must happen under the same lock.
	// Checking under RLock and then writing under Lock would let two
	// goroutines both see "free" and silently overwrite each other.
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.urls[code]; exists {
		return ErrCodeExists
	}
	m.urls[code] = url
	return nil
}

func (m *Memory) Get(ctx context.Context, code string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	url, ok := m.urls[code]
	if !ok {
		return "", ErrNotFound
	}
	return url, nil
}
