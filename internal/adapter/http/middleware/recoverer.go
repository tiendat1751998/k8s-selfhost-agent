package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"

	chimw "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type recovererWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (rw *recovererWriter) WriteHeader(code int) {
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *recovererWriter) Write(b []byte) (int, error) {
	rw.wroteHeader = true
	return rw.ResponseWriter.Write(b)
}

func (rw *recovererWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func (rw *recovererWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *recovererWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack not supported")
	}
	return hijacker.Hijack()
}

// JSONRecoverer recovers from panics and converts them into structured 500 JSON responses.
// http.ErrAbortHandler panics are re-panicked to maintain Go's net/http server abort contract.
func JSONRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &recovererWriter{ResponseWriter: w}

		defer func() {
			rvr := recover()
			if rvr == nil {
				return
			}

			if rvr == http.ErrAbortHandler {
				panic(rvr)
			}

			reqID := chimw.GetReqID(r.Context())
			stack := string(debug.Stack())

			zap.L().Error("panic recovered",
				zap.Any("error", rvr),
				zap.String("stack", stack),
				zap.String("request_id", reqID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
			)

			if !rw.wroteHeader {
				rw.Header().Set("Content-Type", "application/json")
				rw.WriteHeader(http.StatusInternalServerError)
				if _, err := rw.Write([]byte(`{"error":"internal server error"}`)); err != nil {
					zap.L().Warn("failed to write 500 JSON response on panic recovery", zap.Error(err))
				}
			}
		}()

		next.ServeHTTP(rw, r)
	})
}
