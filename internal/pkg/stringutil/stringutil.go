package stringutil

// Truncate returns a string truncated to maxLen.
// If the string length is greater than maxLen, it appends "...".
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
