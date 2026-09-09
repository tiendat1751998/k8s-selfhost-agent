package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

func BenchmarkGenerateAccessToken(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		token, err := middleware.GenerateAccessToken("user-123456", "admin", "tenant-uuid-789")
		if err != nil {
			b.Fatal(err)
		}
		_ = token
	}
}

func BenchmarkValidateJWT(b *testing.B) {
	token, err := middleware.GenerateAccessToken("user-123456", "admin", "tenant-uuid-789")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		claims, err := middleware.ValidateJWT(token)
		if err != nil {
			b.Fatal(err)
		}
		_ = claims
	}
}

func BenchmarkJWTAuthMiddleware(b *testing.B) {
	token, err := middleware.GenerateAccessToken("user-123456", "admin", "tenant-uuid-789")
	if err != nil {
		b.Fatal(err)
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.JWTAuthMiddleware(nextHandler)
	req := httptest.NewRequest("GET", "/api/v1/incidents", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkRouter_Dispatch_Public(b *testing.B) {
	healthHandler := health.NewHandler(5 * time.Second)
	router := NewRouterWithWS(healthHandler, nil, nil)
	req := httptest.NewRequest("GET", "/healthz", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
	}
}

func BenchmarkRouter_Dispatch_Protected_Offline(b *testing.B) {
	healthHandler := health.NewHandler(5 * time.Second)
	router := NewRouterWithWS(healthHandler, nil, nil)
	token, err := middleware.GenerateAccessToken("user-123456", "platform_admin", "tenant-uuid-789")
	if err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/v1/explorer", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
	}
}

func BenchmarkFilterLogStream_1000Lines(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		if i%10 == 0 {
			sb.WriteString(fmt.Sprintf("2026-08-28T00:00:%02dZ [ERROR] Failed to connect to upstream database at 10.0.0.%d\n", i%60, i%256))
		} else if i%5 == 0 {
			sb.WriteString(fmt.Sprintf("2026-08-28T00:00:%02dZ [WARN] Slow response detected from service backend: %d ms\n", i%60, i*12))
		} else {
			sb.WriteString(fmt.Sprintf("2026-08-28T00:00:%02dZ [INFO] Processing request ID %d successfully\n", i%60, i))
		}
	}
	rawLogs := sb.String()

	b.Run("Filter_All", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = FilterLogStream(rawLogs, "", "all", 100)
		}
	})

	b.Run("Filter_Errors_Only", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = FilterLogStream(rawLogs, "", "error", 100)
		}
	})

	b.Run("Filter_Search_Query", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = FilterLogStream(rawLogs, "database", "all", 100)
		}
	})
}
