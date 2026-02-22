// cmd/migrate — applies all *.sql migrations in migrations/ against the target database.
// Uses the same schema_migrations table format as golang-migrate (bigint version + dirty flag)
// so the two tools are interchangeable.
//
// Usage:
//
//	go run ./cmd/migrate
//	DATABASE_URL=postgres://... go run ./cmd/migrate
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultDB = "postgres://nexus:nexus@localhost:5432/nexus?sslmode=disable"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = defaultDB
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	fmt.Println("connected to database")

	// Ensure tracking table exists — same schema as golang-migrate.
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version bigint NOT NULL,
			dirty   boolean NOT NULL,
			PRIMARY KEY (version)
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// Collect *.sql files.
	entries, err := os.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	applied := 0
	for _, f := range files {
		// Extract numeric prefix: "001_init.up.sql" → 1
		ver, err := versionFromFile(f)
		if err != nil {
			return fmt.Errorf("parse version from %s: %w", f, err)
		}

		var exists bool
		if err := db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1 AND dirty = false)`,
			ver,
		).Scan(&exists); err != nil {
			return fmt.Errorf("check %s: %w", f, err)
		}
		if exists {
			fmt.Printf("  skip  %s (already applied)\n", f)
			continue
		}

		sql, err := os.ReadFile(filepath.Join("migrations", f))
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}

		// Mark dirty=true before applying so a crash is visible.
		if _, err := db.Exec(ctx,
			`INSERT INTO schema_migrations (version, dirty) VALUES ($1, true)
			 ON CONFLICT (version) DO UPDATE SET dirty = true`, ver,
		); err != nil {
			return fmt.Errorf("mark dirty %s: %w", f, err)
		}

		if _, err := db.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply %s: %w", f, err)
		}

		if _, err := db.Exec(ctx,
			`UPDATE schema_migrations SET dirty = false WHERE version = $1`, ver,
		); err != nil {
			return fmt.Errorf("mark clean %s: %w", f, err)
		}

		fmt.Printf("  apply %s\n", f)
		applied++
	}

	if applied == 0 {
		fmt.Println("nothing to migrate — already up to date")
	} else {
		fmt.Printf("applied %d migration(s)\n", applied)
	}
	return nil
}

// versionFromFile extracts the leading integer from a migration filename.
// "001_init.up.sql" → 1, "004_account_recovery.up.sql" → 4
func versionFromFile(filename string) (int64, error) {
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) == 0 {
		return 0, fmt.Errorf("unexpected filename format")
	}
	return strconv.ParseInt(parts[0], 10, 64)
}
