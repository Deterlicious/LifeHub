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
	"testing"
	"time"

	"lifehub/services/api/internal/auth"
	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/store"
)

type fakeStore struct {
	profile      domain.Profile
	profileErr   error
	created      domain.CreateTaskParams
	completeTask domain.Task
	completeErr  error
	todayTasks   []domain.Task
	pingErr      error
}

func (fake *fakeStore) Ping(context.Context) error {
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

func (fake *fakeStore) UpdateProfileTimezone(_ context.Context, userID, timezone string) (domain.Profile, error) {
	fake.profile = domain.Profile{UserID: userID, Timezone: &timezone, Locale: "id-ID", Currency: "IDR"}
	return fake.profile, nil
}

func (fake *fakeStore) CreateTask(_ context.Context, params domain.CreateTaskParams) (domain.Task, error) {
	fake.created = params
	now := time.Date(2026, time.August, 19, 1, 0, 0, 0, time.UTC)
	return domain.Task{
		ID: params.ID, Title: params.Title, Notes: params.Notes, Priority: params.Priority,
		DueAt: params.DueAt, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (fake *fakeStore) CompleteTask(context.Context, string, string) (domain.Task, error) {
	return fake.completeTask, fake.completeErr
}

func (fake *fakeStore) ListTodayTasks(context.Context, string, time.Time, time.Time) ([]domain.Task, error) {
	return fake.todayTasks, nil
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
