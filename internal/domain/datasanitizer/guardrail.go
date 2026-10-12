package datasanitizer

import (
	"strings"
)

var prodKeywords = map[string]struct{}{
	"prod":       {},
	"production": {},
	"prd":        {},
	"live":       {},
}

// IsProductionCluster checks if a cluster identifier signifies a production environment.
func IsProductionCluster(clusterID string) bool {
	return isProductionTokenPresent(clusterID)
}

// IsProductionTarget checks if either target cluster ID or target database name indicates production.
func IsProductionTarget(targetClusterID, targetDB string) bool {
	return IsProductionCluster(targetClusterID) || isProductionTokenPresent(targetDB)
}

func isProductionTokenPresent(val string) bool {
	normalized := strings.ToLower(strings.TrimSpace(val))
	if normalized == "" {
		return false
	}

	// Exact match check
	if _, ok := prodKeywords[normalized]; ok {
		return true
	}

	// Tokenize on common separators: -, _, ., /
	tokens := strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '/' || r == ':'
	})

	for _, token := range tokens {
		if _, ok := prodKeywords[token]; ok {
			return true
		}
	}

	return false
}
