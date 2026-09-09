// Package main provides a minimal HTTP GET health check probe for distroless containers.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	url := "http://localhost:8080/livez"
	if len(os.Args) > 1 && os.Args[1] != "" {
		url = os.Args[1]
	}

	timeout := 3 * time.Second
	if len(os.Args) > 2 {
		if d, err := time.ParseDuration(os.Args[2]); err == nil {
			timeout = d
		}
	}

	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "health probe error connecting to %s: %v\n", url, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "health probe HTTP %d from %s\n", resp.StatusCode, url)
		os.Exit(1)
	}

	os.Exit(0)
}
