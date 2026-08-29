package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"lifehub/services/api/internal/domain"
)

func TestRecurringCreatesUseProfileTimezoneAndMaterializationWindow(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	now := time.Date(2026, time.August, 19, 3, 0, 0, 0, time.UTC)
	fake := &fakeStore{profile: domain.Profile{UserID: "profile", Timezone: &timezone, Locale: "id-ID", Currency: "IDR"}}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
		Clock: func() time.Time { return now },
	})
	token := issueThroughHTTP(t, handler)

	response := performJSON(t, handler, http.MethodPost, "/api/v1/tasks", `{
		"title":"Tutup buku","priority":"high","due_local":"2026-08-31T09:15",
		"recurrence":{"frequency":"monthly","interval":2,"ends_on":"2027-02-28"}
	}`, token)
	if response.Code != http.StatusCreated {
		t.Fatalf("recurring task status=%d body=%s", response.Code, response.Body.String())
	}
	if fake.createdRecurringTask.SeriesID == "" || fake.createdRecurringTask.Frequency != "monthly" || fake.createdRecurringTask.Interval != 2 {
		t.Fatalf("recurring task params=%#v", fake.createdRecurringTask)
	}
	if got := fake.createdRecurringTask.AnchorOn.Format(time.DateOnly); got != "2026-08-31" {
		t.Fatalf("task anchor=%s", got)
	}
	if fake.createdRecurringTask.TimeLocal != "09:15:00" || fake.createdRecurringTask.Timezone != timezone {
		t.Fatalf("task local schedule=%q timezone=%q", fake.createdRecurringTask.TimeLocal, fake.createdRecurringTask.Timezone)
	}
	if got := fake.createdRecurringTask.ThroughOn.Format(time.DateOnly); got != "2026-11-17" {
		t.Fatalf("task materialization through=%s", got)
	}

	response = performJSON(t, handler, http.MethodPost, "/api/v1/events", `{
		"title":"Retret","all_day":true,"starts_on":"2026-08-21","ends_on":"2026-08-23",
		"recurrence":{"frequency":"weekly"}
	}`, token)
	if response.Code != http.StatusCreated {
		t.Fatalf("recurring event status=%d body=%s", response.Code, response.Body.String())
	}
	if fake.createdRecurringEvent.SeriesID == "" || fake.createdRecurringEvent.Frequency != "weekly" || fake.createdRecurringEvent.Interval != 1 {
		t.Fatalf("recurring event params=%#v", fake.createdRecurringEvent)
	}
	if fake.createdRecurringEvent.AllDaySpan != 2 || fake.createdRecurringEvent.TimeLocal != nil {
		t.Fatalf("all-day recurrence shape=%#v", fake.createdRecurringEvent)
	}

	response = performJSON(t, handler, http.MethodPost, "/api/v1/bills", `{
		"title":"Sewa","amount":2500000,"due_local":"2026-09-01T08:00",
		"recurrence":{"frequency":"monthly","interval":1}
	}`, token)
	if response.Code != http.StatusCreated {
		t.Fatalf("recurring bill status=%d body=%s", response.Code, response.Body.String())
	}
	if fake.createdRecurringBill.SeriesID == "" || fake.createdRecurringBill.TimeLocal != "08:00:00" || fake.createdRecurringBill.Timezone != timezone {
		t.Fatalf("recurring bill params=%#v", fake.createdRecurringBill)
	}
}

