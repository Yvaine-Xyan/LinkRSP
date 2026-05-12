package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	var heartbeatID string
	err = conn.QueryRow(ctx, `
		INSERT INTO ops_heartbeats(source, note)
		VALUES ($1, $2)
		RETURNING heartbeat_id
	`, "local_keepalive", "manual_heartbeat").Scan(&heartbeatID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert heartbeat: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("ok: inserted ops_heartbeats heartbeat_id=%s\n", heartbeatID)
}

