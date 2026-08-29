package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"lifehub/services/api/internal/auth"
	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/smartcapture"

	"github.com/go-chi/chi/v5"
)

type Store interface {
	Ping(ctx context.Context) error
	Ready(ctx context.Context) error
	GetProfile(ctx context.Context, userID string) (domain.Profile, error)
	UpdateProfileTimezone(ctx context.Context, userID, timezone string) (domain.Profile, error)
	DeleteUserData(ctx context.Context, userID string) error
	CreateTask(ctx context.Context, params domain.CreateTaskParams) (domain.Task, error)
	CreateRecurringTask(ctx context.Context, params domain.CreateRecurringTaskParams) (domain.Task, error)
	GetTask(ctx context.Context, userID, taskID string) (domain.Task, error)
	UpdateTask(ctx context.Context, params domain.UpdateTaskParams) (domain.Task, error)
	DeleteTask(ctx context.Context, userID, taskID string) error
	CompleteTask(ctx context.Context, userID, taskID string) (domain.Task, error)
	UncompleteTask(ctx context.Context, userID, taskID string) (domain.Task, error)
	ListTodayTasks(ctx context.Context, userID string, start, end, horizonEnd time.Time) ([]domain.Task, error)
	ListAgendaTasks(ctx context.Context, userID string, start, end time.Time) ([]domain.Task, error)
	CreateEvent(ctx context.Context, params domain.CreateEventParams) (domain.Event, error)
	CreateRecurringEvent(ctx context.Context, params domain.CreateRecurringEventParams) (domain.Event, error)
	ListEvents(ctx context.Context, userID, from, to string, start, end time.Time) ([]domain.Event, error)
	GetEvent(ctx context.Context, userID, eventID string) (domain.Event, error)
	UpdateEvent(ctx context.Context, params domain.UpdateEventParams) (domain.Event, error)
	DeleteEvent(ctx context.Context, userID, eventID string) error
	ListTodayEvents(ctx context.Context, userID, date, horizonDate string, start, horizonEnd time.Time) ([]domain.Event, error)
	CreateBill(ctx context.Context, params domain.CreateBillParams) (domain.Bill, error)
	CreateRecurringBill(ctx context.Context, params domain.CreateRecurringBillParams) (domain.Bill, error)
	ListBills(ctx context.Context, userID, state string, limit int, afterAt *time.Time, afterID *string) ([]domain.Bill, error)
	GetBill(ctx context.Context, userID, billID string) (domain.Bill, error)
	UpdateBill(ctx context.Context, params domain.UpdateBillParams) (domain.Bill, error)
	DeleteBill(ctx context.Context, userID, billID string) error
	MarkBillPaid(ctx context.Context, userID, billID string) (domain.Bill, error)
	MarkBillUnpaid(ctx context.Context, userID, billID string) (domain.Bill, error)
	ListTodayBills(ctx context.Context, userID string, start, end, horizonEnd time.Time) ([]domain.Bill, error)
	ListAgendaBills(ctx context.Context, userID string, start, end time.Time) ([]domain.Bill, error)
	CreateDocument(ctx context.Context, params domain.CreateDocumentParams) (domain.Document, error)
	ListDocuments(ctx context.Context, userID string) ([]domain.Document, error)
	GetDocument(ctx context.Context, userID, documentID string) (domain.Document, error)
	UpdateDocument(ctx context.Context, params domain.UpdateDocumentParams) (domain.Document, error)
	DeleteDocument(ctx context.Context, userID, documentID string) error
	ListTodayDocuments(ctx context.Context, userID, horizonDate string) ([]domain.Document, error)
	ListAgendaDocuments(ctx context.Context, userID, from, to string) ([]domain.Document, error)
	CreateReminder(ctx context.Context, params domain.CreateReminderParams) (domain.Reminder, error)
	GetReminder(ctx context.Context, userID, reminderID string) (domain.Reminder, error)
	ListReminders(ctx context.Context, userID, sourceKind, sourceID string) ([]domain.Reminder, error)
	UpdateReminder(ctx context.Context, params domain.UpdateReminderParams) (domain.Reminder, error)
	DeleteReminder(ctx context.Context, userID, reminderID string) error
	ListNotifications(ctx context.Context, userID string, limit int, afterAt *time.Time, afterID *string) ([]domain.Notification, int, error)
	NotificationUnreadCount(ctx context.Context, userID string) (int, error)
	MarkNotificationRead(ctx context.Context, userID, notificationID string) (domain.Notification, int, error)
	MarkAllNotificationsRead(ctx context.Context, userID string) (int, error)
	MaterializeRecurrences(ctx context.Context, userID string, fromOn, throughOn time.Time) error
	ListRecurrenceSeries(ctx context.Context, userID string) ([]domain.RecurrenceSeries, error)
	GetRecurrenceSeries(ctx context.Context, userID, seriesID string) (domain.RecurrenceSeries, error)
	UpdateRecurrenceSeries(ctx context.Context, params domain.UpdateRecurrenceSeriesParams) (domain.RecurrenceSeries, error)
	StopRecurrenceSeries(ctx context.Context, userID, seriesID string, fromOn time.Time) error
}

