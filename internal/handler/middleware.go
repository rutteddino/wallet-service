package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// statusRecorder запоминает HTTP-статус ответа для логирования.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware логирует каждый запрос: метод, путь, статус,
// длительность и request_id (его генерирует middleware.RequestID).
// ID также возвращается клиенту в заголовке X-Request-Id, чтобы
// по нему можно было найти запрос в логах.
// Регистрировать нужно ПОСЛЕ RequestID, но ДО Recoverer:
// тогда Recoverer отработает панику внутри, а лог всё равно запишется.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		if reqID := middleware.GetReqID(r.Context()); reqID != "" {
			rec.Header().Set("X-Request-Id", reqID)
		}

		next.ServeHTTP(rec, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}
