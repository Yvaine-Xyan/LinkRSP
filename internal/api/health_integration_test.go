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
	w := performJSONRequest(t, mux, http.MethodGet, "/api/v1/healthz", "")
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
