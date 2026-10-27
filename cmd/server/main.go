// Command server runs the URL shortener HTTP API.
package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/kimi0217/go-url-shortener/internal/handler"
	"github.com/kimi0217/go-url-shortener/internal/store"
)

func main() {
	// JSON logs are easy for log collectors to parse; slog.SetDefault also
	// routes any stray log/slog package-level calls through this logger.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	port := getenv("PORT", "8080")
	// BASE_URL is configurable because behind a proxy or in a container the
	// public address differs from the port the process listens on.
	baseURL := getenv("BASE_URL", "http://localhost:"+port)

	h := handler.New(store.NewMemory(), baseURL, logger)

	// An explicit http.Server instead of http.ListenAndServe, whose defaults
	// have no timeouts: a slow or malicious client could then hold a
	// connection (and its goroutine) open forever.
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: h.Routes(),
		// Time allowed to send request headers. Guards against Slowloris,
		// where a client trickles headers to tie up connections.
		ReadHeaderTimeout: 5 * time.Second,
		// Time allowed to read the whole request, headers plus body.
		ReadTimeout: 10 * time.Second,
		// Time allowed from the end of reading the request to finishing the
		// response. Our handlers are fast, so this only cuts off clients
		// that read the response too slowly.
		WriteTimeout: 10 * time.Second,
		// How long an idle keep-alive connection is kept for reuse.
		IdleTimeout: 60 * time.Second,
		// Send the server's internal errors (e.g. TLS handshake failures)
		// through slog so all output shares one format.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("server starting", "addr", srv.Addr, "base_url", baseURL)

	// ListenAndServe blocks until the server stops and always returns a
	// non-nil error. ErrServerClosed only means a deliberate Shutdown.
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// getenv returns the environment variable key, or fallback when it is unset
// or empty. Treating empty as unset avoids e.g. PORT="" producing ":".
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
