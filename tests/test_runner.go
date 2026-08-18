package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type TestSuite struct {
	Name    string
	Command string
	Args    []string
}

func main() {
	fmt.Println("=======================================================================")
	fmt.Println(" 🚀 K8S-SELFHOST ENTERPRISE PLATFORM — UNIFIED TEST SUITE RUNNER")
	fmt.Println("=======================================================================")

	suites := []TestSuite{
		{
			Name:    "1. Domain & Clean Architecture Rules",
			Command: "go",
			Args:    []string{"test", "-v", "./internal/domain/..."},
		},
		{
			Name:    "2. Dual-Target Database Backup & 8 DB Drivers",
			Command: "go",
			Args:    []string{"test", "-v", "./internal/infrastructure/backup/..."},
		},
		{
			Name:    "3. SRE Telegram Bot Copilot & Alert Debouncer",
			Command: "go",
			Args:    []string{"test", "-v", "./internal/infrastructure/telegram/..."},
		},
		{
			Name:    "4. Real-Time Log Engine & WebSocket Streamer",
			Command: "go",
			Args:    []string{"test", "-v", "./internal/infrastructure/logging/..."},
		},
		{
			Name:    "5. DevSecOps Vault, Trivy CVE & Checkov IaC Scanners",
			Command: "go",
			Args:    []string{"test", "-v", "./internal/infrastructure/security/..."},
		},
		{
			Name:    "6. Infrastructure as Code (Terraform & Ansible Runners)",
			Command: "go",
			Args:    []string{"test", "-v", "./internal/infrastructure/iac/..."},
		},
		{
			Name:    "7. HTTP API Adapter & Middleware Security Checks",
			Command: "go",
			Args:    []string{"test", "-v", "./internal/adapter/http/..."},
		},
		{
			Name:    "8. Full Platform E2E Integration Test Suite",
			Command: "go",
			Args:    []string{"test", "-v", "./tests/integration/..."},
		},
	}

	startAll := time.Now()
	allPassed := true

	for _, s := range suites {
		fmt.Printf("\n▶ Running Suite: %s\n", s.Name)
		start := time.Now()
		cmd := exec.Command(s.Command, s.Args...)
		output, err := cmd.CombinedOutput()
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("❌ FAILED (%s)\n", duration.Round(time.Millisecond))
			fmt.Println(string(output))
			allPassed = false
		} else {
			// Print summary line
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			lastLine := ""
			if len(lines) > 0 {
				lastLine = lines[len(lines)-1]
			}
			fmt.Printf("✅ PASSED (%s) -> %s\n", duration.Round(time.Millisecond), lastLine)
		}
	}

	fmt.Println("\n=======================================================================")
	if allPassed {
		fmt.Printf("🎉 ALL TEST SUITES PASSED SUCCESSFULLY! Total time: %s\n", time.Since(startAll).Round(time.Millisecond))
		fmt.Println("=======================================================================")
	} else {
		fmt.Printf("💥 SOME TEST SUITES FAILED! Total time: %s\n", time.Since(startAll).Round(time.Millisecond))
		fmt.Println("=======================================================================")
		os.Exit(1)
	}
}
