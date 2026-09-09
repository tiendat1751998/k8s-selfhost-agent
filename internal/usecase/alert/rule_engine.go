package alert

import (
	"context"
	"fmt"
	"log"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
)

type RuleEngineOption func(*RuleEngine)

func WithLogger(logger *zap.Logger) RuleEngineOption {
	return func(e *RuleEngine) {
		e.logger = logger
	}
}

type RuleEngine struct {
	repo      alert.Repository
	notifiers map[string]alert.Notifier
	logger    *zap.Logger
}

func NewRuleEngine(repo alert.Repository, notifiers map[string]alert.Notifier, opts ...RuleEngineOption) (*RuleEngine, error) {
	if repo == nil {
		return nil, fmt.Errorf("alert repository is required")
	}
	e := &RuleEngine{
		repo:      repo,
		notifiers: notifiers,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e, nil
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
								if err := notifier.Send(context.Background(), c, m); err != nil {
									if e.logger != nil {
										e.logger.Error("failed to send alert notification",
											zap.String("channel_id", c.ID),
											zap.String("channel_type", c.Type),
											zap.Error(err),
										)
									} else {
										log.Printf("failed to send alert notification to %s (%s): %v", c.ID, c.Type, err)
									}
								}
							}(ch, msg)
						}
					}
				}
			}
		}
	}
	return nil
}
