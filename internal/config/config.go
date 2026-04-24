package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	Port        string
	LogLevel    string
	Env         string
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

	return &Config{
		DatabaseURL: dbURL,
		Port:        port,
		LogLevel:    logLevel,
		Env:         env,
	}, nil
}

func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}
