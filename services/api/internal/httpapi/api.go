package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"lifehub/services/api/internal/auth"
	"lifehub/services/api/internal/domain"

	"github.com/go-chi/chi/v5"
)

type Store interface {
	Ping(ctx context.Context) error
	GetProfile(ctx context.Context, userID string) (domain.Profile, error)
	UpdateProfileTimezone(ctx context.Context, userID, timezone string) (domain.Profile, error)
	CreateTask(ctx context.Context, params domain.CreateTaskParams) (domain.Task, error)
	CompleteTask(ctx context.Context, userID, taskID string) (domain.Task, error)
	ListTodayTasks(ctx context.Context, userID string, start, end time.Time) ([]domain.Task, error)
}

type DevTokenIssuer interface {
	Issue(email string) (string, error)
}

type Options struct {
	Store      Store
	Verifier   auth.Verifier
	DevIssuer  DevTokenIssuer
	WebOrigins []string
	Logger     *slog.Logger
	Clock      func() time.Time
}

type API struct {
	store     Store
	verifier  auth.Verifier
	devIssuer DevTokenIssuer
	logger    *slog.Logger
	clock     func() time.Time
}

func New(options Options) http.Handler {
	api := &API{
		store:     options.Store,
		verifier:  options.Verifier,
		devIssuer: options.DevIssuer,
		logger:    options.Logger,
		clock:     options.Clock,
	}
	if api.logger == nil {
		api.logger = slog.Default()
	}
	if api.clock == nil {
		api.clock = time.Now
	}

	router := chi.NewRouter()
	router.Use(api.requestID)
	router.Use(api.recoverPanics)
	router.Use(api.logRequests)
	router.Use(cors(options.WebOrigins))
	router.Use(requestTimeout(10 * time.Second))

	router.Get("/healthz", api.health)
	router.Get("/readyz", api.ready)
	if api.devIssuer != nil {
		router.Post("/api/v1/auth/dev-session", api.devSession)
	}

	router.Group(func(private chi.Router) {
		private.Use(api.authenticate)
		private.Get("/api/v1/profile", api.getProfile)
		private.Patch("/api/v1/profile", api.patchProfile)
		private.Post("/api/v1/tasks", api.createTask)
		private.Post("/api/v1/tasks/{taskID}/complete", api.completeTask)
		private.Get("/api/v1/today", api.getToday)
	})

	router.NotFound(func(response http.ResponseWriter, request *http.Request) {
		writeError(response, request, http.StatusNotFound, "NOT_FOUND", "Endpoint tidak ditemukan.", nil)
	})
	router.MethodNotAllowed(func(response http.ResponseWriter, request *http.Request) {
		writeError(response, request, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Metode tidak diizinkan.", nil)
	})
	return router
}
