package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/pkg/logger"
)

// Publisher publishes events to NATS JetStream subjects.
type Publisher struct {
	js jetstream.JetStream
}

// NewPublisher creates a new JetStream publisher.
func NewPublisher(client *Client) *Publisher {
	return &Publisher{js: client.JetStream()}
}

// Publish serializes the payload to JSON and publishes to the given subject.
func (p *Publisher) Publish(ctx context.Context, subject string, payload interface{}) error {
	log := logger.WithContext(ctx)

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling event payload: %w", err)
	}

	ack, err := p.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("publishing to subject %s: %w", subject, err)
	}

	log.Debug("published event to JetStream",
		zap.String("subject", subject),
		zap.Uint64("sequence", ack.Sequence),
		zap.String("stream", ack.Stream),
	)

	return nil
}

// PublishRaw publishes raw bytes to the given subject.
func (p *Publisher) PublishRaw(ctx context.Context, subject string, data []byte) error {
	log := logger.WithContext(ctx)

	ack, err := p.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("publishing raw data to subject %s: %w", subject, err)
	}

	log.Debug("published raw event to JetStream",
		zap.String("subject", subject),
		zap.Uint64("sequence", ack.Sequence),
	)

	return nil
}
