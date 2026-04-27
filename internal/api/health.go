package api

import (
	"context"
	"net/http"
	"time"
)

type healthzResponse struct {
	Status string         `json:"status"`
	Env    string         `json:"env"`
	DB     healthzDBState `json:"db"`
}

type healthzDBState struct {
	Status string `json:"status"`
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if s.Pool == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, healthzResponse{
			Status: "degraded",
			Env:    s.Env,
			DB:     healthzDBState{Status: "unavailable"},
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.Pool.Ping(ctx); err != nil {
		s.Logger.Error("healthz db ping", "error", err)
		s.writeJSON(w, http.StatusServiceUnavailable, healthzResponse{
			Status: "degraded",
			Env:    s.Env,
			DB:     healthzDBState{Status: "down"},
		})
		return
	}

	s.writeJSON(w, http.StatusOK, healthzResponse{
		Status: "ok",
		Env:    s.Env,
		DB:     healthzDBState{Status: "ok"},
	})
}
