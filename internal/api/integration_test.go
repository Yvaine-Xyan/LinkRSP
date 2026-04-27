//go:build integration

package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Yvaine-Xyan/linkrsp/internal/db"
	"github.com/google/uuid"
)

func newIntegrationServer(t *testing.T) (*Server, func()) {
	t.Helper()

	databaseURL := os.Getenv("LINKRSP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("LINKRSP_TEST_DATABASE_URL is required for integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}

	cleanupDatabase(t, pool)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &Server{Pool: pool, Logger: logger, Env: "test"}
	return server, func() {
		cleanupDatabase(t, pool)
		pool.Close()
	}
}

func newIntegrationMux(server *Server) *http.ServeMux {
	mux := http.NewServeMux()
	server.RegisterHealthRoutes(mux)
	server.RegisterRoutes(mux)
	return mux
}

func cleanupDatabase(t *testing.T, pool *db.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := pool.Exec(ctx, `
		TRUNCATE TABLE audit_events, ledger_entries, attestations, tasks
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate test tables: %v", err)
	}
}

func performJSONRequest(t *testing.T, mux *http.ServeMux, method, path string, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func decodeJSON[T any](t *testing.T, body *httptest.ResponseRecorder) T {
	t.Helper()

	var out T
	if err := json.Unmarshal(body.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode json: %v; body=%s", err, body.Body.String())
	}
	return out
}

func mustParseRFC3339(t *testing.T, raw string) time.Time {
	t.Helper()
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse RFC3339 %q: %v", raw, err)
	}
	return value.UTC()
}

func mustUUIDString() string {
	return uuid.NewString()
}