type DevTokenIssuer interface {
	Issue(email string) (string, error)
}

type Options struct {
	Store                Store
	Verifier             auth.Verifier
	DevIssuer            DevTokenIssuer
	SmartCaptureProvider smartcapture.Provider
	WebOrigins           []string
	Logger               *slog.Logger
	Clock                func() time.Time
	Production           bool
}

type API struct {
	store                Store
	verifier             auth.Verifier
	devIssuer            DevTokenIssuer
	smartCaptureProvider smartcapture.Provider
	smartCaptureLimiter  *fixedWindowLimiter
	logger               *slog.Logger
	clock                func() time.Time
}

func New(options Options) http.Handler {
	api := &API{
		store:                options.Store,
		verifier:             options.Verifier,
		devIssuer:            options.DevIssuer,
		smartCaptureProvider: options.SmartCaptureProvider,
		logger:               options.Logger,
		clock:                options.Clock,
	}
	if api.logger == nil {
		api.logger = slog.Default()
	}
	if api.clock == nil {
		api.clock = time.Now
	}
	if api.smartCaptureProvider == nil {
		api.smartCaptureProvider = smartcapture.RuleProvider{}
	}
	api.smartCaptureLimiter = newFixedWindowLimiter(20, time.Minute, api.clock)

	router := chi.NewRouter()
	router.Use(securityHeaders(options.Production))
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
		private.Delete("/api/v1/profile/data", api.deleteProfileData)
		private.Post("/api/v1/smart-capture/parse", api.parseSmartCapture)
		private.Post("/api/v1/tasks", api.createTask)
		private.Get("/api/v1/tasks/{taskID}", api.getTask)
		private.Patch("/api/v1/tasks/{taskID}", api.patchTask)
		private.Delete("/api/v1/tasks/{taskID}", api.deleteTask)
		private.Post("/api/v1/tasks/{taskID}/complete", api.completeTask)
		private.Post("/api/v1/tasks/{taskID}/uncomplete", api.uncompleteTask)
		private.Post("/api/v1/events", api.createEvent)
		private.Get("/api/v1/events", api.listEvents)
		private.Get("/api/v1/events/{eventID}", api.getEvent)
		private.Patch("/api/v1/events/{eventID}", api.patchEvent)
		private.Delete("/api/v1/events/{eventID}", api.deleteEvent)
		private.Post("/api/v1/bills", api.createBill)
		private.Get("/api/v1/bills", api.listBills)
		private.Get("/api/v1/bills/{billID}", api.getBill)
		private.Patch("/api/v1/bills/{billID}", api.patchBill)
		private.Delete("/api/v1/bills/{billID}", api.deleteBill)
		private.Post("/api/v1/bills/{billID}/mark-paid", api.markBillPaid)
		private.Post("/api/v1/bills/{billID}/mark-unpaid", api.markBillUnpaid)
		private.Get("/api/v1/documents", api.listDocuments)
		private.Post("/api/v1/documents", api.createDocument)
		private.Get("/api/v1/documents/{documentID}", api.getDocument)
		private.Patch("/api/v1/documents/{documentID}", api.patchDocument)
		private.Delete("/api/v1/documents/{documentID}", api.deleteDocument)
		private.Get("/api/v1/today", api.getToday)
		private.Get("/api/v1/agenda", api.getAgenda)
		private.Get("/api/v1/reminders", api.listReminders)
		private.Post("/api/v1/reminders", api.createReminder)
		private.Get("/api/v1/reminders/{reminderID}", api.getReminder)
		private.Patch("/api/v1/reminders/{reminderID}", api.patchReminder)
		private.Delete("/api/v1/reminders/{reminderID}", api.deleteReminder)
		private.Get("/api/v1/notifications", api.listNotifications)
		private.Get("/api/v1/notifications/unread-count", api.notificationUnreadCount)
		private.Post("/api/v1/notifications/mark-all-read", api.markAllNotificationsRead)
		private.Post("/api/v1/notifications/{notificationID}/mark-read", api.markNotificationRead)
		private.Get("/api/v1/recurrence-series", api.listRecurrenceSeries)
		private.Get("/api/v1/recurrence-series/{seriesID}", api.getRecurrenceSeries)
		private.Patch("/api/v1/recurrence-series/{seriesID}", api.patchRecurrenceSeries)
		private.Delete("/api/v1/recurrence-series/{seriesID}", api.stopRecurrenceSeries)
	})

	router.NotFound(func(response http.ResponseWriter, request *http.Request) {
		writeError(response, request, http.StatusNotFound, "NOT_FOUND", "Endpoint tidak ditemukan.", nil)
	})
	router.MethodNotAllowed(func(response http.ResponseWriter, request *http.Request) {
		writeError(response, request, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Metode tidak diizinkan.", nil)
	})
	return router
}
