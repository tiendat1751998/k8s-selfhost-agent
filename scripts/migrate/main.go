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
	dsn := "postgres://myuser:mysecretpassword@10.10.10.133:5432/mydatabase?sslmode=disable"
	
	fmt.Println("Connecting to PostgreSQL at 10.10.10.133...")
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
	files, err := filepath.Glob("migrations/*.sql")
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
