package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kimi0217/go-url-shortener/internal/shortener"
	"github.com/kimi0217/go-url-shortener/internal/store"
)

const testBaseURL = "http://localhost:8080"

func newTestHandler(s store.Store) *Handler {
	// Discard logs so test output only shows failures.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(s, testBaseURL, logger)
}

// failingStore always errors, to exercise the 500 paths. Being able to drop
// in a fake like this is the payoff of handlers depending on an interface.
type failingStore struct{}

var errStoreDown = errors.New("store down")

func (failingStore) Save(context.Context, string, string) error  { return errStoreDown }
func (failingStore) Get(context.Context, string) (string, error) { return "", errStoreDown }

// assertError checks a JSON error response. t.Helper makes failures point
// at the calling test line instead of inside this function.
func assertError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantMsg string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d", rec.Code, wantStatus)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Error != wantMsg {
		t.Errorf("error = %q, want %q", body.Error, wantMsg)
	}
}

func TestShorten(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantError  string // empty means a successful response is expected
	}{
		{"valid https", `{"url":"https://example.com/path?q=1"}`, http.StatusCreated, ""},
		{"valid http", `{"url":"http://example.com"}`, http.StatusCreated, ""},
		{"uppercase scheme", `{"url":"HTTPS://example.com"}`, http.StatusCreated, ""},
		{"malformed JSON", `{"url":`, http.StatusBadRequest, msgInvalidJSON},
		{"empty body", ``, http.StatusBadRequest, msgInvalidJSON},
		{"url not a string", `{"url":123}`, http.StatusBadRequest, msgInvalidJSON},
		{"missing url", `{}`, http.StatusBadRequest, msgInvalidURL},
		{"no scheme", `{"url":"example.com"}`, http.StatusBadRequest, msgInvalidURL},
		{"ftp scheme", `{"url":"ftp://example.com"}`, http.StatusBadRequest, msgInvalidURL},
		{"javascript scheme", `{"url":"javascript:alert(1)"}`, http.StatusBadRequest, msgInvalidURL},
		{"missing host", `{"url":"http://"}`, http.StatusBadRequest, msgInvalidURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewMemory()
			h := newTestHandler(s)

			req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)

			if tt.wantError != "" {
				assertError(t, rec, tt.wantStatus, tt.wantError)
				return
			}

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			var resp shortenResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(resp.Code) != shortener.Length {
				t.Errorf("code %q has length %d, want %d", resp.Code, len(resp.Code), shortener.Length)
			}
			if want := testBaseURL + "/" + resp.Code; resp.ShortURL != want {
				t.Errorf("short_url = %q, want %q", resp.ShortURL, want)
			}

			// The response alone could be faked; confirm the store was written.
			var sent shortenRequest
			_ = json.Unmarshal([]byte(tt.body), &sent)
			got, err := s.Get(context.Background(), resp.Code)
			if err != nil {
				t.Fatalf("store.Get(%q): %v", resp.Code, err)
			}
			if got != sent.URL {
				t.Errorf("stored URL = %q, want %q", got, sent.URL)
			}
		})
	}
}

// TestShorten_CodeAllocation covers the retry loop, which needs a
// predictable generator and so cannot use the real random one.
func TestShorten_CodeAllocation(t *testing.T) {
	const taken = "TAKEN0"

	tests := []struct {
		name       string
		store      store.Store
		codes      []string // codes the fake generator returns, in order
		genErr     error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "retries after collision",
			codes:      []string{taken, "fresh0"},
			wantStatus: http.StatusCreated,
			wantCode:   "fresh0",
		},
		{
			name:       "gives up after max attempts",
			codes:      []string{taken, taken, taken, taken, taken},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "generator error",
			genErr:     errors.New("entropy exhausted"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "store error",
			store:      failingStore{},
			codes:      []string{"fresh0"},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.store
			if s == nil {
				m := store.NewMemory()
				if err := m.Save(context.Background(), taken, "https://already.example"); err != nil {
					t.Fatalf("seed store: %v", err)
				}
				s = m
			}
			h := newTestHandler(s)

			calls := 0
			h.generate = func() (string, error) {
				if tt.genErr != nil {
					return "", tt.genErr
				}
				if calls >= len(tt.codes) {
					t.Fatalf("generate called %d times, only %d codes provided", calls+1, len(tt.codes))
				}
				code := tt.codes[calls]
				calls++
				return code, nil
			}

			req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"https://example.com"}`))
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)

			if tt.wantStatus != http.StatusCreated {
				assertError(t, rec, tt.wantStatus, msgInternal)
				return
			}

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			var resp shortenResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", resp.Code, tt.wantCode)
			}
		})
	}
}

func TestRedirect(t *testing.T) {
	seeded := store.NewMemory()
	if err := seeded.Save(context.Background(), "abc123", "https://example.com/target"); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	tests := []struct {
		name         string
		store        store.Store
		path         string
		wantStatus   int
		wantLocation string
		wantError    string
	}{
		{"known code", seeded, "/abc123", http.StatusFound, "https://example.com/target", ""},
		{"unknown code", seeded, "/zzzzzz", http.StatusNotFound, "", msgNotFound},
		{"store error", failingStore{}, "/abc123", http.StatusInternalServerError, "", msgInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(tt.store)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)

			if tt.wantError != "" {
				assertError(t, rec, tt.wantStatus, tt.wantError)
				return
			}

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if loc := rec.Header().Get("Location"); loc != tt.wantLocation {
				t.Errorf("Location = %q, want %q", loc, tt.wantLocation)
			}
		})
	}
}
