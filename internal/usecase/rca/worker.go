// Package rca provides the background NATS worker for asynchronous Root Cause Analysis.
package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	infraNats "github.com/datdt/k8sselfhost/internal/infrastructure/nats"
	"github.com/datdt/k8sselfhost/pkg/logger"
)

// WSBroadcaster defines the interface for real-time WebSocket notifications.
type WSBroadcaster interface {
	Broadcast(msgType string, data interface{})
}

// Worker consumes analysis requests from NATS JetStream and runs the RCA pipeline.
type Worker struct {
	natsClient *infraNats.Client
	pipeline   *Pipeline
	incRepo    incident.Repository
	wsHub      WSBroadcaster
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewWorker creates a new RCA worker.
func NewWorker(natsClient *infraNats.Client, pipeline *Pipeline, incRepo incident.Repository, wsHub WSBroadcaster) *Worker {
	return &Worker{
		natsClient: natsClient,
		pipeline:   pipeline,
		incRepo:    incRepo,
		wsHub:      wsHub,
	}
}

// Start begins the JetStream consumer loop in a background goroutine.
func (w *Worker) Start(ctx context.Context) error {
	ctx, w.cancel = context.WithCancel(ctx)
	log := logger.WithContext(ctx)

	js := w.natsClient.JetStream()
	
	// Create or update durable consumer for analysis tasks
	cons, err := js.CreateOrUpdateConsumer(ctx, "INCIDENTS", jetstream.ConsumerConfig{
		Durable:       "rca-worker",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "incidents.analyze",
		MaxDeliver:    3,
	})
	if err != nil {
		return fmt.Errorf("creating JetStream consumer: %w", err)
	}

	log.Info("RCA worker started, listening on incidents.analyze")

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		consumeCtx, err := cons.Consume(func(msg jetstream.Msg) {
			w.wg.Add(1)
			go func() {
				defer w.wg.Done()
				msgCtx, cancelMsg := context.WithTimeout(ctx, 5*time.Minute)
				defer cancelMsg()
				w.processMessage(msgCtx, msg)
			}()
		}, jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
			log.Error("error during message consumption", zap.Error(err))
		}))

		if err != nil {
			log.Error("failed to start consume loop", zap.Error(err))
			return
		}

		<-ctx.Done()
		consumeCtx.Stop()
		log.Info("RCA worker consumer stopped")
	}()

	return nil
}

// Stop stops the consumer and waits for active tasks to complete.
func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
	logger.Get().Info("RCA worker stopped gracefully")
}

func (w *Worker) processMessage(ctx context.Context, msg jetstream.Msg) {
	log := logger.WithContext(ctx)

	var task struct {
		IncidentID string `json:"incident_id"`
	}
	if err := json.Unmarshal(msg.Data(), &task); err != nil {
		log.Error("failed to unmarshal analysis task", zap.Error(err))
		if termErr := msg.Term(); termErr != nil {
			log.Error("failed to term msg", zap.Error(termErr))
		}
		return
	}

	log.Info("received analysis task", zap.String("incident_id", task.IncidentID))

	inc, err := w.incRepo.GetByID(ctx, task.IncidentID)
	if err != nil {
		log.Error("failed to fetch incident for analysis", zap.String("incident_id", task.IncidentID), zap.Error(err))
		if termErr := msg.Term(); termErr != nil {
			log.Error("failed to term msg", zap.Error(termErr))
		}
		return
	}

	// Notify frontend about starting analysis
	w.wsHub.Broadcast("log", fmt.Sprintf("rca: starting analysis for incident %s (%s)", inc.ID, inc.Type))
	w.wsHub.Broadcast("incident", inc)

	// Run RCA pipeline
	rpt, err := w.pipeline.Analyze(ctx, inc)
	if err != nil {
		log.Error("RCA analysis failed", zap.String("incident_id", inc.ID), zap.Error(err))

		// Mark incident as failed in DB and notify UI using a fresh detached context
		dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if markErr := inc.MarkFailed(); markErr == nil {
			_ = w.incRepo.Update(dbCtx, inc)
		}
		dbCancel()

		w.wsHub.Broadcast("incident", inc)
		w.wsHub.Broadcast("log", fmt.Sprintf("rca: analysis failed for incident %s: %v", inc.ID, err))

		// Handle retry backoff
		numDelivered := uint64(1)
		if meta, err := msg.Metadata(); err == nil {
			numDelivered = meta.NumDelivered
		}
		backoff := time.Duration(1<<numDelivered) * time.Second
		if nakErr := msg.NakWithDelay(backoff); nakErr != nil {
			log.Error("failed to nak msg", zap.Error(nakErr))
		}
		return
	}

	// Persisted and analyzed successfully! Update status on frontend
	w.wsHub.Broadcast("incident", inc)
	w.wsHub.Broadcast("log", fmt.Sprintf("rca: analysis complete for incident %s. Root cause: %s", inc.ID, rpt.RootCause))
	if ackErr := msg.Ack(); ackErr != nil {
		log.Error("failed to ack msg", zap.Error(ackErr))
	}
}
