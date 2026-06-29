package reporting

import "time"

// Report represents a generated or scheduled report.
type Report struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Type      string    `json:"type"`      // operational | security | compliance | cost | incident
	Format    string    `json:"format"`    // pdf | excel | csv
	Status    string    `json:"status"`    // pending | generating | completed | failed
	FileURL   string    `json:"file_url"`  // URL to download the report
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
