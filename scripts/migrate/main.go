package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Environment variables supported (with fallback):
//   DB_HOST / K8S_POSTGRES_HOST                             - PostgreSQL host (default: "localhost")
//   DB_PORT / K8S_POSTGRES_PORT                             - PostgreSQL port (default: "5432")
//   DB_USER / K8S_POSTGRES_USER                             - PostgreSQL user (default: "myuser")
//   POSTGRES_PASSWORD / DB_PASSWORD / K8S_POSTGRES_PASSWORD - PostgreSQL password (REQUIRED)
//   DB_NAME / K8S_POSTGRES_DBNAME                           - PostgreSQL database name (default: "mydatabase")
//   DB_SSLMODE / K8S_POSTGRES_SSLMODE                       - SSL mode (default: "disable")

// SQL queries for migration locking and timeouts
const (
	AdvisoryLockQuery   = "SELECT pg_advisory_lock(hashtext('k8s_selfhost_migrations'));"
	AdvisoryUnlockQuery = "SELECT pg_advisory_unlock(hashtext('k8s_selfhost_migrations'));"
	SetLockTimeoutQuery = "SET lock_timeout = '2s'; SET statement_timeout = '30s';"
)

// MigrationExecutor defines the interface for executing migration SQL statements.
type MigrationExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func getEnv(primary, secondary, fallback string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	if v := os.Getenv(secondary); v != "" {
		return v
	}
	return fallback
}

// RunMigrations executes migration files sequentially under a PostgreSQL advisory lock and lock timeout.
func RunMigrations(ctx context.Context, conn MigrationExecutor, files []string, directionName string) error {
	fmt.Println("Acquiring PostgreSQL advisory lock for migrations...")
	if _, err := conn.Exec(ctx, AdvisoryLockQuery); err != nil {
		return fmt.Errorf("acquiring advisory lock: %w", err)
	}

	defer func() {
		fmt.Println("Releasing PostgreSQL advisory lock...")
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, unlockErr := conn.Exec(unlockCtx, AdvisoryUnlockQuery); unlockErr != nil {
			fmt.Printf("Warning: failed to release advisory lock: %v\n", unlockErr)
		}
	}()

	fmt.Println("Setting lock_timeout = '2s' and statement_timeout = '30s'...")
	if _, err := conn.Exec(ctx, SetLockTimeoutQuery); err != nil {
		return fmt.Errorf("setting lock_timeout: %w", err)
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", file, err)
			return fmt.Errorf("reading migration file %s: %w", file, err)
		}

		fmt.Printf("Applying %s... ", filepath.Base(file))

		_, err = conn.Exec(ctx, string(content))
		if err != nil {
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "duplicate key") {
				fmt.Println("Already applied (or partially).")
			} else {
				fmt.Printf("Error: %v\n", err)
				return fmt.Errorf("executing migration %s: %w", file, err)
			}
		} else {
			fmt.Println("Success.")
		}
	}
	fmt.Printf("Migrations %s complete!\n", directionName)
	return nil
}

func main() {
	downFlag := flag.Bool("down", false, "Run down migrations instead of up migrations")
	flag.Parse()

	host := getEnv("DB_HOST", "K8S_POSTGRES_HOST", "localhost")
	port := getEnv("DB_PORT", "K8S_POSTGRES_PORT", "5432")
	user := getEnv("DB_USER", "K8S_POSTGRES_USER", "myuser")
	password := getEnv("POSTGRES_PASSWORD", "DB_PASSWORD", os.Getenv("K8S_POSTGRES_PASSWORD"))
	if password == "" {
		fmt.Fprintln(os.Stderr, "error: postgres password is required: please set POSTGRES_PASSWORD, DB_PASSWORD, or K8S_POSTGRES_PASSWORD")
		os.Exit(1)
	}
	dbName := getEnv("DB_NAME", "K8S_POSTGRES_DBNAME", "mydatabase")
	sslMode := getEnv("DB_SSLMODE", "K8S_POSTGRES_SSLMODE", "disable")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbName, sslMode,
	)

	pattern := "migrations/*.up.sql"
	directionName := "UP"
	if *downFlag || (len(os.Args) > 1 && os.Args[1] == "down") {
		pattern = "migrations/*.down.sql"
		directionName = "DOWN"
	}

	fmt.Printf("Connecting to PostgreSQL at %s:%s (database: %s)...\n", host, port, dbName)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		fmt.Printf("Failed to ping database: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Connected successfully!")

	fmt.Printf("Running %s migrations...\n", directionName)
	files, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Printf("Failed to read migrations directory: %v\n", err)
		os.Exit(1)
	}

	sort.Strings(files)
	if directionName == "DOWN" {
		for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
			files[i], files[j] = files[j], files[i]
		}
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		fmt.Printf("Failed to acquire connection: %v\n", err)
		os.Exit(1)
	}
	defer conn.Release()

	if err := RunMigrations(ctx, conn, files, directionName); err != nil {
		fmt.Printf("Migration run failed: %v\n", err)
		os.Exit(1)
	}
}
