package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL                 string
	Port                        string
	LogLevel                    string
	Env                         string
	APISharedSecret             string
	R006AccelerationThreshold   float64
	R010GenesisEndTime          time.Time
}

func Load() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	apiSharedSecret := os.Getenv("API_SHARED_SECRET")

	r006AccelerationThreshold := 1000.0
	r006AccelerationThresholdRaw := os.Getenv("R006_ACCELERATION_THRESHOLD")
	if r006AccelerationThresholdRaw != "" {
		parsed, err := strconv.ParseFloat(r006AccelerationThresholdRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid R006_ACCELERATION_THRESHOLD: %w", err)
		}
		r006AccelerationThreshold = parsed
	}

	r010GenesisEndTime := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	r010GenesisEndTimeRaw := os.Getenv("R010_GENESIS_END_TIME")
	if r010GenesisEndTimeRaw != "" {
		parsed, err := time.Parse(time.RFC3339, r010GenesisEndTimeRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid R010_GENESIS_END_TIME: %w", err)
		}
		r010GenesisEndTime = parsed.UTC()
	}

	return &Config{
		DatabaseURL:               dbURL,
		Port:                      port,
		LogLevel:                  logLevel,
		Env:                       env,
		APISharedSecret:           apiSharedSecret,
		R006AccelerationThreshold: r006AccelerationThreshold,
		R010GenesisEndTime:        r010GenesisEndTime,
	}, nil
}

func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}
