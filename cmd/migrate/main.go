package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 创建迁移记录表（幂等）
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create schema_migrations: %v\n", err)
		os.Exit(1)
	}

	files, err := filepath.Glob("db/migrations/*.sql")
	if err != nil || len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no migration files found in db/migrations/")
		os.Exit(1)
	}
	sort.Strings(files)

	applied := 0
	for _, f := range files {
		name := filepath.Base(f)

		var exists bool
		err = pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename=$1)", name,
		).Scan(&exists)
		if err != nil {
			fmt.Fprintf(os.Stderr, "check %s: %v\n", name, err)
			os.Exit(1)
		}
		if exists {
			fmt.Printf("skip  %s (already applied)\n", name)
			continue
		}

		sql, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", f, err)
			os.Exit(1)
		}

		// 逐条执行，跳过 CREATE RULE（Supabase pooler 不支持）
		statements := filterUnsupported(string(sql))
		failed := false
		for i, stmt := range statements {
			if _, err = pool.Exec(ctx, stmt); err != nil {
				fmt.Fprintf(os.Stderr, "apply %s stmt[%d]: %v\nSQL: %s\n", name, i, err, stmt)
				failed = true
				break
			}
		}
		if failed {
			os.Exit(1)
		}

		_, err = pool.Exec(ctx,
			"INSERT INTO schema_migrations(filename) VALUES($1)", name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "record %s: %v\n", name, err)
			os.Exit(1)
		}
		fmt.Printf("ok    %s\n", name)
		applied++
	}

	if applied == 0 {
		fmt.Println("nothing to migrate")
	} else {
		fmt.Printf("done  %d migration(s) applied\n", applied)
	}
}

// filterUnsupported 移除 CREATE RULE 语句，返回可逐条执行的语句列表
func filterUnsupported(sql string) []string {
	var kept []string
	for _, stmt := range splitStatements(sql) {
		upper := strings.ToUpper(strings.TrimSpace(stmt))
		if strings.HasPrefix(upper, "CREATE OR REPLACE RULE") ||
			strings.HasPrefix(upper, "CREATE RULE") {
			continue
		}
		kept = append(kept, stmt)
	}
	return kept
}

func splitStatements(sql string) []string {
	var stmts []string
	for _, s := range strings.Split(sql, ";") {
		s = strings.TrimSpace(s)
		// 跳过空行和纯注释行
		if s == "" {
			continue
		}
		lines := strings.Split(s, "\n")
		nonComment := false
		for _, l := range lines {
			if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "--") {
				nonComment = true
				break
			}
		}
		if nonComment {
			stmts = append(stmts, s)
		}
	}
	return stmts
}
