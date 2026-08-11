package postgres_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func stripComments(sql string) string {
	lines := strings.Split(sql, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		if idx := strings.Index(line, "--"); idx != -1 {
			line = line[:idx]
		}
		cleanLines = append(cleanLines, line)
	}
	return strings.Join(cleanLines, "\n")
}

// TestMigrations_DownFilesExistAndCleanup verifies migration integrity:
// 1. Every .up.sql file in migrations/ has a non-empty matching .down.sql file.
// 2. Every table created in .up.sql is dropped in .down.sql.
// 3. Every column added in .up.sql is removed in .down.sql.
// 4. Every enum type created in .up.sql is dropped in .down.sql.
func TestMigrations_DownFilesExistAndCleanup(t *testing.T) {
	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("Failed to read migrations directory: %v", err)
	}

	upFiles := make(map[string]string)
	downFiles := make(map[string]string)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".up.sql") {
			prefix := strings.TrimSuffix(name, ".up.sql")
			upFiles[prefix] = filepath.Join(migrationsDir, name)
		} else if strings.HasSuffix(name, ".down.sql") {
			prefix := strings.TrimSuffix(name, ".down.sql")
			downFiles[prefix] = filepath.Join(migrationsDir, name)
		}
	}

	if len(upFiles) == 0 {
		t.Fatalf("No .up.sql files found in %s", migrationsDir)
	}

	for prefix, upPath := range upFiles {
		downPath, exists := downFiles[prefix]
		if !exists {
			t.Errorf("Missing down migration for: %s (.up.sql exists at %s)", prefix, upPath)
			continue
		}

		downInfo, err := os.Stat(downPath)
		if err != nil || downInfo.Size() == 0 {
			t.Errorf("[%s] Down migration file is empty or unreadable (%s)", prefix, downPath)
			continue
		}

		upBytes, err := os.ReadFile(upPath)
		if err != nil {
			t.Errorf("Error reading %s: %v", upPath, err)
			continue
		}
		upContent := stripComments(string(upBytes))

		downBytes, err := os.ReadFile(downPath)
		if err != nil {
			t.Errorf("Error reading %s: %v", downPath, err)
			continue
		}
		downContent := stripComments(string(downBytes))

		// Check tables
		createTableRegex := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z0-9_]+)`)
		for _, match := range createTableRegex.FindAllStringSubmatch(upContent, -1) {
			tableName := match[1]
			dropRegex := regexp.MustCompile(`(?i)DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?` + tableName)
			if !dropRegex.MatchString(downContent) {
				t.Errorf("[%s] Table '%s' created in UP migration but not dropped in DOWN migration (%s)", prefix, tableName, downPath)
			}
		}

		// Check added columns
		addColumnRegex := regexp.MustCompile(`(?i)ALTER\s+TABLE\s+([a-zA-Z0-9_]+)\s+ADD\s+COLUMN\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z0-9_]+)`)
		for _, match := range addColumnRegex.FindAllStringSubmatch(upContent, -1) {
			tableName := match[1]
			colName := match[2]
			dropTableRegex := regexp.MustCompile(`(?i)DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?` + tableName)
			if !dropTableRegex.MatchString(downContent) {
				dropColRegex := regexp.MustCompile(`(?i)DROP\s+COLUMN\s+(?:IF\s+EXISTS\s+)?` + colName)
				if !dropColRegex.MatchString(downContent) {
					t.Errorf("[%s] Column '%s' added to '%s' in UP migration but neither table nor column dropped in DOWN migration (%s)", prefix, colName, tableName, downPath)
				}
			}
		}

		// Check created types
		createTypeRegex := regexp.MustCompile(`(?i)CREATE\s+TYPE\s+([a-zA-Z0-9_]+)`)
		for _, match := range createTypeRegex.FindAllStringSubmatch(upContent, -1) {
			typeName := match[1]
			dropTypeRegex := regexp.MustCompile(`(?i)DROP\s+TYPE\s+(?:IF\s+EXISTS\s+)?` + typeName)
			if !dropTypeRegex.MatchString(downContent) {
				t.Errorf("[%s] Type '%s' created in UP migration but not dropped in DOWN migration (%s)", prefix, typeName, downPath)
			}
		}
	}
}
