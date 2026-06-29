package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := "postgres://myuser:mysecretpassword@10.10.10.133:5432/mydatabase?sslmode=disable"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, "SELECT id, name FROM fleet_clusters")
	if err != nil {
		fmt.Printf("Failed to query: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	fmt.Println("Clusters in DB:")
	count := 0
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Printf("- %s (ID: %s)\n", name, id)
		count++
	}
	fmt.Printf("Total: %d\n", count)
}
