package middleware

import (
	"net/http"
	"runtime/debug"

	chimw "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// JSONRecoverer recovers from panics and converts them into structured 500 JSON responses.
// http.ErrAbortHandler panics are re-panicked to maintain Go's net/http server abort contract.
func JSONRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := newResponseWriter(w)

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

			if !rw.wroteHeader && !rw.hijacked {
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
