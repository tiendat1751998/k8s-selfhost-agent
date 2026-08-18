package nats

import (
	"context"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// Subscriber consumes events from NATS JetStream.
type Subscriber struct {
	client *Client
	js     jetstream.JetStream
}

// NewSubscriber creates a new JetStream subscriber.
func NewSubscriber(client *Client) *Subscriber {
	return &Subscriber{
		client: client,
		js:     client.JetStream(),
	}
}

// Subscribe creates a durable consumer and starts consuming messages.
func (s *Subscriber) Subscribe(ctx context.Context, subject string, handler func(ctx context.Context, subject string, data []byte) error) error {
	log := logger.WithContext(ctx)

	streamName := s.client.cfg.StreamName
	if streamName == "" {
		return fmt.Errorf("stream name is empty")
	}

	durableName := "sub_" + strings.ReplaceAll(strings.ReplaceAll(subject, ".", "_"), "*", "all")
	durableName = strings.ReplaceAll(durableName, ">", "all")

	cfg := jetstream.ConsumerConfig{
		Durable:       durableName,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
	}

	cons, err := s.js.CreateOrUpdateConsumer(ctx, streamName, cfg)
	if err != nil {
		return fmt.Errorf("creating or updating consumer: %w", err)
	}

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		err := handler(ctx, msg.Subject(), msg.Data())
		if err != nil {
			log.Error("handler failed, nacking message", zap.Error(err), zap.String("subject", msg.Subject()))
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	if err != nil {
		return fmt.Errorf("starting consume: %w", err)
	}

	// Stop consuming when context is canceled
	go func() {
		<-ctx.Done()
		cc.Stop()
	}()

	return nil
}
