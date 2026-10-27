package http

import (
	"strings"
)

type createAlertChannelRequest struct {
	Name    string                 `json:"name"`
	Type    string                 `json:"type"`
	Config  map[string]interface{} `json:"config"`
	Enabled bool                   `json:"enabled"`
}

func (r *createAlertChannelRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.Type) == "" {
		ve.Add("type", "type is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type createAlertRuleRequest struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	MetricName      string   `json:"metric_name"`
	Condition       string   `json:"condition"`
	Threshold       float64  `json:"threshold"`
	DurationSeconds int      `json:"duration_seconds"`
	Severity        string   `json:"severity"`
	ChannelIDs      []string `json:"channel_ids"`
	Enabled         bool     `json:"enabled"`
}

func (r *createAlertRuleRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Name) == "" {
		ve.Add("name", "name is required")
	}
	if strings.TrimSpace(r.MetricName) == "" {
		ve.Add("metric_name", "metric_name is required")
	}
	if strings.TrimSpace(r.Condition) == "" {
		ve.Add("condition", "condition is required")
	}
	if r.Threshold < 0 {
		ve.Add("threshold", "threshold must be positive")
	}
	if r.DurationSeconds <= 0 {
		ve.Add("duration_seconds", "duration_seconds must be greater than 0")
	}
	if strings.TrimSpace(r.Severity) == "" {
		ve.Add("severity", "severity is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type updateAlertRuleRequest struct {
	Name            *string   `json:"name"`
	Description     *string   `json:"description"`
	MetricName      *string   `json:"metric_name"`
	Condition       *string   `json:"condition"`
	Threshold       *float64  `json:"threshold"`
	DurationSeconds *int      `json:"duration_seconds"`
	Severity        *string   `json:"severity"`
	ChannelIDs      *[]string `json:"channel_ids"`
	Enabled         *bool     `json:"enabled"`
}

func (r *updateAlertRuleRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if r.Name != nil && strings.TrimSpace(*r.Name) == "" {
		ve.Add("name", "name cannot be empty")
	}
	if r.MetricName != nil && strings.TrimSpace(*r.MetricName) == "" {
		ve.Add("metric_name", "metric_name cannot be empty")
	}
	if r.Condition != nil && strings.TrimSpace(*r.Condition) == "" {
		ve.Add("condition", "condition cannot be empty")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type createSilenceRequest struct {
	AlertID         string `json:"alert_id"`
	DurationMinutes int    `json:"duration_minutes"`
	Comment         string `json:"comment"`
}

func (r *createSilenceRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.AlertID) == "" {
		ve.Add("alert_id", "alert_id is required")
	}
	if r.DurationMinutes <= 0 {
		ve.Add("duration_minutes", "duration_minutes must be greater than 0")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}
