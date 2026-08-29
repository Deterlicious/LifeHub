package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lifehub/services/api/internal/auth"
	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/store"
)

type fakeStore struct {
	profile               domain.Profile
	profileErr            error
	deletedProfileUserID  string
	created               domain.CreateTaskParams
	createdRecurringTask  domain.CreateRecurringTaskParams
	task                  domain.Task
	taskErr               error
	updatedTask           domain.UpdateTaskParams
	updateTaskResult      domain.Task
	deleteTaskID          string
	uncompleteTask        domain.Task
	completeTask          domain.Task
	completeErr           error
	todayTasks            []domain.Task
	todayDate             string
	todayHorizonDate      string
	todayStart            time.Time
	todayEnd              time.Time
	todayHorizonEnd       time.Time
	agendaTasks           []domain.Task
	agendaStart           time.Time
	agendaEnd             time.Time
	createdEvent          domain.CreateEventParams
	createdRecurringEvent domain.CreateRecurringEventParams
	todayEvents           []domain.Event
	events                []domain.Event
	event                 domain.Event
	eventErr              error
	updatedEvent          domain.UpdateEventParams
	updateEventResult     domain.Event
	deleteEventID         string
	listEventFrom         string
	listEventTo           string
	listEventStart        time.Time
	listEventEnd          time.Time
	createdBill           domain.CreateBillParams
	createdRecurringBill  domain.CreateRecurringBillParams
	markedBill            domain.Bill
	markBillErr           error
	markedBillID          string
	markUserID            string
	todayBills            []domain.Bill
	bills                 []domain.Bill
	bill                  domain.Bill
	billErr               error
	updatedBill           domain.UpdateBillParams
	updateBillResult      domain.Bill
	deleteBillID          string
	markedUnpaidBill      domain.Bill
	agendaBills           []domain.Bill
	listBillState         string
	listBillLimit         int
	listBillAfterAt       *time.Time
	listBillAfterID       *string
	createdDocument       domain.CreateDocumentParams
	documents             []domain.Document
	document              domain.Document
	documentErr           error
	updatedDocument       domain.UpdateDocumentParams
	updateResult          domain.Document
	updateErr             error
	deletedDocumentID     string
	deleteUserID          string
	deleteErr             error
	todayDocuments        []domain.Document
	agendaDocuments       []domain.Document
	createdReminder       domain.CreateReminderParams
	updatedReminder       domain.UpdateReminderParams
	reminder              domain.Reminder
	reminders             []domain.Reminder
	reminderErr           error
	deletedReminderID     string
	notifications         []domain.Notification
	notification          domain.Notification
	notificationErr       error
	unreadCount           int
	listNotifyLimit       int
	listNotifyAfterAt     *time.Time
	listNotifyAfterID     *string
	markedAllCount        int
	recurrenceSeries      []domain.RecurrenceSeries
	recurrenceItem        domain.RecurrenceSeries
	recurrenceErr         error
	materializeFrom       time.Time
	materializeThrough    time.Time
	stoppedSeriesID       string
	stoppedFrom           time.Time
	updatedRecurrence     domain.UpdateRecurrenceSeriesParams
	pingErr               error
}

func (fake *fakeStore) Ping(context.Context) error {
	return fake.pingErr
}

func (fake *fakeStore) Ready(context.Context) error {
	return fake.pingErr
}

func (fake *fakeStore) GetProfile(_ context.Context, userID string) (domain.Profile, error) {
	if fake.profileErr != nil {
		return domain.Profile{}, fake.profileErr
	}
	if fake.profile.UserID == "" {
		fake.profile = domain.Profile{UserID: userID, Locale: "id-ID", Currency: "IDR"}
	}
	return fake.profile, nil
}

func TestCanceledProfileReadIsNotReportedAsServerFailure(t *testing.T) {
	dev := mustDevAuth(t)
	handler := New(Options{
		Store:    &fakeStore{profileErr: fmt.Errorf("read profile: %w", context.Canceled)},
		Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodGet, "/api/v1/profile", "", token)
	if response.Code != statusClientClosedRequest {
		t.Fatalf("status = %d, want %d", response.Code, statusClientClosedRequest)
	}
}

func TestSecurityHeadersCoverPublicAndPrivateResponses(t *testing.T) {
	dev := mustDevAuth(t)
	handler := New(Options{Store: &fakeStore{}, Verifier: dev, DevIssuer: dev, Logger: discardLogger(), Production: true})
	for _, path := range []string{"/healthz", "/api/v1/profile"} {
		response := performJSON(t, handler, http.MethodGet, path, "", "")
		if response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("X-Frame-Options") != "DENY" {
			t.Fatalf("path=%s missing baseline security headers: %#v", path, response.Header())
		}
		if response.Header().Get("Strict-Transport-Security") == "" || response.Header().Get("Content-Security-Policy") == "" {
			t.Fatalf("path=%s missing production security policy", path)
		}
	}
}

func (fake *fakeStore) UpdateProfileTimezone(_ context.Context, userID, timezone string) (domain.Profile, error) {
	fake.profile = domain.Profile{UserID: userID, Timezone: &timezone, Locale: "id-ID", Currency: "IDR"}
	return fake.profile, nil
}

func (fake *fakeStore) DeleteUserData(_ context.Context, userID string) error {
	fake.deletedProfileUserID = userID
	return fake.deleteErr
}

func TestDeleteProfileDataRequiresExactConfirmationAndVerifiedIdentity(t *testing.T) {
	dev := mustDevAuth(t)
	fake := &fakeStore{}
	handler := New(Options{Store: fake, Verifier: dev, DevIssuer: dev, Logger: discardLogger()})
	token := issueThroughHTTP(t, handler)

	response := performJSON(t, handler, http.MethodDelete, "/api/v1/profile/data", `{"confirmation":"hapus"}`, token)
	if response.Code != http.StatusBadRequest || fake.deletedProfileUserID != "" {
		t.Fatalf("invalid confirmation status=%d deleted=%q body=%s", response.Code, fake.deletedProfileUserID, response.Body.String())
	}

	response = performJSON(t, handler, http.MethodDelete, "/api/v1/profile/data", `{"confirmation":"HAPUS DATA LIFEHUB","user_id":"00000000-0000-4000-8000-000000000001"}`, token)
	if response.Code != http.StatusBadRequest || fake.deletedProfileUserID != "" {
		t.Fatalf("browser identity status=%d deleted=%q body=%s", response.Code, fake.deletedProfileUserID, response.Body.String())
	}

	response = performJSON(t, handler, http.MethodDelete, "/api/v1/profile/data", `{"confirmation":"HAPUS DATA LIFEHUB"}`, token)
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", response.Code, response.Body.String())
	}
	if fake.deletedProfileUserID == "" || fake.deletedProfileUserID == "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("deleted user=%q", fake.deletedProfileUserID)
	}
}

