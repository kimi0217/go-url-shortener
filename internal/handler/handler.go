// Package handler exposes the URL shortener over HTTP.
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/kimi0217/go-url-shortener/internal/shortener"
	"github.com/kimi0217/go-url-shortener/internal/store"
)

const (
	// maxBodyBytes caps the request body so a client cannot exhaust memory
	// by streaming an endless payload into the JSON decoder.
	maxBodyBytes = 1 << 20 // 1 MiB

	// maxGenerateAttempts bounds the collision retry loop. With 62^6 codes
	// a single collision is already rare; several in a row means something
	// is wrong (e.g. a broken generator), and looping forever would hang
	// the request.
	maxGenerateAttempts = 5
)

// Error messages are constants so tests can assert on them without
// duplicating string literals that could drift apart.
const (
	msgInvalidJSON = "invalid JSON body"
	msgInvalidURL  = "url must be a valid http or https URL"
	msgNotFound    = "short code not found"
	msgInternal    = "internal server error"
)

// Handler holds the dependencies shared by all endpoints. Passing them in
// through New (instead of using package-level globals) keeps each test
// isolated: every test builds its own Handler with its own store.
type Handler struct {
	store   store.Store
	baseURL string
	logger  *slog.Logger
	// generate is a field rather than a direct call to shortener.Generate
	// so tests in this package can substitute a fake and force collisions.
	generate func() (string, error)
}

// New returns a Handler. baseURL is the public origin used to build
// short_url in responses, e.g. "http://localhost:8080".
func New(s store.Store, baseURL string, logger *slog.Logger) *Handler {
	return &Handler{
		store:    s,
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		logger:   logger,
		generate: shortener.Generate,
	}
}

// Routes returns an http.Handler with every endpoint registered. Returning
// the interface rather than *http.ServeMux hides how routing is done.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	// Go 1.22+ patterns: the method prefix restricts the route, and
	// {code} is a wildcard read back with r.PathValue("code").
	mux.HandleFunc("POST /shorten", h.shorten)
	mux.HandleFunc("GET /{code}", h.redirect)
	return mux
}

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) shorten(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var req shortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, msgInvalidJSON)
		return
	}
	if !isValidURL(req.URL) {
		writeError(w, http.StatusBadRequest, msgInvalidURL)
		return
	}

	ctx := r.Context()
	for range maxGenerateAttempts {
		code, err := h.generate()
		if err != nil {
			h.logger.Error("generate short code", "err", err)
			writeError(w, http.StatusInternalServerError, msgInternal)
			return
		}

		err = h.store.Save(ctx, code, req.URL)
		if errors.Is(err, store.ErrCodeExists) {
			h.logger.Warn("short code collision, retrying", "code", code)
			continue
		}
		if err != nil {
			h.logger.Error("save short code", "code", code, "err", err)
			writeError(w, http.StatusInternalServerError, msgInternal)
			return
		}

		h.logger.Info("short code created", "code", code, "url", req.URL)
		// 201 rather than 200: the request created a new resource.
		writeJSON(w, http.StatusCreated, shortenResponse{
			Code:     code,
			ShortURL: h.baseURL + "/" + code,
		})
		return
	}

	h.logger.Error("no free short code", "attempts", maxGenerateAttempts)
	writeError(w, http.StatusInternalServerError, msgInternal)
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	// Named target, not url, so it does not shadow the net/url package.
	target, err := h.store.Get(r.Context(), code)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, msgNotFound)
		return
	}
	if err != nil {
		h.logger.Error("get short code", "code", code, "err", err)
		writeError(w, http.StatusInternalServerError, msgInternal)
		return
	}

	// 302 (temporary) rather than 301 (permanent): browsers cache 301s
	// indefinitely, so later requests would bypass this server entirely.
	http.Redirect(w, r, target, http.StatusFound)
}

// isValidURL accepts only absolute http(s) URLs with a host. Restricting
// the scheme matters: storing e.g. "javascript:..." would turn this
// service into a redirector for malicious links.
func isValidURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	// url.Parse lowercases the scheme, so "HTTPS://..." is accepted too.
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	// Headers must be set before WriteHeader; afterwards they are ignored.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// The status line is already sent, so an encode error cannot change
	// the response; there is nothing useful to do with it.
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
