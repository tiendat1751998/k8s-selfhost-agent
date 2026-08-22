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

	"github.com/jackc/pgx/v5/pgxpool"
)

// Environment variables supported (with fallback):
//   DB_HOST / K8S_POSTGRES_HOST         - PostgreSQL host (default: "localhost")
//   DB_PORT / K8S_POSTGRES_PORT         - PostgreSQL port (default: "5432")
//   DB_USER / K8S_POSTGRES_USER         - PostgreSQL user (default: "myuser")
//   DB_PASSWORD / K8S_POSTGRES_PASSWORD - PostgreSQL password (default: "mysecretpassword")
//   DB_NAME / K8S_POSTGRES_DBNAME       - PostgreSQL database name (default: "mydatabase")
//   DB_SSLMODE / K8S_POSTGRES_SSLMODE   - SSL mode (default: "disable")

func getEnv(primary, secondary, fallback string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	if v := os.Getenv(secondary); v != "" {
		return v
	}
	return fallback
}

func main() {
	downFlag := flag.Bool("down", false, "Run down migrations instead of up migrations")
	flag.Parse()

	host := getEnv("DB_HOST", "K8S_POSTGRES_HOST", "localhost")
	port := getEnv("DB_PORT", "K8S_POSTGRES_PORT", "5432")
	user := getEnv("DB_USER", "K8S_POSTGRES_USER", "myuser")
	password := getEnv("DB_PASSWORD", "K8S_POSTGRES_PASSWORD", "mysecretpassword")
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
		// Reverse order for down migrations
		for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
			files[i], files[j] = files[j], files[i]
		}
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", file, err)
			continue
		}

		fmt.Printf("Applying %s... ", filepath.Base(file))

		_, err = pool.Exec(ctx, string(content))
		if err != nil {
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "duplicate key") {
				fmt.Println("Already applied (or partially).")
			} else {
				fmt.Printf("Error: %v\n", err)
			}
		} else {
			fmt.Println("Success.")
		}
	}
	fmt.Printf("Migrations %s complete!\n", directionName)
}