func (fake *fakeStore) CreateTask(_ context.Context, params domain.CreateTaskParams) (domain.Task, error) {
	fake.created = params
	now := time.Date(2026, time.August, 19, 1, 0, 0, 0, time.UTC)
	return domain.Task{
		ID: params.ID, Title: params.Title, Notes: params.Notes, Priority: params.Priority,
		DueAt: params.DueAt, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (fake *fakeStore) CreateRecurringTask(_ context.Context, params domain.CreateRecurringTaskParams) (domain.Task, error) {
	fake.createdRecurringTask = params
	return fake.CreateTask(context.Background(), params.Task)
}

func (fake *fakeStore) CompleteTask(context.Context, string, string) (domain.Task, error) {
	return fake.completeTask, fake.completeErr
}

func (fake *fakeStore) GetTask(context.Context, string, string) (domain.Task, error) {
	return fake.task, fake.taskErr
}

func (fake *fakeStore) UpdateTask(_ context.Context, params domain.UpdateTaskParams) (domain.Task, error) {
	fake.updatedTask = params
	return fake.updateTaskResult, fake.taskErr
}

func (fake *fakeStore) DeleteTask(_ context.Context, _ string, taskID string) error {
	fake.deleteTaskID = taskID
	return fake.taskErr
}

func (fake *fakeStore) UncompleteTask(context.Context, string, string) (domain.Task, error) {
	return fake.uncompleteTask, fake.taskErr
}

func (fake *fakeStore) ListTodayTasks(_ context.Context, _ string, start, end, horizonEnd time.Time) ([]domain.Task, error) {
	fake.todayStart = start
	fake.todayEnd = end
	fake.todayHorizonEnd = horizonEnd
	return fake.todayTasks, nil
}

func (fake *fakeStore) ListAgendaTasks(_ context.Context, _ string, start, end time.Time) ([]domain.Task, error) {
	fake.agendaStart = start
	fake.agendaEnd = end
	return fake.agendaTasks, nil
}

func (fake *fakeStore) CreateEvent(_ context.Context, params domain.CreateEventParams) (domain.Event, error) {
	fake.createdEvent = params
	now := time.Date(2026, time.August, 19, 1, 0, 0, 0, time.UTC)
	event := domain.Event{
		ID: params.ID, Title: params.Title, Notes: params.Notes, Location: params.Location,
		AllDay: params.AllDay, Timezone: params.Timezone, StartsAt: params.StartsAt, EndsAt: params.EndsAt,
		CreatedAt: now, UpdatedAt: now,
	}
	if params.StartsOn != nil {
		value := params.StartsOn.Format(time.DateOnly)
		event.StartsOn = &value
	}
	if params.EndsOn != nil {
		value := params.EndsOn.Format(time.DateOnly)
		event.EndsOn = &value
	}
	return event, nil
}

func (fake *fakeStore) CreateRecurringEvent(_ context.Context, params domain.CreateRecurringEventParams) (domain.Event, error) {
	fake.createdRecurringEvent = params
	return fake.CreateEvent(context.Background(), params.Event)
}

func (fake *fakeStore) ListTodayEvents(_ context.Context, _ string, date, horizonDate string, _ time.Time, _ time.Time) ([]domain.Event, error) {
	fake.todayDate = date
	fake.todayHorizonDate = horizonDate
	return fake.todayEvents, nil
}

func (fake *fakeStore) ListEvents(_ context.Context, _ string, from, to string, start, end time.Time) ([]domain.Event, error) {
	fake.listEventFrom = from
	fake.listEventTo = to
	fake.listEventStart = start
	fake.listEventEnd = end
	return fake.events, fake.eventErr
}

func (fake *fakeStore) GetEvent(context.Context, string, string) (domain.Event, error) {
	return fake.event, fake.eventErr
}

func (fake *fakeStore) UpdateEvent(_ context.Context, params domain.UpdateEventParams) (domain.Event, error) {
	fake.updatedEvent = params
	return fake.updateEventResult, fake.eventErr
}

func (fake *fakeStore) DeleteEvent(_ context.Context, _ string, eventID string) error {
	fake.deleteEventID = eventID
	return fake.eventErr
}

func (fake *fakeStore) CreateBill(_ context.Context, params domain.CreateBillParams) (domain.Bill, error) {
	fake.createdBill = params
	now := time.Date(2026, time.August, 19, 1, 0, 0, 0, time.UTC)
	return domain.Bill{
		ID: params.ID, Title: params.Title, Notes: params.Notes, Amount: params.Amount,
		Currency: params.Currency, DueAt: params.DueAt, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (fake *fakeStore) CreateRecurringBill(_ context.Context, params domain.CreateRecurringBillParams) (domain.Bill, error) {
	fake.createdRecurringBill = params
	return fake.CreateBill(context.Background(), params.Bill)
}

func (fake *fakeStore) MarkBillPaid(_ context.Context, userID, billID string) (domain.Bill, error) {
	fake.markUserID = userID
	fake.markedBillID = billID
	return fake.markedBill, fake.markBillErr
}

func (fake *fakeStore) ListTodayBills(context.Context, string, time.Time, time.Time, time.Time) ([]domain.Bill, error) {
	return fake.todayBills, nil
}

func (fake *fakeStore) ListBills(_ context.Context, _ string, state string, limit int, afterAt *time.Time, afterID *string) ([]domain.Bill, error) {
	fake.listBillState = state
	fake.listBillLimit = limit
	fake.listBillAfterAt = afterAt
	fake.listBillAfterID = afterID
	return fake.bills, fake.billErr
}

func (fake *fakeStore) GetBill(context.Context, string, string) (domain.Bill, error) {
	return fake.bill, fake.billErr
}

func (fake *fakeStore) UpdateBill(_ context.Context, params domain.UpdateBillParams) (domain.Bill, error) {
	fake.updatedBill = params
	return fake.updateBillResult, fake.billErr
}

func (fake *fakeStore) DeleteBill(_ context.Context, _ string, billID string) error {
	fake.deleteBillID = billID
	return fake.billErr
}

func (fake *fakeStore) MarkBillUnpaid(context.Context, string, string) (domain.Bill, error) {
	return fake.markedUnpaidBill, fake.billErr
}

func (fake *fakeStore) ListAgendaBills(context.Context, string, time.Time, time.Time) ([]domain.Bill, error) {
	return fake.agendaBills, nil
}

func (fake *fakeStore) CreateDocument(_ context.Context, params domain.CreateDocumentParams) (domain.Document, error) {
	fake.createdDocument = params
	now := time.Date(2026, time.August, 19, 1, 0, 0, 0, time.UTC)
	return domain.Document{
		ID: params.ID, Name: params.Name, Category: params.Category, Notes: params.Notes,
		ExpiresOn: params.ExpiresOn.Format(time.DateOnly), CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (fake *fakeStore) ListDocuments(context.Context, string) ([]domain.Document, error) {
	return fake.documents, fake.documentErr
}

func (fake *fakeStore) GetDocument(context.Context, string, string) (domain.Document, error) {
	return fake.document, fake.documentErr
}

func (fake *fakeStore) UpdateDocument(_ context.Context, params domain.UpdateDocumentParams) (domain.Document, error) {
	fake.updatedDocument = params
	return fake.updateResult, fake.updateErr
}

func (fake *fakeStore) DeleteDocument(_ context.Context, userID, documentID string) error {
	fake.deleteUserID = userID
	fake.deletedDocumentID = documentID
	return fake.deleteErr
}

func (fake *fakeStore) ListTodayDocuments(context.Context, string, string) ([]domain.Document, error) {
	return fake.todayDocuments, nil
}

func (fake *fakeStore) ListAgendaDocuments(context.Context, string, string, string) ([]domain.Document, error) {
	return fake.agendaDocuments, nil
}

func (fake *fakeStore) CreateReminder(_ context.Context, params domain.CreateReminderParams) (domain.Reminder, error) {
	fake.createdReminder = params
	return fake.reminder, fake.reminderErr
}

func (fake *fakeStore) GetReminder(context.Context, string, string) (domain.Reminder, error) {
	return fake.reminder, fake.reminderErr
}

func (fake *fakeStore) ListReminders(context.Context, string, string, string) ([]domain.Reminder, error) {
	return fake.reminders, fake.reminderErr
}

func (fake *fakeStore) UpdateReminder(_ context.Context, params domain.UpdateReminderParams) (domain.Reminder, error) {
	fake.updatedReminder = params
	return fake.reminder, fake.reminderErr
}

func (fake *fakeStore) DeleteReminder(_ context.Context, _ string, reminderID string) error {
	fake.deletedReminderID = reminderID
	return fake.reminderErr
}

func (fake *fakeStore) ListNotifications(_ context.Context, _ string, limit int, afterAt *time.Time, afterID *string) ([]domain.Notification, int, error) {
	fake.listNotifyLimit = limit
	fake.listNotifyAfterAt = afterAt
	fake.listNotifyAfterID = afterID
	return fake.notifications, fake.unreadCount, fake.notificationErr
}

func (fake *fakeStore) NotificationUnreadCount(context.Context, string) (int, error) {
	return fake.unreadCount, fake.notificationErr
}

func (fake *fakeStore) MarkNotificationRead(context.Context, string, string) (domain.Notification, int, error) {
	return fake.notification, fake.unreadCount, fake.notificationErr
}

func (fake *fakeStore) MarkAllNotificationsRead(context.Context, string) (int, error) {
	return fake.markedAllCount, fake.notificationErr
}

func (fake *fakeStore) MaterializeRecurrences(_ context.Context, _ string, fromOn, throughOn time.Time) error {
	fake.materializeFrom = fromOn
	fake.materializeThrough = throughOn
	return fake.recurrenceErr
}

func (fake *fakeStore) ListRecurrenceSeries(context.Context, string) ([]domain.RecurrenceSeries, error) {
	return fake.recurrenceSeries, fake.recurrenceErr
}

func (fake *fakeStore) GetRecurrenceSeries(context.Context, string, string) (domain.RecurrenceSeries, error) {
	return fake.recurrenceItem, fake.recurrenceErr
}

func (fake *fakeStore) UpdateRecurrenceSeries(_ context.Context, params domain.UpdateRecurrenceSeriesParams) (domain.RecurrenceSeries, error) {
	fake.updatedRecurrence = params
	return fake.recurrenceItem, fake.recurrenceErr
}

func (fake *fakeStore) StopRecurrenceSeries(_ context.Context, _ string, seriesID string, fromOn time.Time) error {
	fake.stoppedSeriesID = seriesID
	fake.stoppedFrom = fromOn
	return fake.recurrenceErr
}

func TestDevSessionRouteIsOnlyRegisteredWithIssuer(t *testing.T) {
	dev := mustDevAuth(t)
	handler := New(Options{
		Store: &fakeStore{}, Verifier: dev, WebOrigins: []string{"http://localhost:3000"},
		Logger: discardLogger(),
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/dev-session", bytes.NewBufferString(`{"email":"user@example.com"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestProfileTimezoneTaskAndTodayFlow(t *testing.T) {
	dev := mustDevAuth(t)
	fake := &fakeStore{}
	now := time.Date(2026, time.August, 19, 3, 0, 0, 0, time.UTC)
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
		Clock: func() time.Time { return now },
	})
	token := issueThroughHTTP(t, handler)

	response := performJSON(t, handler, http.MethodPost, "/api/v1/tasks", `{"title":"Submit laporan"}`, token)
	if response.Code != http.StatusConflict {
		t.Fatalf("task before timezone status = %d, body=%s", response.Code, response.Body.String())
	}
	assertErrorCode(t, response.Body.Bytes(), "PROFILE_INCOMPLETE")

	response = performJSON(t, handler, http.MethodPatch, "/api/v1/profile", `{"timezone":"Asia/Jakarta"}`, token)
	if response.Code != http.StatusOK {
		t.Fatalf("timezone status = %d, body=%s", response.Code, response.Body.String())
	}

	response = performJSON(t, handler, http.MethodPost, "/api/v1/tasks",
		`{"title":" Submit laporan ","notes":" catatan ","priority":"high","due_local":"2026-08-19T14:00"}`, token)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", response.Code, response.Body.String())
	}
	wantDue := time.Date(2026, time.August, 19, 7, 0, 0, 0, time.UTC)
	if fake.created.DueAt == nil || !fake.created.DueAt.Equal(wantDue) {
		t.Fatalf("due_at = %v, want %s", fake.created.DueAt, wantDue)
	}
	if fake.created.Title != "Submit laporan" || fake.created.UserID == "" {
		t.Fatalf("unexpected server-owned task params: %#v", fake.created)
	}

	fake.todayTasks = []domain.Task{{
		ID: "00000000-0000-4000-8000-000000000001", Title: "Submit laporan",
		Priority: domain.PriorityHigh, DueAt: &wantDue, CreatedAt: now.Add(-time.Hour),
	}}
	response = performJSON(t, handler, http.MethodGet, "/api/v1/today", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("today status = %d, body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Date  string `json:"date"`
		Items []struct {
			Urgency string `json:"urgency"`
		} `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Date != "2026-08-19" || len(payload.Items) != 1 || payload.Items[0].Urgency != "today" {
		t.Fatalf("unexpected Today payload: %#v", payload)
	}
}

func TestCreateTaskRejectsAmbiguousLocalTime(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "America/New_York"
	fake := &fakeStore{profile: domain.Profile{
		UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone, Locale: "id-ID", Currency: "IDR",
	}}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodPost, "/api/v1/tasks",
		`{"title":"DST fold","due_local":"2026-11-01T01:30"}`, token)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if fake.created.ID != "" {
		t.Fatal("ambiguous task reached persistence")
	}
}

func TestCreateTimedEventAndExposeTypedTodayItem(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	fake := &fakeStore{profile: domain.Profile{
		UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone, Locale: "id-ID", Currency: "IDR",
	}}
	now := time.Date(2026, time.August, 19, 3, 0, 0, 0, time.UTC)
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
		Clock: func() time.Time { return now },
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodPost, "/api/v1/events",
		`{"title":" Meeting proyek ","notes":" agenda ","location":" Online ","all_day":false,"starts_local":"2026-08-19T14:00","ends_local":"2026-08-19T15:00"}`, token)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", response.Code, response.Body.String())
	}
	wantStart := time.Date(2026, time.August, 19, 7, 0, 0, 0, time.UTC)
	wantEnd := wantStart.Add(time.Hour)
	if fake.createdEvent.StartsAt == nil || !fake.createdEvent.StartsAt.Equal(wantStart) ||
		fake.createdEvent.EndsAt == nil || !fake.createdEvent.EndsAt.Equal(wantEnd) {
		t.Fatalf("event moments = %v..%v, want %s..%s", fake.createdEvent.StartsAt, fake.createdEvent.EndsAt, wantStart, wantEnd)
	}
	if fake.createdEvent.UserID == "" || fake.createdEvent.Timezone != timezone || fake.createdEvent.Title != "Meeting proyek" ||
		fake.createdEvent.Notes == nil || *fake.createdEvent.Notes != "agenda" ||
		fake.createdEvent.Location == nil || *fake.createdEvent.Location != "Online" {
		t.Fatalf("unexpected server-owned event params: %#v", fake.createdEvent)
	}

	fake.todayEvents = []domain.Event{{
		ID: fake.createdEvent.ID, Title: fake.createdEvent.Title, Notes: fake.createdEvent.Notes,
		Location: fake.createdEvent.Location, Timezone: timezone, StartsAt: &wantStart, EndsAt: &wantEnd,
		CreatedAt: now.Add(-time.Hour),
	}}
	response = performJSON(t, handler, http.MethodGet, "/api/v1/today", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("today status = %d, body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 1 || payload.Items[0]["kind"] != "event" || payload.Items[0]["starts_at"] == nil {
		t.Fatalf("unexpected Today event: %#v", payload.Items)
	}
	if allDay, exists := payload.Items[0]["all_day"]; !exists || allDay != false || payload.Items[0]["timezone"] != timezone {
		t.Fatalf("Today event lost type/timezone fields: %#v", payload.Items[0])
	}
	for _, taskOnlyField := range []string{"priority", "due_at", "completed_at"} {
		if _, exists := payload.Items[0][taskOnlyField]; exists {
			t.Fatalf("Today event leaked task-only field %q: %#v", taskOnlyField, payload.Items[0])
		}
	}
}

func TestCreateAllDayEventUsesInclusiveDates(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	fake := &fakeStore{profile: domain.Profile{
		UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone, Locale: "id-ID", Currency: "IDR",
	}}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodPost, "/api/v1/events",
		`{"title":"Libur","all_day":true,"starts_on":"2026-08-19","ends_on":"2026-08-19"}`, token)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", response.Code, response.Body.String())
	}
	if !fake.createdEvent.AllDay || fake.createdEvent.StartsOn == nil || fake.createdEvent.EndsOn == nil ||
		fake.createdEvent.StartsOn.Format(time.DateOnly) != "2026-08-19" ||
		fake.createdEvent.EndsOn.Format(time.DateOnly) != "2026-08-19" ||
		fake.createdEvent.StartsAt != nil || fake.createdEvent.EndsAt != nil {
		t.Fatalf("unexpected all-day params: %#v", fake.createdEvent)
	}
}

func TestCreateEventRejectsInvalidTimeShapesAndDST(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		body     string
		field    string
	}{
		{name: "all day is required", timezone: "Asia/Jakarta", body: `{"title":"Acara","starts_local":"2026-08-19T10:00"}`, field: "all_day"},
		{name: "browser user id is rejected", timezone: "Asia/Jakarta", body: `{"title":"Acara","all_day":false,"starts_local":"2026-08-19T10:00","user_id":"00000000-0000-4000-8000-000000000001"}`, field: "body"},
		{name: "timed start is required", timezone: "Asia/Jakarta", body: `{"title":"Acara","all_day":false}`, field: "starts_local"},
		{name: "timed forbids dates", timezone: "Asia/Jakarta", body: `{"title":"Acara","all_day":false,"starts_local":"2026-08-19T10:00","starts_on":"2026-08-19"}`, field: "starts_on"},
		{name: "all day forbids moments", timezone: "Asia/Jakarta", body: `{"title":"Acara","all_day":true,"starts_on":"2026-08-19","starts_local":"2026-08-19T10:00"}`, field: "starts_local"},
		{name: "timed end equals start", timezone: "Asia/Jakarta", body: `{"title":"Acara","all_day":false,"starts_local":"2026-08-19T10:00","ends_local":"2026-08-19T10:00"}`, field: "ends_local"},
		{name: "timed end before start", timezone: "Asia/Jakarta", body: `{"title":"Acara","all_day":false,"starts_local":"2026-08-19T10:00","ends_local":"2026-08-19T09:00"}`, field: "ends_local"},
		{name: "all day end before start", timezone: "Asia/Jakarta", body: `{"title":"Acara","all_day":true,"starts_on":"2026-08-20","ends_on":"2026-08-19"}`, field: "ends_on"},
		{name: "invalid all day date", timezone: "Asia/Jakarta", body: `{"title":"Acara","all_day":true,"starts_on":"2026-02-30"}`, field: "starts_on"},
		{name: "DST gap", timezone: "America/New_York", body: `{"title":"Acara","all_day":false,"starts_local":"2026-03-08T02:30"}`, field: "starts_local"},
		{name: "DST fold", timezone: "America/New_York", body: `{"title":"Acara","all_day":false,"starts_local":"2026-11-01T01:30"}`, field: "starts_local"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dev := mustDevAuth(t)
			fake := &fakeStore{profile: domain.Profile{
				UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &test.timezone, Locale: "id-ID", Currency: "IDR",
			}}
			handler := New(Options{
				Store: fake, Verifier: dev, DevIssuer: dev,
				WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
			})
			token := issueThroughHTTP(t, handler)
			response := performJSON(t, handler, http.MethodPost, "/api/v1/events", test.body, token)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
			}
			var payload errorEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if _, exists := payload.Error.Fields[test.field]; !exists {
				t.Fatalf("fields = %#v, want %q", payload.Error.Fields, test.field)
			}
			if fake.createdEvent.ID != "" {
				t.Fatalf("invalid event reached persistence: %#v", fake.createdEvent)
			}
		})
	}
}

func TestCreateEventRequiresCompletedTimezoneProfile(t *testing.T) {
	dev := mustDevAuth(t)
	fake := &fakeStore{}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodPost, "/api/v1/events",
		`{"title":"Acara","all_day":false,"starts_local":"2026-08-19T10:00"}`, token)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	assertErrorCode(t, response.Body.Bytes(), "PROFILE_INCOMPLETE")
}

func TestCreateBillDefaultsCurrencyConvertsDueAndFeedsToday(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	fake := &fakeStore{profile: domain.Profile{
		UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone, Locale: "id-ID", Currency: "IDR",
	}}
	now := time.Date(2026, time.August, 19, 3, 0, 0, 0, time.UTC)
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
		Clock: func() time.Time { return now },
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodPost, "/api/v1/bills",
		`{"title":" Internet rumah ","notes":" bayar lewat aplikasi ","amount":350000,"due_local":"2026-08-19T14:00"}`, token)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", response.Code, response.Body.String())
	}
	wantDue := time.Date(2026, time.August, 19, 7, 0, 0, 0, time.UTC)
	if fake.createdBill.DueAt != wantDue || fake.createdBill.Amount != 350000 || fake.createdBill.Currency != "IDR" ||
		fake.createdBill.Title != "Internet rumah" || fake.createdBill.Notes == nil || *fake.createdBill.Notes != "bayar lewat aplikasi" ||
		fake.createdBill.UserID == "" {
		t.Fatalf("unexpected server-owned bill params: %#v", fake.createdBill)
	}

	fake.todayBills = []domain.Bill{{
		ID: fake.createdBill.ID, Title: fake.createdBill.Title, Notes: fake.createdBill.Notes,
		Amount: fake.createdBill.Amount, Currency: fake.createdBill.Currency, DueAt: wantDue, CreatedAt: now.Add(-time.Hour),
	}}
	response = performJSON(t, handler, http.MethodGet, "/api/v1/today", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("today status = %d, body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 1 || payload.Items[0]["kind"] != "bill" || payload.Items[0]["amount"] != float64(350000) ||
		payload.Items[0]["currency"] != "IDR" || payload.Items[0]["due_at"] == nil {
		t.Fatalf("unexpected Today bill: %#v", payload.Items)
	}
	for _, unrelatedField := range []string{"priority", "completed_at", "starts_at", "starts_on", "all_day"} {
		if _, exists := payload.Items[0][unrelatedField]; exists {
			t.Fatalf("Today bill leaked unrelated field %q: %#v", unrelatedField, payload.Items[0])
		}
	}
}

func TestCreateBillRejectsInvalidMoneyCurrencyDueAndOwnershipInput(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		body     string
		field    string
	}{
		{name: "missing amount", timezone: "Asia/Jakarta", body: `{"title":"Internet","due_local":"2026-08-19T10:00"}`, field: "amount"},
		{name: "zero amount", timezone: "Asia/Jakarta", body: `{"title":"Internet","amount":0,"due_local":"2026-08-19T10:00"}`, field: "amount"},
		{name: "negative amount", timezone: "Asia/Jakarta", body: `{"title":"Internet","amount":-1,"due_local":"2026-08-19T10:00"}`, field: "amount"},
		{name: "unsafe amount", timezone: "Asia/Jakarta", body: `{"title":"Internet","amount":9007199254740992,"due_local":"2026-08-19T10:00"}`, field: "amount"},
		{name: "float amount", timezone: "Asia/Jakarta", body: `{"title":"Internet","amount":1.5,"due_local":"2026-08-19T10:00"}`, field: "body"},
		{name: "string amount", timezone: "Asia/Jakarta", body: `{"title":"Internet","amount":"350000","due_local":"2026-08-19T10:00"}`, field: "body"},
		{name: "lowercase currency", timezone: "Asia/Jakarta", body: `{"title":"Internet","amount":350000,"currency":"idr","due_local":"2026-08-19T10:00"}`, field: "currency"},
		{name: "non ASCII currency", timezone: "Asia/Jakarta", body: `{"title":"Internet","amount":350000,"currency":"İDR","due_local":"2026-08-19T10:00"}`, field: "currency"},
		{name: "missing due", timezone: "Asia/Jakarta", body: `{"title":"Internet","amount":350000}`, field: "due_local"},
		{name: "DST gap", timezone: "America/New_York", body: `{"title":"Internet","amount":350000,"due_local":"2026-03-08T02:30"}`, field: "due_local"},
		{name: "DST fold", timezone: "America/New_York", body: `{"title":"Internet","amount":350000,"due_local":"2026-11-01T01:30"}`, field: "due_local"},
		{name: "browser user id", timezone: "Asia/Jakarta", body: `{"title":"Internet","amount":350000,"due_local":"2026-08-19T10:00","user_id":"00000000-0000-4000-8000-000000000001"}`, field: "body"},
		{name: "title too long", timezone: "Asia/Jakarta", body: fmt.Sprintf(`{"title":%q,"amount":350000,"due_local":"2026-08-19T10:00"}`, strings.Repeat("a", 201)), field: "title"},
		{name: "notes too long", timezone: "Asia/Jakarta", body: fmt.Sprintf(`{"title":"Internet","notes":%q,"amount":350000,"due_local":"2026-08-19T10:00"}`, strings.Repeat("a", 5001)), field: "notes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dev := mustDevAuth(t)
			fake := &fakeStore{profile: domain.Profile{
				UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &test.timezone, Locale: "id-ID", Currency: "IDR",
			}}
			handler := New(Options{
				Store: fake, Verifier: dev, DevIssuer: dev,
				WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
			})
			token := issueThroughHTTP(t, handler)
			response := performJSON(t, handler, http.MethodPost, "/api/v1/bills", test.body, token)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
			}
			var payload errorEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if _, exists := payload.Error.Fields[test.field]; !exists {
				t.Fatalf("fields = %#v, want %q", payload.Error.Fields, test.field)
			}
			if fake.createdBill.ID != "" {
				t.Fatalf("invalid bill reached persistence: %#v", fake.createdBill)
			}
		})
	}
}

func TestCreateBillRequiresCompletedTimezoneProfile(t *testing.T) {
	dev := mustDevAuth(t)
	fake := &fakeStore{}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodPost, "/api/v1/bills",
		`{"title":"Internet","amount":350000,"due_local":"2026-08-19T10:00"}`, token)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	assertErrorCode(t, response.Body.Bytes(), "PROFILE_INCOMPLETE")
}

func TestMarkBillPaidUsesAuthenticatedOwnerAndMapsMissingToNotFound(t *testing.T) {
	dev := mustDevAuth(t)
	billID := "00000000-0000-4000-8000-000000000001"
	paidAt := time.Date(2026, time.August, 19, 4, 0, 0, 0, time.UTC)
	fake := &fakeStore{markedBill: domain.Bill{
		ID: billID, Title: "Internet", Amount: 350000, Currency: "IDR",
		DueAt: paidAt.Add(-time.Hour), PaidAt: &paidAt, CreatedAt: paidAt.Add(-24 * time.Hour), UpdatedAt: paidAt,
	}}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodPost, "/api/v1/bills/"+billID+"/mark-paid", `{}`, token)
	if response.Code != http.StatusOK || fake.markUserID == "" || fake.markedBillID != billID {
		t.Fatalf("status=%d owner=%q bill=%q body=%s", response.Code, fake.markUserID, fake.markedBillID, response.Body.String())
	}

	fake.markBillErr = store.ErrNotFound
	response = performJSON(t, handler, http.MethodPost, "/api/v1/bills/"+billID+"/mark-paid", `{}`, token)
	if response.Code != http.StatusNotFound {
		t.Fatalf("cross-owner status = %d, body=%s", response.Code, response.Body.String())
	}
	response = performJSON(t, handler, http.MethodPost, "/api/v1/bills/not-a-uuid/mark-paid", `{}`, token)
	if response.Code != http.StatusNotFound {
		t.Fatalf("invalid UUID status = %d, body=%s", response.Code, response.Body.String())
	}
}

func TestDocumentCRUDDerivesLocalExpiryAndClearsNotes(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	fake := &fakeStore{profile: domain.Profile{
		UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone, Locale: "id-ID", Currency: "IDR",
	}}
	now := time.Date(2026, time.August, 18, 18, 0, 0, 0, time.UTC)
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
		Clock: func() time.Time { return now },
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodPost, "/api/v1/documents",
		`{"name":" SIM ","category":"license","notes":" perpanjang ","expires_on":"2026-08-18"}`, token)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body=%s", response.Code, response.Body.String())
	}
	if fake.createdDocument.UserID == "" || fake.createdDocument.Name != "SIM" || fake.createdDocument.Category != "license" ||
		fake.createdDocument.Notes == nil || *fake.createdDocument.Notes != "perpanjang" ||
		fake.createdDocument.ExpiresOn.Format(time.DateOnly) != "2026-08-18" {
		t.Fatalf("unexpected document params: %#v", fake.createdDocument)
	}
	var created domain.Document
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != "expired" || created.DaysUntilExpiry != -1 {
		t.Fatalf("created derived expiry = %#v", created)
	}

	fake.documents = []domain.Document{
		{ID: "00000000-0000-4000-8000-000000000010", Name: "SIM", Category: "license", ExpiresOn: "2026-08-18"},
		{ID: "00000000-0000-4000-8000-000000000011", Name: "Paspor", Category: "identity", ExpiresOn: "2026-08-21"},
		{ID: "00000000-0000-4000-8000-000000000012", Name: "Polis", Category: "insurance", ExpiresOn: "2026-09-18"},
		{ID: "00000000-0000-4000-8000-000000000013", Name: "Ijazah", Category: "education", ExpiresOn: "2026-09-19"},
	}
	response = performJSON(t, handler, http.MethodGet, "/api/v1/documents", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", response.Code, response.Body.String())
	}
	var listed []domain.Document
	if err := json.Unmarshal(response.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 4 || listed[0].Status != "expired" || listed[0].DaysUntilExpiry != -1 ||
		listed[1].Status != "expiring" || listed[1].DaysUntilExpiry != 2 ||
		listed[2].Status != "expiring" || listed[2].DaysUntilExpiry != 30 ||
		listed[3].Status != "valid" || listed[3].DaysUntilExpiry != 31 {
		t.Fatalf("listed documents = %#v", listed)
	}

	documentID := "00000000-0000-4000-8000-000000000010"
	fake.document = fake.documents[0]
	response = performJSON(t, handler, http.MethodGet, "/api/v1/documents/"+documentID, "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("get status = %d, body=%s", response.Code, response.Body.String())
	}
	var gotDocument domain.Document
	if err := json.Unmarshal(response.Body.Bytes(), &gotDocument); err != nil {
		t.Fatal(err)
	}
	if gotDocument.Status != "expired" || gotDocument.DaysUntilExpiry != -1 {
		t.Fatalf("get derived expiry = %#v", gotDocument)
	}

	fake.updateResult = domain.Document{
		ID: documentID, Name: "SIM", Category: "license", ExpiresOn: "2026-08-18",
	}
	response = performJSON(t, handler, http.MethodPatch, "/api/v1/documents/"+documentID, `{"notes":null}`, token)
	if response.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body=%s", response.Code, response.Body.String())
	}
	if !fake.updatedDocument.NotesSet || fake.updatedDocument.Notes != nil || fake.updatedDocument.UserID == "" || fake.updatedDocument.ID != documentID {
		t.Fatalf("notes clear was not preserved: %#v", fake.updatedDocument)
	}
	var patched domain.Document
	if err := json.Unmarshal(response.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Status != "expired" || patched.DaysUntilExpiry != -1 {
		t.Fatalf("patch derived expiry = %#v", patched)
	}

	response = performJSON(t, handler, http.MethodDelete, "/api/v1/documents/"+documentID, "", token)
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 || fake.deleteUserID == "" || fake.deletedDocumentID != documentID {
		t.Fatalf("delete status=%d body=%q owner=%q id=%q", response.Code, response.Body.String(), fake.deleteUserID, fake.deletedDocumentID)
	}
}

func TestDocumentValidationAndTimezoneRequirements(t *testing.T) {
	timezone := "Asia/Jakarta"
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		field  string
	}{
		{name: "create missing name", method: http.MethodPost, path: "/api/v1/documents", body: `{"category":"license","expires_on":"2026-08-19"}`, field: "name"},
		{name: "create invalid category", method: http.MethodPost, path: "/api/v1/documents", body: `{"name":"SIM","category":"passport","expires_on":"2026-08-19"}`, field: "category"},
		{name: "create invalid date", method: http.MethodPost, path: "/api/v1/documents", body: `{"name":"SIM","category":"license","expires_on":"2026-02-30"}`, field: "expires_on"},
		{name: "create notes too long", method: http.MethodPost, path: "/api/v1/documents", body: fmt.Sprintf(`{"name":"SIM","category":"license","notes":%q,"expires_on":"2026-08-19"}`, strings.Repeat("a", 5001)), field: "notes"},
		{name: "create user id rejected", method: http.MethodPost, path: "/api/v1/documents", body: `{"name":"SIM","category":"license","expires_on":"2026-08-19","user_id":"00000000-0000-4000-8000-000000000001"}`, field: "body"},
		{name: "patch requires field", method: http.MethodPatch, path: "/api/v1/documents/00000000-0000-4000-8000-000000000010", body: `{}`, field: "body"},
		{name: "patch unknown field", method: http.MethodPatch, path: "/api/v1/documents/00000000-0000-4000-8000-000000000010", body: `{"file":"secret.pdf"}`, field: "body"},
		{name: "patch name null", method: http.MethodPatch, path: "/api/v1/documents/00000000-0000-4000-8000-000000000010", body: `{"name":null}`, field: "name"},
		{name: "patch category null", method: http.MethodPatch, path: "/api/v1/documents/00000000-0000-4000-8000-000000000010", body: `{"category":null}`, field: "category"},
		{name: "patch expiry null", method: http.MethodPatch, path: "/api/v1/documents/00000000-0000-4000-8000-000000000010", body: `{"expires_on":null}`, field: "expires_on"},
		{name: "patch invalid category", method: http.MethodPatch, path: "/api/v1/documents/00000000-0000-4000-8000-000000000010", body: `{"category":"passport"}`, field: "category"},
		{name: "patch invalid date", method: http.MethodPatch, path: "/api/v1/documents/00000000-0000-4000-8000-000000000010", body: `{"expires_on":"not-a-date"}`, field: "expires_on"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dev := mustDevAuth(t)
			fake := &fakeStore{profile: domain.Profile{
				UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone, Locale: "id-ID", Currency: "IDR",
			}}
			handler := New(Options{
				Store: fake, Verifier: dev, DevIssuer: dev,
				WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
			})
			token := issueThroughHTTP(t, handler)
			response := performJSON(t, handler, test.method, test.path, test.body, token)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
			}
			var payload errorEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if _, exists := payload.Error.Fields[test.field]; !exists {
				t.Fatalf("fields = %#v, want %q", payload.Error.Fields, test.field)
			}
			if fake.createdDocument.ID != "" || fake.updatedDocument.ID != "" {
				t.Fatal("invalid document reached persistence")
			}
		})
	}

	dev := mustDevAuth(t)
	fake := &fakeStore{}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodPost, "/api/v1/documents",
		`{"name":"SIM","category":"license","expires_on":"2026-08-19"}`, token)
	if response.Code != http.StatusConflict {
		t.Fatalf("create incomplete profile status = %d, body=%s", response.Code, response.Body.String())
	}
	response = performJSON(t, handler, http.MethodPatch, "/api/v1/documents/00000000-0000-4000-8000-000000000010",
		`{"notes":null}`, token)
	if response.Code != http.StatusConflict {
		t.Fatalf("patch incomplete profile status = %d, body=%s", response.Code, response.Body.String())
	}
}

func TestDocumentOwnershipMapsToNotFound(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	documentID := "00000000-0000-4000-8000-000000000010"
	fake := &fakeStore{
		profile:     domain.Profile{UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone, Locale: "id-ID", Currency: "IDR"},
		documentErr: store.ErrNotFound, updateErr: store.ErrNotFound, deleteErr: store.ErrNotFound,
	}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
	})
	token := issueThroughHTTP(t, handler)
	for _, request := range []struct {
		method string
		body   string
	}{
		{method: http.MethodGet},
		{method: http.MethodPatch, body: `{"name":"Paspor"}`},
		{method: http.MethodDelete},
	} {
		response := performJSON(t, handler, request.method, "/api/v1/documents/"+documentID, request.body, token)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, body=%s", request.method, response.Code, response.Body.String())
		}
	}
	response := performJSON(t, handler, http.MethodGet, "/api/v1/documents/not-a-uuid", "", token)
	if response.Code != http.StatusNotFound {
		t.Fatalf("invalid UUID status = %d", response.Code)
	}
}

func TestTodaySeparatesUpcomingDocumentsAndOmitsUnrelatedFields(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	fake := &fakeStore{profile: domain.Profile{
		UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone, Locale: "id-ID", Currency: "IDR",
	}}
	for index, expiresOn := range []string{"2026-08-18", "2026-08-19", "2026-08-20", "2026-09-18", "2026-09-19"} {
		fake.todayDocuments = append(fake.todayDocuments, domain.Document{
			ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", index+1), Name: fmt.Sprintf("Document %d", index+1),
			Category: "other", ExpiresOn: expiresOn,
		})
	}
	now := time.Date(2026, time.August, 19, 3, 0, 0, 0, time.UTC)
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
		Clock: func() time.Time { return now },
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodGet, "/api/v1/today", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Items               []map[string]any `json:"items"`
		Upcoming            []map[string]any `json:"upcoming"`
		UpcomingHorizonDays int              `json:"upcoming_horizon_days"`
		Summary             struct {
			Upcoming int `json:"upcoming"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 2 || len(payload.Upcoming) != 2 || payload.UpcomingHorizonDays != 30 || payload.Summary.Upcoming != 2 {
		t.Fatalf("unexpected document partition: %#v", payload)
	}
	if payload.Items[0]["bucket"] != "expired" || payload.Items[1]["bucket"] != "expires_today" ||
		payload.Upcoming[0]["bucket"] != "expiring_soon" {
		t.Fatalf("unexpected document buckets: items=%#v upcoming=%#v", payload.Items, payload.Upcoming)
	}
	for _, item := range append(payload.Items, payload.Upcoming...) {
		if item["kind"] != "document" || item["title"] == nil || item["category"] == nil || item["expires_on"] == nil || item["days_until_expiry"] == nil {
			t.Fatalf("document fields missing: %#v", item)
		}
		for _, unrelated := range []string{"priority", "due_at", "amount", "currency", "paid_at", "starts_at", "all_day"} {
			if _, exists := item[unrelated]; exists {
				t.Fatalf("document leaked %q: %#v", unrelated, item)
			}
		}
	}
}

func TestAgendaUsesProfileLocalBoundedRangeAndMixedSummary(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "America/New_York"
	taskAt := time.Date(2026, time.March, 8, 16, 0, 0, 0, time.UTC)
	billAt := taskAt.Add(time.Hour)
	allDay := "2026-03-08"
	fake := &fakeStore{
		profile:         domain.Profile{UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone, Locale: "id-ID", Currency: "IDR"},
		agendaTasks:     []domain.Task{{ID: "task", Title: "Task", Priority: domain.PriorityHigh, DueAt: &taskAt}},
		events:          []domain.Event{{ID: "event", Title: "Event", AllDay: true, Timezone: timezone, StartsOn: &allDay}},
		agendaBills:     []domain.Bill{{ID: "bill", Title: "Bill", Amount: 1000, Currency: "IDR", DueAt: billAt}},
		agendaDocuments: []domain.Document{{ID: "document", Name: "SIM", Category: "license", ExpiresOn: allDay}},
	}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
		Clock: func() time.Time { return time.Date(2026, time.March, 7, 17, 0, 0, 0, time.UTC) },
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodGet, "/api/v1/agenda?from=2026-03-07&to=2026-03-09", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	wantStart := time.Date(2026, time.March, 7, 5, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, time.March, 10, 4, 0, 0, 0, time.UTC)
	if !fake.agendaStart.Equal(wantStart) || !fake.agendaEnd.Equal(wantEnd) || fake.listEventFrom != "2026-03-07" || fake.listEventTo != "2026-03-09" {
		t.Fatalf("range start=%s end=%s event=%s..%s", fake.agendaStart, fake.agendaEnd, fake.listEventFrom, fake.listEventTo)
	}
	var payload struct {
		From     string           `json:"from"`
		To       string           `json:"to"`
		Timezone string           `json:"timezone"`
		Items    []map[string]any `json:"items"`
		Summary  struct {
			Total     int `json:"total"`
			Tasks     int `json:"tasks"`
			Events    int `json:"events"`
			Bills     int `json:"bills"`
			Documents int `json:"documents"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.From != "2026-03-07" || payload.To != "2026-03-09" || payload.Timezone != timezone || payload.Summary.Total != 4 ||
		payload.Summary.Tasks != 1 || payload.Summary.Events != 1 || payload.Summary.Bills != 1 || payload.Summary.Documents != 1 {
		t.Fatalf("payload = %#v", payload)
	}
	for _, item := range payload.Items {
		if item["display_on"] == nil || item["kind"] == nil || item["created_at"] == nil || item["updated_at"] == nil {
			t.Fatalf("agenda base DTO incomplete: %#v", item)
		}
	}
}

func TestAgendaAndTodayUseFirstRepresentableLocalDateBoundary(t *testing.T) {
	tests := []struct {
		name                string
		timezone            string
		date                string
		now                 time.Time
		wantStart           time.Time
		wantEnd             time.Time
		wantHorizonDate     string
		wantTodayHorizonEnd time.Time
	}{
		{
			name:                "Cairo midnight gap",
			timezone:            "Africa/Cairo",
			date:                "2026-04-24",
			now:                 time.Date(2026, time.April, 24, 9, 0, 0, 0, time.UTC),
			wantStart:           time.Date(2026, time.April, 23, 22, 0, 0, 0, time.UTC),
			wantEnd:             time.Date(2026, time.April, 24, 21, 0, 0, 0, time.UTC),
			wantHorizonDate:     "2026-05-24",
			wantTodayHorizonEnd: time.Date(2026, time.May, 24, 21, 0, 0, 0, time.UTC),
		},
		{
			name:                "Santiago midnight gap",
			timezone:            "America/Santiago",
			date:                "2026-09-06",
			now:                 time.Date(2026, time.September, 6, 15, 0, 0, 0, time.UTC),
			wantStart:           time.Date(2026, time.September, 6, 4, 0, 0, 0, time.UTC),
			wantEnd:             time.Date(2026, time.September, 7, 3, 0, 0, 0, time.UTC),
			wantHorizonDate:     "2026-10-06",
			wantTodayHorizonEnd: time.Date(2026, time.October, 7, 3, 0, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dev := mustDevAuth(t)
			fake := &fakeStore{profile: domain.Profile{
				UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &test.timezone,
			}}
			handler := New(Options{
				Store: fake, Verifier: dev, DevIssuer: dev,
				WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
				Clock: func() time.Time { return test.now },
			})
			token := issueThroughHTTP(t, handler)

			agendaResponse := performJSON(t, handler, http.MethodGet, "/api/v1/agenda?from="+test.date+"&to="+test.date, "", token)
			if agendaResponse.Code != http.StatusOK {
				t.Fatalf("agenda status = %d, body=%s", agendaResponse.Code, agendaResponse.Body.String())
			}
			if !fake.agendaStart.Equal(test.wantStart) || !fake.agendaEnd.Equal(test.wantEnd) ||
				!fake.listEventStart.Equal(test.wantStart) || !fake.listEventEnd.Equal(test.wantEnd) {
				t.Fatalf("agenda range tasks=[%s,%s) events=[%s,%s), want [%s,%s)",
					fake.agendaStart, fake.agendaEnd, fake.listEventStart, fake.listEventEnd, test.wantStart, test.wantEnd)
			}

			todayResponse := performJSON(t, handler, http.MethodGet, "/api/v1/today", "", token)
			if todayResponse.Code != http.StatusOK {
				t.Fatalf("today status = %d, body=%s", todayResponse.Code, todayResponse.Body.String())
			}
			if fake.todayDate != test.date || fake.todayHorizonDate != test.wantHorizonDate ||
				!fake.todayStart.Equal(test.wantStart) || !fake.todayEnd.Equal(test.wantEnd) || !fake.todayHorizonEnd.Equal(test.wantTodayHorizonEnd) {
				t.Fatalf("today date=%s horizon=%s range=[%s,%s) horizon_end=%s", fake.todayDate, fake.todayHorizonDate,
					fake.todayStart, fake.todayEnd, fake.todayHorizonEnd)
			}
		})
	}
}

func TestAgendaAndEventRangesRejectWhollySkippedProfileDate(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Pacific/Apia"
	fake := &fakeStore{profile: domain.Profile{
		UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone,
	}}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
	})
	token := issueThroughHTTP(t, handler)

	for _, test := range []struct {
		path  string
		field string
	}{
		{path: "/api/v1/agenda?from=2011-12-30&to=2011-12-30", field: "from"},
		{path: "/api/v1/events?from=2011-12-30&to=2011-12-30", field: "from"},
		{path: "/api/v1/agenda?from=2011-12-29&to=2011-12-30", field: "to"},
		{path: "/api/v1/events?from=2011-12-29&to=2011-12-30", field: "to"},
	} {
		response := performJSON(t, handler, http.MethodGet, test.path, "", token)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body=%s", test.path, response.Code, response.Body.String())
		}
		assertErrorCode(t, response.Body.Bytes(), "VALIDATION_ERROR")
		var payload errorEnvelope
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Error.Fields[test.field] == "" {
			t.Fatalf("%s missing skipped-date %s error: %#v", test.path, test.field, payload.Error.Fields)
		}
	}
	if !fake.agendaStart.IsZero() || !fake.listEventStart.IsZero() {
		t.Fatalf("store queried for skipped date: agenda=%s event=%s", fake.agendaStart, fake.listEventStart)
	}
}

func TestAgendaAndEventRangesRejectInvalidOrUnboundedQueries(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	fake := &fakeStore{profile: domain.Profile{UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone}}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
		Clock: func() time.Time { return time.Date(2026, time.August, 19, 3, 0, 0, 0, time.UTC) },
	})
	token := issueThroughHTTP(t, handler)
	for _, path := range []string{
		"/api/v1/agenda?from=2026-08-20",
		"/api/v1/agenda?from=2026-08-01&to=2026-09-01",
		"/api/v1/agenda?from=2026-02-30&to=2026-03-01",
		"/api/v1/agenda?from=2026-08-20&to=2026-08-21&extra=true",
		"/api/v1/events?from=2026-08-20&from=2026-08-21&to=2026-08-22",
	} {
		response := performJSON(t, handler, http.MethodGet, path, "", token)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, body=%s", path, response.Code, response.Body.String())
		}
		assertErrorCode(t, response.Body.Bytes(), "VALIDATION_ERROR")
	}
}

func TestTodayUpcomingIncludesAllUnresolvedKindsWithoutInflatingOpen(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	start := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	eventAt := start.AddDate(0, 0, 1).Add(8 * time.Hour)
	billAt := eventAt.Add(time.Hour)
	taskAt := eventAt.Add(2 * time.Hour)
	tomorrow := "2026-08-20"
	fake := &fakeStore{
		profile:        domain.Profile{UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone},
		todayTasks:     []domain.Task{{ID: "task", Title: "Task", Priority: domain.PriorityHigh, DueAt: &taskAt}},
		todayEvents:    []domain.Event{{ID: "event", Title: "Event", Timezone: timezone, StartsAt: &eventAt}},
		todayBills:     []domain.Bill{{ID: "bill", Title: "Bill", Amount: 1000, Currency: "IDR", DueAt: billAt}},
		todayDocuments: []domain.Document{{ID: "document", Name: "SIM", Category: "license", ExpiresOn: tomorrow}},
	}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(), Clock: func() time.Time { return start.Add(8 * time.Hour) },
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodGet, "/api/v1/today", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Items    []map[string]any `json:"items"`
		Upcoming []map[string]any `json:"upcoming"`
		Summary  struct {
			Open      int `json:"open"`
			Completed int `json:"completed"`
			Upcoming  int `json:"upcoming"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) != 0 || len(payload.Upcoming) != 4 || payload.Summary.Open != 0 || payload.Summary.Completed != 0 || payload.Summary.Upcoming != 4 {
		t.Fatalf("payload = %#v", payload)
	}
	wantKinds := []string{"document", "event", "bill", "task"}
	for index, wantKind := range wantKinds {
		if payload.Upcoming[index]["kind"] != wantKind || payload.Upcoming[index]["urgency"] != "upcoming" {
			t.Fatalf("upcoming[%d] = %#v", index, payload.Upcoming[index])
		}
	}
}

func TestTaskCorrectionsPreserveTriStateAndRejectDSTFold(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "America/New_York"
	taskID := "00000000-0000-4000-8000-000000000101"
	fake := &fakeStore{
		profile:          domain.Profile{UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone},
		task:             domain.Task{ID: taskID, Title: "Task", Priority: domain.PriorityNormal},
		updateTaskResult: domain.Task{ID: taskID, Title: "Task", Priority: domain.PriorityNormal},
		uncompleteTask:   domain.Task{ID: taskID, Title: "Task", Priority: domain.PriorityNormal},
	}
	handler := New(Options{Store: fake, Verifier: dev, DevIssuer: dev, WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger()})
	token := issueThroughHTTP(t, handler)

	response := performJSON(t, handler, http.MethodPatch, "/api/v1/tasks/"+taskID, `{"notes":null,"due_local":null}`, token)
	if response.Code != http.StatusOK || !fake.updatedTask.NotesSet || fake.updatedTask.Notes != nil || !fake.updatedTask.DueAtSet || fake.updatedTask.DueAt != nil {
		t.Fatalf("clear patch status=%d params=%#v body=%s", response.Code, fake.updatedTask, response.Body.String())
	}
	response = performJSON(t, handler, http.MethodPatch, "/api/v1/tasks/"+taskID, `{"due_local":"2026-03-09T09:30"}`, token)
	wantDue := time.Date(2026, time.March, 9, 13, 30, 0, 0, time.UTC)
	if response.Code != http.StatusOK || fake.updatedTask.DueAt == nil || !fake.updatedTask.DueAt.Equal(wantDue) {
		t.Fatalf("timed patch status=%d due=%v body=%s", response.Code, fake.updatedTask.DueAt, response.Body.String())
	}
	response = performJSON(t, handler, http.MethodPatch, "/api/v1/tasks/"+taskID, `{"due_local":"2026-11-01T01:30"}`, token)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("fold status=%d body=%s", response.Code, response.Body.String())
	}

	response = performJSON(t, handler, http.MethodPost, "/api/v1/tasks/"+taskID+"/uncomplete", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("uncomplete status=%d body=%s", response.Code, response.Body.String())
	}
	response = performJSON(t, handler, http.MethodDelete, "/api/v1/tasks/"+taskID, "", token)
	if response.Code != http.StatusNoContent || fake.deleteTaskID != taskID {
		t.Fatalf("delete status=%d id=%q", response.Code, fake.deleteTaskID)
	}
}

func TestEventListAndStrictScheduleReplacement(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	eventID := "00000000-0000-4000-8000-000000000201"
	timedAt := time.Date(2026, time.August, 20, 1, 0, 0, 0, time.UTC)
	allDay := "2026-08-20"
	fake := &fakeStore{
		profile: domain.Profile{UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone},
		events: []domain.Event{
			{ID: "timed", Title: "Timed", Timezone: timezone, StartsAt: &timedAt},
			{ID: "all-day", Title: "All day", AllDay: true, Timezone: timezone, StartsOn: &allDay},
		},
		event:             domain.Event{ID: eventID, Title: "Event", Timezone: timezone, StartsAt: &timedAt},
		updateEventResult: domain.Event{ID: eventID, Title: "Event", Timezone: timezone, StartsAt: &timedAt},
	}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev, WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
		Clock: func() time.Time { return time.Date(2026, time.August, 19, 3, 0, 0, 0, time.UTC) },
	})
	token := issueThroughHTTP(t, handler)

	response := performJSON(t, handler, http.MethodGet, "/api/v1/events", "", token)
	if response.Code != http.StatusOK || fake.listEventFrom != "2026-08-19" || fake.listEventTo != "2026-09-18" {
		t.Fatalf("list status=%d range=%s..%s body=%s", response.Code, fake.listEventFrom, fake.listEventTo, response.Body.String())
	}
	var listed eventListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 2 || listed.Items[0].ID != "all-day" || listed.Items[1].ID != "timed" {
		t.Fatalf("event order = %#v", listed.Items)
	}

	response = performJSON(t, handler, http.MethodPatch, "/api/v1/events/"+eventID, `{"notes":null,"location":null}`, token)
	if response.Code != http.StatusOK || fake.updatedEvent.ScheduleSet || !fake.updatedEvent.NotesSet || !fake.updatedEvent.LocationSet {
		t.Fatalf("metadata status=%d params=%#v body=%s", response.Code, fake.updatedEvent, response.Body.String())
	}
	response = performJSON(t, handler, http.MethodPatch, "/api/v1/events/"+eventID, `{"all_day":true,"starts_on":"2026-08-21","ends_on":null}`, token)
	if response.Code != http.StatusOK || !fake.updatedEvent.ScheduleSet || !fake.updatedEvent.AllDay || fake.updatedEvent.StartsOn == nil || fake.updatedEvent.EndsOn != nil || fake.updatedEvent.StartsAt != nil {
		t.Fatalf("all-day replacement status=%d params=%#v body=%s", response.Code, fake.updatedEvent, response.Body.String())
	}
	response = performJSON(t, handler, http.MethodPatch, "/api/v1/events/"+eventID, `{"all_day":false,"starts_local":"2026-08-22T10:00","ends_local":"2026-08-22T11:00"}`, token)
	wantStart := time.Date(2026, time.August, 22, 3, 0, 0, 0, time.UTC)
	if response.Code != http.StatusOK || fake.updatedEvent.AllDay || fake.updatedEvent.StartsAt == nil || !fake.updatedEvent.StartsAt.Equal(wantStart) || fake.updatedEvent.Timezone != timezone {
		t.Fatalf("timed replacement status=%d params=%#v body=%s", response.Code, fake.updatedEvent, response.Body.String())
	}

	for _, body := range []string{
		`{"starts_local":"2026-08-22T10:00"}`,
		`{"all_day":false,"starts_local":"2026-08-22T10:00","starts_on":"2026-08-22"}`,
		`{"all_day":true}`,
		`{"all_day":true,"starts_on":"2026-08-22","ends_on":"2026-08-21"}`,
	} {
		response = performJSON(t, handler, http.MethodPatch, "/api/v1/events/"+eventID, body, token)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%s", body, response.Code, response.Body.String())
		}
	}
}

func TestEventScheduleReplacementRejectsDSTGap(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "America/New_York"
	eventID := "00000000-0000-4000-8000-000000000202"
	fake := &fakeStore{
		profile: domain.Profile{UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone},
		event:   domain.Event{ID: eventID},
	}
	handler := New(Options{Store: fake, Verifier: dev, DevIssuer: dev, WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger()})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodPatch, "/api/v1/events/"+eventID, `{"all_day":false,"starts_local":"2026-03-08T02:30"}`, token)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestBillCursorCorrectionsAndMarkUnpaid(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	billID := "00000000-0000-4000-8000-000000000301"
	secondID := "00000000-0000-4000-8000-000000000302"
	thirdID := "00000000-0000-4000-8000-000000000303"
	paidAt := time.Date(2026, time.August, 19, 2, 0, 0, 123, time.UTC)
	dueAt := time.Date(2026, time.August, 20, 2, 0, 0, 0, time.UTC)
	fake := &fakeStore{
		profile: domain.Profile{UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone},
		bills: []domain.Bill{
			{ID: billID, Title: "One", Amount: 1, Currency: "IDR", DueAt: dueAt, PaidAt: &paidAt},
			{ID: secondID, Title: "Two", Amount: 2, Currency: "IDR", DueAt: dueAt, PaidAt: &paidAt},
			{ID: thirdID, Title: "Three", Amount: 3, Currency: "IDR", DueAt: dueAt, PaidAt: &paidAt},
		},
		bill:             domain.Bill{ID: billID, Title: "One", Amount: 1, Currency: "IDR", DueAt: dueAt, PaidAt: &paidAt},
		updateBillResult: domain.Bill{ID: billID, Title: "One", Amount: 1, Currency: "IDR", DueAt: dueAt, PaidAt: &paidAt},
		markedUnpaidBill: domain.Bill{ID: billID, Title: "One", Amount: 1, Currency: "IDR", DueAt: dueAt},
	}
	handler := New(Options{Store: fake, Verifier: dev, DevIssuer: dev, WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger()})
	token := issueThroughHTTP(t, handler)

	response := performJSON(t, handler, http.MethodGet, "/api/v1/bills?state=paid&limit=2", "", token)
	if response.Code != http.StatusOK || fake.listBillState != "paid" || fake.listBillLimit != 3 {
		t.Fatalf("list status=%d state=%q limit=%d body=%s", response.Code, fake.listBillState, fake.listBillLimit, response.Body.String())
	}
	var listed billListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 2 || listed.NextCursor == nil {
		t.Fatalf("list response = %#v", listed)
	}
	at, id, err := decodeBillCursor(*listed.NextCursor, "paid")
	if err != nil || !at.Equal(paidAt) || id != secondID {
		t.Fatalf("cursor at=%s id=%q err=%v", at, id, err)
	}
	response = performJSON(t, handler, http.MethodGet, "/api/v1/bills?state=paid&limit=2&cursor="+*listed.NextCursor, "", token)
	if response.Code != http.StatusOK || fake.listBillAfterAt == nil || fake.listBillAfterID == nil || *fake.listBillAfterID != secondID {
		t.Fatalf("cursor request status=%d at=%v id=%v", response.Code, fake.listBillAfterAt, fake.listBillAfterID)
	}
	if mismatch, err := encodeBillCursor("paid", paidAt, billID); err != nil {
		t.Fatal(err)
	} else {
		response = performJSON(t, handler, http.MethodGet, "/api/v1/bills?state=unpaid&cursor="+mismatch, "", token)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("state mismatch status=%d body=%s", response.Code, response.Body.String())
		}
	}

	response = performJSON(t, handler, http.MethodPatch, "/api/v1/bills/"+billID, `{"notes":null,"amount":2500,"currency":"USD","due_local":"2026-08-21T09:00"}`, token)
	wantDue := time.Date(2026, time.August, 21, 2, 0, 0, 0, time.UTC)
	if response.Code != http.StatusOK || !fake.updatedBill.NotesSet || fake.updatedBill.Notes != nil || fake.updatedBill.Amount == nil || *fake.updatedBill.Amount != 2500 ||
		fake.updatedBill.Currency == nil || *fake.updatedBill.Currency != "USD" || fake.updatedBill.DueAt == nil || !fake.updatedBill.DueAt.Equal(wantDue) {
		t.Fatalf("patch status=%d params=%#v body=%s", response.Code, fake.updatedBill, response.Body.String())
	}
	response = performJSON(t, handler, http.MethodPost, "/api/v1/bills/"+billID+"/mark-unpaid", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("mark-unpaid status=%d body=%s", response.Code, response.Body.String())
	}
	response = performJSON(t, handler, http.MethodDelete, "/api/v1/bills/"+billID, "", token)
	if response.Code != http.StatusNoContent || fake.deleteBillID != billID {
		t.Fatalf("delete status=%d id=%q", response.Code, fake.deleteBillID)
	}
}

func TestCorrectionRoutesMapCrossOwnerAndInvalidIDsToNotFound(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	fake := &fakeStore{
		profile: domain.Profile{UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone},
		taskErr: store.ErrNotFound, eventErr: store.ErrNotFound, billErr: store.ErrNotFound,
	}
	handler := New(Options{Store: fake, Verifier: dev, DevIssuer: dev, WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger()})
	token := issueThroughHTTP(t, handler)
	for _, path := range []string{
		"/api/v1/tasks/00000000-0000-4000-8000-000000000401",
		"/api/v1/events/00000000-0000-4000-8000-000000000402",
		"/api/v1/bills/00000000-0000-4000-8000-000000000403",
		"/api/v1/tasks/not-a-uuid",
		"/api/v1/events/not-a-uuid",
		"/api/v1/bills/not-a-uuid",
	} {
		response := performJSON(t, handler, http.MethodGet, path, "", token)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestCorrectionPatchesRejectEmptyUnknownAndInvalidNullFields(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	fake := &fakeStore{profile: domain.Profile{UserID: "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270", Timezone: &timezone}}
	handler := New(Options{Store: fake, Verifier: dev, DevIssuer: dev, WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger()})
	token := issueThroughHTTP(t, handler)
	tests := []struct {
		path string
		body string
	}{
		{"/api/v1/tasks/00000000-0000-4000-8000-000000000501", `{}`},
		{"/api/v1/tasks/00000000-0000-4000-8000-000000000501", `{"title":null}`},
		{"/api/v1/tasks/00000000-0000-4000-8000-000000000501", `{"user_id":"00000000-0000-4000-8000-000000000999"}`},
		{"/api/v1/events/00000000-0000-4000-8000-000000000502", `{}`},
		{"/api/v1/events/00000000-0000-4000-8000-000000000502", `{"all_day":null,"starts_on":"2026-08-20"}`},
		{"/api/v1/bills/00000000-0000-4000-8000-000000000503", `{}`},
		{"/api/v1/bills/00000000-0000-4000-8000-000000000503", `{"due_local":null}`},
		{"/api/v1/bills/00000000-0000-4000-8000-000000000503", `{"amount":1.5}`},
	}
	for _, test := range tests {
		response := performJSON(t, handler, http.MethodPatch, test.path, test.body, token)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s %s status=%d body=%s", test.path, test.body, response.Code, response.Body.String())
		}
		assertErrorCode(t, response.Body.Bytes(), "VALIDATION_ERROR")
	}
	actionPath := "/api/v1/bills/00000000-0000-4000-8000-000000000503/mark-unpaid"
	response := performJSON(t, handler, http.MethodPost, actionPath, `{"paid":false}`, token)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("action unknown body status=%d body=%s", response.Code, response.Body.String())
	}
	oversized := `{"padding":"` + strings.Repeat("x", maxJSONBody) + `"}`
	response = performJSON(t, handler, http.MethodPost, actionPath, oversized, token)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("action oversized status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPrivateResponsesAreNoStoreAndDeletePreflightIsAllowed(t *testing.T) {
	dev := mustDevAuth(t)
	handler := New(Options{
		Store: &fakeStore{}, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodGet, "/api/v1/profile", "", token)
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/documents/00000000-0000-4000-8000-000000000010", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	preflight := httptest.NewRecorder()
	handler.ServeHTTP(preflight, request)
	if preflight.Code != http.StatusNoContent || !strings.Contains(preflight.Header().Get("Access-Control-Allow-Methods"), "DELETE") {
		t.Fatalf("preflight status=%d methods=%q", preflight.Code, preflight.Header().Get("Access-Control-Allow-Methods"))
	}
}

func TestCompleteMapsCrossOwnerToNotFound(t *testing.T) {
	dev := mustDevAuth(t)
	fake := &fakeStore{completeErr: store.ErrNotFound}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
	})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodPost,
		"/api/v1/tasks/00000000-0000-4000-8000-000000000001/complete", `{}`, token)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
}

func TestStrictBearerAndReadiness(t *testing.T) {
	dev := mustDevAuth(t)
	fake := &fakeStore{pingErr: errors.New("database unavailable")}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	request.Header.Add("Authorization", "Bearer first")
	request.Header.Add("Authorization", "Bearer second")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || response.Header().Get("X-Request-ID") == "" {
		t.Fatalf("strict auth status=%d request_id=%q", response.Code, response.Header().Get("X-Request-ID"))
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d", response.Code)
	}
}

func mustDevAuth(t *testing.T) *auth.DevAuth {
	t.Helper()
	dev, err := auth.NewDevAuth("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	return dev
}

func issueThroughHTTP(t *testing.T, handler http.Handler) string {
	t.Helper()
	response := performJSON(t, handler, http.MethodPost, "/api/v1/auth/dev-session", `{"email":"user@example.com"}`, "")
	if response.Code != http.StatusOK {
		t.Fatalf("dev session status = %d, body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.AccessToken
}

func performJSON(t *testing.T, handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	request := httptest.NewRequest(method, path, reader)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertErrorCode(t *testing.T, body []byte, want string) {
	t.Helper()
	var payload errorEnvelope
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != want {
		t.Fatalf("error code = %q, want %q", payload.Error.Code, want)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
