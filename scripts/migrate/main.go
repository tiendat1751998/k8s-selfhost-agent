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

// Required environment variables:
//   DB_HOST     - PostgreSQL host (e.g., "localhost")
//   DB_PORT     - PostgreSQL port (e.g., "5432")
//   DB_USER     - PostgreSQL user
//   DB_PASSWORD - PostgreSQL password
//   DB_NAME     - PostgreSQL database name
//
// Optional environment variables:
//   DB_SSLMODE  - SSL mode (default: "disable")

func main() {
	required := map[string]string{
		"DB_HOST":     os.Getenv("DB_HOST"),
		"DB_PORT":     os.Getenv("DB_PORT"),
		"DB_USER":     os.Getenv("DB_USER"),
		"DB_PASSWORD": os.Getenv("DB_PASSWORD"),
		"DB_NAME":     os.Getenv("DB_NAME"),
	}

	var missing []string
	for k, v := range required {
		if v == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		fmt.Fprintf(os.Stderr, "Missing required environment variables: %s\n", strings.Join(missing, ", "))
		os.Exit(1)
	}

	sslMode := os.Getenv("DB_SSLMODE")
	if sslMode == "" {
		sslMode = "disable"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		required["DB_USER"], required["DB_PASSWORD"],
		required["DB_HOST"], required["DB_PORT"],
		required["DB_NAME"], sslMode,
	)

	fmt.Printf("Connecting to PostgreSQL at %s:%s...\n", required["DB_HOST"], required["DB_PORT"])
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	fmt.Println("Running migrations...")
	files, err := filepath.Glob("migrations/*.up.sql")
	if err != nil {
		fmt.Printf("Failed to read migrations directory: %v\n", err)
		os.Exit(1)
	}

	sort.Strings(files)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", file, err)
			continue
		}

		fmt.Printf("Applying %s... ", filepath.Base(file))
		
		// Split multiple statements if necessary, but Exec usually handles simple multiple statements
		_, err = pool.Exec(context.Background(), string(content))
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
	fmt.Println("Setup complete!")
}
