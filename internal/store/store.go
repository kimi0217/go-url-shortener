// Package store defines how short codes are persisted and looked up.
package store

import (
	"context"
	"errors"
)

// Sentinel errors let callers branch on the failure kind with errors.Is
// instead of comparing error strings, which would break if wording changed.
var (
	// ErrNotFound is returned by Get when no URL is stored under the code.
	ErrNotFound = errors.New("store: code not found")
	// ErrCodeExists is returned by Save when the code is already taken.
	// The caller is expected to generate a new code and retry.
	ErrCodeExists = errors.New("store: code already exists")
)

// Store is kept deliberately small so handlers depend only on behaviour,
// not on a concrete type. Tests and future backends (e.g. a database)
// can then be swapped in without touching the handler code.
//
// Every method takes a context.Context first, by Go convention, so a
// network-backed implementation can stop work when the HTTP request is
// cancelled or times out.
type Store interface {
	// Save stores url under code. It must never overwrite an existing
	// entry; it returns ErrCodeExists instead so collisions are detected.
	Save(ctx context.Context, code, url string) error
	// Get returns the URL stored under code, or ErrNotFound.
	Get(ctx context.Context, code string) (string, error)
}
