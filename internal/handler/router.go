package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.uber.org/zap"
)

// serveOpenAPISpec отдаёт OpenAPI спецификацию
func serveOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	// Спецификация находится в корне проекта
	http.ServeFile(w, r, "openapi.yml")
}

// Router создаёт и настраивает HTTP роутер
func Router(
	teamHandler *TeamHandler,
	userHandler *UserHandler,
	prHandler *PullRequestHandler,
	logger *zap.Logger,
) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(loggerMiddleware(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// OpenAPI спецификация
	r.Get("/openapi.yml", serveOpenAPISpec)

	// Swagger UI
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/openapi.yml"),
	)) // Team endpoints
	r.Post("/team/add", teamHandler.CreateTeam)
	r.Get("/team/get", teamHandler.GetTeam)

	// User endpoints
	r.Post("/users/setIsActive", userHandler.SetIsActive)
	r.Get("/users/getReview", userHandler.GetReview)

	// Pull Request endpoints
	r.Post("/pullRequest/create", prHandler.CreatePullRequest)
	r.Post("/pullRequest/merge", prHandler.MergePullRequest)
	r.Post("/pullRequest/reassign", prHandler.ReassignReviewer)

	return r
}

// loggerMiddleware добавляет структурированное логирование HTTP запросов
func loggerMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				logger.Info("http request",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.String("remote_addr", r.RemoteAddr),
					zap.String("user_agent", r.UserAgent()),
					zap.Int("status", ww.Status()),
					zap.Int("bytes", ww.BytesWritten()),
					zap.Duration("duration", time.Since(start)),
					zap.String("request_id", middleware.GetReqID(r.Context())),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
