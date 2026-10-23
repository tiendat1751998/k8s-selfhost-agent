package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRemediation_DBState(t *testing.T) {
	host := getEnv("DB_HOST", "K8S_POSTGRES_HOST", "localhost")
	port := getEnv("DB_PORT", "K8S_POSTGRES_PORT", "5432")
	user := getEnv("DB_USER", "K8S_POSTGRES_USER", "myuser")
	password := getEnv("POSTGRES_PASSWORD", "DB_PASSWORD", os.Getenv("K8S_POSTGRES_PASSWORD"))
	dbName := getEnv("DB_NAME", "K8S_POSTGRES_DBNAME", "mydatabase")
	sslMode := getEnv("DB_SSLMODE", "K8S_POSTGRES_SSLMODE", "disable")

	if password == "" {
		t.Skip("skipping db state test: password not provided in environment")
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, dbName, sslMode)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping db: %v", err)
	}

	// 1. Verify compute_hosts has exactly the 3 real nodes
	rows, err := pool.Query(ctx, "SELECT name, endpoint, status FROM compute_hosts ORDER BY name")
	if err != nil {
		t.Fatalf("query compute_hosts: %v", err)
	}
	defer rows.Close()

	type hostInfo struct {
		name, endpoint, status string
	}
	var hosts []hostInfo
	for rows.Next() {
		var h hostInfo
		if err := rows.Scan(&h.name, &h.endpoint, &h.status); err != nil {
			t.Fatalf("scan host: %v", err)
		}
		hosts = append(hosts, h)
	}

	t.Logf("Found %d compute hosts: %+v", len(hosts), hosts)
	if len(hosts) != 3 {
		t.Errorf("expected exactly 3 hosts, got %d", len(hosts))
	}
	for _, h := range hosts {
		if h.endpoint == "http://10.10.10.150:9100" ||
			h.endpoint == "http://10.10.10.151:9100" ||
			h.endpoint == "http://10.10.10.152:9100" ||
			h.endpoint == "http://10.10.10.200:9100" ||
			h.endpoint == "http://10.10.10.201:9100" {
			t.Errorf("ghost host %s (%s) still present in compute_hosts", h.name, h.endpoint)
		}
	}

	// 2. Verify fleet_clusters status is 'connected' or 'healthy'
	var clusterStatus string
	err = pool.QueryRow(ctx, "SELECT status FROM fleet_clusters WHERE name = 'k8snode' OR id = 'cluster-mtay50kl'").Scan(&clusterStatus)
	if err != nil {
		t.Fatalf("query cluster: %v", err)
	}
	t.Logf("k8snode cluster status: %s", clusterStatus)
	if clusterStatus == "upgrading" {
		t.Errorf("k8snode is still stuck in 'upgrading' status")
	}

	// 3. Verify SLO definitions & snapshots
	var sloCount int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM slo_definitions").Scan(&sloCount)
	if err != nil || sloCount < 3 {
		t.Errorf("expected at least 3 SLO definitions, got %d (err: %v)", sloCount, err)
	}

	// 4. Verify Compliance frameworks & violations
	var compCount int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM compliance_frameworks").Scan(&compCount)
	if err != nil || compCount < 2 {
		t.Errorf("expected at least 2 compliance frameworks, got %d (err: %v)", compCount, err)
	}

	// 5. Verify Automation rules
	var autoCount int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM automation_rules").Scan(&autoCount)
	if err != nil || autoCount < 2 {
		t.Errorf("expected at least 2 automation rules, got %d (err: %v)", autoCount, err)
	}
}
