package metrics

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestEnrichServices_NoProportionalMetricCopying_ToBackendServices(t *testing.T) {
	collector := NewTPSCollector(nil, nil, zap.NewNop())

	services := []ServiceTPS{
		{
			ServiceName:   "edge-traefik",
			RxBytesPerSec: 100000,
			TxBytesPerSec: 200000,
		},
		{
			ServiceName:   "payment-worker",
			RxBytesPerSec: 50000,
			TxBytesPerSec: 80000,
		},
		{
			ServiceName:   "report-generator",
			RxBytesPerSec: 20000,
			TxBytesPerSec: 30000,
		},
	}

	httpTPS := HTTPTPS{
		RequestsPerSec: 150.0,
		ErrorRate:      8.5,
		AvgLatencyMs:   45.2,
		MaxLatencyMs:   120.0,
	}
	dbTPS := DatabaseTPS{}
	msgTPS := MessagingTPS{}

	enriched := collector.enrichServicesWithLBStats(context.Background(), services, httpTPS, dbTPS, msgTPS)

	svcMap := make(map[string]ServiceTPS)
	for _, s := range enriched {
		svcMap[s.ServiceName] = s
	}

	// 1. Backend services lacking direct HTTP metrics must NOT inherit Traefik error rate, latency, or RPS
	backend, ok := svcMap["payment-worker"]
	if !ok {
		t.Fatal("expected payment-worker in enriched services")
	}
	if backend.RequestsPerSec != 0 {
		t.Errorf("expected RequestsPerSec = 0 for backend service without HTTP metrics, got %.2f", backend.RequestsPerSec)
	}
	if backend.ErrorRate != 0 {
		t.Errorf("expected ErrorRate = 0 (must not copy edge Traefik errors), got %.2f", backend.ErrorRate)
	}
	if backend.AvgLatencyMs != 0 {
		t.Errorf("expected AvgLatencyMs = 0 (must not copy edge Traefik latency), got %.2f", backend.AvgLatencyMs)
	}
	if backend.MaxLatencyMs != 0 {
		t.Errorf("expected MaxLatencyMs = 0, got %.2f", backend.MaxLatencyMs)
	}
	// Factual network throughput must be strictly preserved
	if backend.RxBytesPerSec != 50000 {
		t.Errorf("expected factual RxBytesPerSec = 50000, got %d", backend.RxBytesPerSec)
	}
	if backend.TxBytesPerSec != 80000 {
		t.Errorf("expected factual TxBytesPerSec = 80000, got %d", backend.TxBytesPerSec)
	}

	// 2. Report generator must also keep 0 HTTP metrics and factual network bytes
	reportSvc, ok := svcMap["report-generator"]
	if !ok {
		t.Fatal("expected report-generator in enriched services")
	}
	if reportSvc.RequestsPerSec != 0 || reportSvc.ErrorRate != 0 || reportSvc.AvgLatencyMs != 0 {
		t.Errorf("expected 0 HTTP metrics for report-generator, got rps=%.2f err=%.2f lat=%.2f",
			reportSvc.RequestsPerSec, reportSvc.ErrorRate, reportSvc.AvgLatencyMs)
	}
	if reportSvc.RxBytesPerSec != 20000 || reportSvc.TxBytesPerSec != 30000 {
		t.Errorf("expected factual network stats for report-generator, got rx=%d tx=%d",
			reportSvc.RxBytesPerSec, reportSvc.TxBytesPerSec)
	}
}