func TestRecurringCreateValidation(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	fake := &fakeStore{profile: domain.Profile{UserID: "profile", Timezone: &timezone, Locale: "id-ID", Currency: "IDR"}}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
		Clock: func() time.Time { return time.Date(2026, time.August, 19, 3, 0, 0, 0, time.UTC) },
	})
	token := issueThroughHTTP(t, handler)

	tests := []struct {
		name, path, body, field string
	}{
		{"task needs due", "/api/v1/tasks", `{"title":"Tugas","recurrence":{"frequency":"daily"}}`, "due_local"},
		{"bad frequency", "/api/v1/tasks", `{"title":"Tugas","due_local":"2026-08-20T09:00","recurrence":{"frequency":"hourly"}}`, "recurrence.frequency"},
		{"zero interval", "/api/v1/bills", `{"title":"Tagihan","amount":1,"due_local":"2026-08-20T09:00","recurrence":{"frequency":"daily","interval":0}}`, "recurrence.interval"},
		{"end before anchor", "/api/v1/events", `{"title":"Jadwal","all_day":true,"starts_on":"2026-08-20","recurrence":{"frequency":"weekly","ends_on":"2026-08-19"}}`, "recurrence.ends_on"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := performJSON(t, handler, http.MethodPost, test.path, test.body, token)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			var payload struct {
				Error struct {
					Fields map[string]string `json:"fields"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Error.Fields[test.field] == "" {
				t.Fatalf("fields=%v, want %q", payload.Error.Fields, test.field)
			}
		})
	}
}

func TestRecurrenceSeriesRoutesAndReadMaterialization(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	now := time.Date(2026, time.August, 19, 3, 0, 0, 0, time.UTC)
	seriesID := "00000000-0000-4000-8000-000000000701"
	fake := &fakeStore{
		profile: domain.Profile{UserID: "profile", Timezone: &timezone, Locale: "id-ID", Currency: "IDR"},
		recurrenceSeries: []domain.RecurrenceSeries{{
			ID: seriesID, SourceKind: "bill", Title: "Sewa", Frequency: "monthly", Interval: 1,
			AnchorOn: "2026-09-01", Timezone: timezone, Active: true,
		}},
		recurrenceItem: domain.RecurrenceSeries{
			ID: seriesID, SourceKind: "bill", Title: "Sewa", Frequency: "monthly", Interval: 1,
			AnchorOn: "2026-09-01", Timezone: timezone, Active: true,
		},
	}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev,
		WebOrigins: []string{"http://localhost:3000"}, Logger: discardLogger(),
		Clock: func() time.Time { return now },
	})
	token := issueThroughHTTP(t, handler)

	response := performJSON(t, handler, http.MethodGet, "/api/v1/recurrence-series", "", token)
	if response.Code != http.StatusOK || !containsAll(response.Body.String(), `"title":"Sewa"`, `"source_kind":"bill"`) {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	response = performJSON(t, handler, http.MethodGet, "/api/v1/recurrence-series/"+seriesID, "", token)
	if response.Code != http.StatusOK || !containsAll(response.Body.String(), `"id":"`+seriesID+`"`) {
		t.Fatalf("get status=%d body=%s", response.Code, response.Body.String())
	}
	response = performJSON(t, handler, http.MethodPatch, "/api/v1/recurrence-series/"+seriesID,
		`{"frequency":"weekly","interval":2,"ends_on":"2027-01-01"}`, token)
	if response.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", response.Code, response.Body.String())
	}
	if fake.updatedRecurrence.ID != seriesID || fake.updatedRecurrence.Frequency != "weekly" || fake.updatedRecurrence.Interval != 2 {
		t.Fatalf("updated recurrence=%#v", fake.updatedRecurrence)
	}
	if fake.updatedRecurrence.EndsOn == nil || fake.updatedRecurrence.EndsOn.Format(time.DateOnly) != "2027-01-01" {
		t.Fatalf("updated recurrence end=%v", fake.updatedRecurrence.EndsOn)
	}
	response = performJSON(t, handler, http.MethodDelete, "/api/v1/recurrence-series/"+seriesID, "", token)
	if response.Code != http.StatusNoContent {
		t.Fatalf("stop status=%d body=%s", response.Code, response.Body.String())
	}
	if fake.stoppedSeriesID != seriesID || fake.stoppedFrom.Format(time.DateOnly) != "2026-08-19" {
		t.Fatalf("stop id=%q from=%s", fake.stoppedSeriesID, fake.stoppedFrom.Format(time.DateOnly))
	}

	response = performJSON(t, handler, http.MethodGet, "/api/v1/today", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("today status=%d body=%s", response.Code, response.Body.String())
	}
	if fake.materializeFrom.Format(time.DateOnly) != "2026-08-19" || fake.materializeThrough.Format(time.DateOnly) != "2026-11-17" {
		t.Fatalf("Today materialization=%s..%s", fake.materializeFrom.Format(time.DateOnly), fake.materializeThrough.Format(time.DateOnly))
	}
	response = performJSON(t, handler, http.MethodGet, "/api/v1/agenda?from=2026-09-01&to=2026-09-10", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("agenda status=%d body=%s", response.Code, response.Body.String())
	}
	if fake.materializeFrom.Format(time.DateOnly) != "2026-09-01" || fake.materializeThrough.Format(time.DateOnly) != "2026-09-10" {
		t.Fatalf("Agenda materialization=%s..%s", fake.materializeFrom.Format(time.DateOnly), fake.materializeThrough.Format(time.DateOnly))
	}
}

func containsAll(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			return false
		}
	}
	return true
}
