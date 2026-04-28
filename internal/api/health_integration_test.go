//go:build integration

package api

import (
	"net/http"
	"testing"
)

func TestHealthz_ReturnsDBStatus(t *testing.T) {
	server, cleanup := newIntegrationServer(t)
	defer cleanup()

	mux := newIntegrationMux(server)
	w := performAuthorizedJSONRequest(t, mux, http.MethodGet, "/api/v1/healthz", "", false)
	if w.Code != http.StatusOK {
		t.Fatalf("status code: got %d want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	resp := decodeJSON[healthzResponse](t, w)
	if resp.Status != "ok" {
		t.Fatalf("status: got %q want ok", resp.Status)
	}
	if resp.Env != "test" {
		t.Fatalf("env: got %q want test", resp.Env)
	}
	if resp.DB.Status != "ok" {
		t.Fatalf("db.status: got %q want ok", resp.DB.Status)
	}
}

func TestProtectedRoutes_RequireSharedSecret(t *testing.T) {
	server, cleanup := newIntegrationServer(t)
	defer cleanup()

	mux := newIntegrationMux(server)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create_task", method: http.MethodPost, path: "/api/v1/tasks", body: `{}`},
		{name: "create_attestation", method: http.MethodPost, path: "/api/v1/tasks/11111111-1111-1111-1111-111111111111/attestations", body: `{}`},
		{name: "preview_settlement", method: http.MethodPost, path: "/api/v1/tasks/11111111-1111-1111-1111-111111111111/settlement/preview", body: ""},
		{name: "commit_settlement", method: http.MethodPost, path: "/api/v1/tasks/11111111-1111-1111-1111-111111111111/settlement/commit", body: ""},
		{name: "list_audit_events", method: http.MethodGet, path: "/api/v1/audit-events", body: ""},
		{name: "get_audit_event", method: http.MethodGet, path: "/api/v1/audit-events/11111111-1111-1111-1111-111111111111", body: ""},
		{name: "ledger_query", method: http.MethodGet, path: "/api/v1/internal/lrs-ledger-query?uid=u&window_start_utc=2026-04-28T00:00:00Z&window_end_utc=2026-04-29T00:00:00Z", body: ""},
		{name: "attestation_index_query", method: http.MethodGet, path: "/api/v1/internal/attestation-index-query?uid=u&window_start_utc=2026-04-28T00:00:00Z&window_end_utc=2026-04-29T00:00:00Z", body: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performAuthorizedJSONRequest(t, mux, tc.method, tc.path, tc.body, false)
			if resp.Code != http.StatusUnauthorized {
				t.Fatalf("status code: got %d want %d; body=%s", resp.Code, http.StatusUnauthorized, resp.Body.String())
			}
		})
	}
}
