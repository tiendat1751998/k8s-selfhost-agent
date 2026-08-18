package alert

import (
	"context"
	"fmt"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
)

type RuleEngine struct {
	repo      alert.Repository
	notifiers map[string]alert.Notifier
}

func NewRuleEngine(repo alert.Repository, notifiers map[string]alert.Notifier) *RuleEngine {
	return &RuleEngine{
		repo:      repo,
		notifiers: notifiers,
	}
}

func (e *RuleEngine) EvaluateRule(ctx context.Context, metricName string, currentValue float64, tenantID string) error {
	rules, err := e.repo.ListRules(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to list rules: %w", err)
	}

	for _, rule := range rules {
		if !rule.Enabled || rule.MetricName != metricName {
			continue
		}

		firing := false
		switch rule.Condition {
		case "gt":
			firing = currentValue > rule.Threshold
		case "lt":
			firing = currentValue < rule.Threshold
		case "eq":
			firing = currentValue == rule.Threshold
		case "ne":
			firing = currentValue != rule.Threshold
		}

		if firing {
			msg := fmt.Sprintf("Alert %s triggered! Metric %s value %f breached threshold %f", rule.Name, metricName, currentValue, rule.Threshold)

			history := &alert.AlertHistory{
				TenantID: tenantID,
				RuleID:   rule.ID,
				Status:   "firing",
				Value:    currentValue,
				Message:  msg,
			}
			if err := e.repo.CreateHistory(ctx, history); err != nil {
				continue // skip on failure
			}

			channels, err := e.repo.ListChannels(ctx, tenantID)
			if err == nil {
				channelMap := make(map[string]*alert.NotificationChannel)
				for _, ch := range channels {
					channelMap[ch.ID] = ch
				}

				for _, chID := range rule.ChannelIDs {
					if ch, ok := channelMap[chID]; ok && ch.Enabled {
						if notifier, exists := e.notifiers[ch.Type]; exists {
							go func(c *alert.NotificationChannel, m string) {
								_ = notifier.Send(context.Background(), c, m)
							}(ch, msg)
						}
					}
				}
			}
		}
	}
	return nil
}
