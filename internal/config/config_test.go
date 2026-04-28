package config

import (
	"testing"
	"time"
)

func TestLoad_UsesDefaultsForSettlementRuntimeConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("ENV", "")
	t.Setenv("R006_ACCELERATION_THRESHOLD", "")
	t.Setenv("R010_GENESIS_END_TIME", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.R006AccelerationThreshold != 1000 {
		t.Fatalf("R006AccelerationThreshold: got %.2f want 1000", cfg.R006AccelerationThreshold)
	}
	wantGenesis := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	if !cfg.R010GenesisEndTime.Equal(wantGenesis) {
		t.Fatalf("R010GenesisEndTime: got %s want %s", cfg.R010GenesisEndTime.Format(time.RFC3339), wantGenesis.Format(time.RFC3339))
	}
}

func TestLoad_ParsesSettlementRuntimeConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("API_SHARED_SECRET", "secret-123")
	t.Setenv("R006_ACCELERATION_THRESHOLD", "12.5")
	t.Setenv("R010_GENESIS_END_TIME", "2026-04-28T00:00:00+08:00")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.APISharedSecret != "secret-123" {
		t.Fatalf("APISharedSecret: got %q want %q", cfg.APISharedSecret, "secret-123")
	}
	if cfg.R006AccelerationThreshold != 12.5 {
		t.Fatalf("R006AccelerationThreshold: got %.2f want 12.5", cfg.R006AccelerationThreshold)
	}
	wantGenesis := time.Date(2026, 4, 27, 16, 0, 0, 0, time.UTC)
	if !cfg.R010GenesisEndTime.Equal(wantGenesis) {
		t.Fatalf("R010GenesisEndTime: got %s want %s", cfg.R010GenesisEndTime.Format(time.RFC3339), wantGenesis.Format(time.RFC3339))
	}
}

func TestLoad_RejectsInvalidSettlementRuntimeConfig(t *testing.T) {
	t.Run("threshold", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://example")
		t.Setenv("R006_ACCELERATION_THRESHOLD", "bad")
		t.Setenv("R010_GENESIS_END_TIME", "")

		_, err := Load()
		if err == nil || err.Error() != "invalid R006_ACCELERATION_THRESHOLD: strconv.ParseFloat: parsing \"bad\": invalid syntax" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("genesis_end_time", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://example")
		t.Setenv("R006_ACCELERATION_THRESHOLD", "")
		t.Setenv("R010_GENESIS_END_TIME", "bad")

		_, err := Load()
		if err == nil || err.Error() != "invalid R010_GENESIS_END_TIME: parsing time \"bad\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"bad\" as \"2006\"" {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
