package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/store"
)

func TestReminderCreateAndPatchContracts(t *testing.T) {
	dev := mustDevAuth(t)
	sourceID := "11111111-1111-4111-8111-111111111111"
	reminderID := "22222222-2222-4222-8222-222222222222"
	fireAt := time.Date(2026, time.August, 25, 8, 0, 0, 0, time.UTC)
	fake := &fakeStore{reminder: domain.Reminder{
		ID: reminderID, SourceKind: "task", SourceID: sourceID, Status: "scheduled", NextFireAt: &fireAt,
	}}
	handler := New(Options{Store: fake, Verifier: dev, DevIssuer: dev, Logger: discardLogger()})
	token := issueThroughHTTP(t, handler)

	created := performJSON(t, handler, http.MethodPost, "/api/v1/reminders", `{
		"source_kind":"task","source_id":"`+sourceID+`",
		"schedule":{"kind":"before_moment","minutes_before":15}
	}`, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", created.Code, created.Body.String())
	}
	if fake.createdReminder.UserID == "" || fake.createdReminder.SourceID != sourceID || fake.createdReminder.MinutesBefore == nil || *fake.createdReminder.MinutesBefore != 15 {
		t.Fatalf("create params = %#v", fake.createdReminder)
	}

	patched := performJSON(t, handler, http.MethodPatch, "/api/v1/reminders/"+reminderID, `{
		"schedule":{"kind":"before_date","days_before":3,"time_local":"09:05"}
	}`, token)
	if patched.Code != http.StatusOK {
		t.Fatalf("patch status = %d body=%s", patched.Code, patched.Body.String())
	}
	if fake.updatedReminder.DaysBefore == nil || *fake.updatedReminder.DaysBefore != 3 || fake.updatedReminder.TimeLocal == nil || *fake.updatedReminder.TimeLocal != "09:05" {
		t.Fatalf("update params = %#v", fake.updatedReminder)
	}
}

func TestReminderStrictScheduleValidationAndErrorMapping(t *testing.T) {
	dev := mustDevAuth(t)
	sourceID := "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name string
		body string
	}{
		{name: "missing schedule", body: `{"source_kind":"task","source_id":"` + sourceID + `"}`},
		{name: "null schedule", body: `{"source_kind":"task","source_id":"` + sourceID + `","schedule":null}`},
		{name: "mixed union", body: `{"source_kind":"task","source_id":"` + sourceID + `","schedule":{"kind":"before_moment","minutes_before":1,"days_before":1}}`},
		{name: "float", body: `{"source_kind":"task","source_id":"` + sourceID + `","schedule":{"kind":"before_moment","minutes_before":1.5}}`},
		{name: "unknown nested", body: `{"source_kind":"task","source_id":"` + sourceID + `","schedule":{"kind":"before_moment","minutes_before":1,"private":true}}`},
		{name: "invalid local", body: `{"source_kind":"document","source_id":"` + sourceID + `","schedule":{"kind":"before_date","days_before":0,"time_local":"9:00"}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := New(Options{Store: &fakeStore{}, Verifier: dev, DevIssuer: dev, Logger: discardLogger()})
			token := issueThroughHTTP(t, handler)
			response := performJSON(t, handler, http.MethodPost, "/api/v1/reminders", test.body, token)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}

	for _, mapped := range []struct {
		err  error
		code int
	}{
		{err: store.ErrNotFound, code: http.StatusNotFound},
		{err: store.ErrReminderSourceUnscheduled, code: http.StatusConflict},
		{err: store.ErrReminderScheduleMismatch, code: http.StatusBadRequest},
		{err: store.ErrReminderInvalidLocalTime, code: http.StatusBadRequest},
	} {
		fake := &fakeStore{reminderErr: mapped.err}
		handler := New(Options{Store: fake, Verifier: dev, DevIssuer: dev, Logger: discardLogger()})
		token := issueThroughHTTP(t, handler)
		response := performJSON(t, handler, http.MethodPost, "/api/v1/reminders", `{
			"source_kind":"task","source_id":"`+sourceID+`",
			"schedule":{"kind":"before_moment","minutes_before":0}
		}`, token)
		if response.Code != mapped.code {
			t.Fatalf("error %v mapped to %d body=%s", mapped.err, response.Code, response.Body.String())
		}
	}
}

func TestReminderListRequiresExactOwnedSourcePair(t *testing.T) {
	dev := mustDevAuth(t)
	sourceID := "11111111-1111-4111-8111-111111111111"
	fake := &fakeStore{reminders: []domain.Reminder{}}
	handler := New(Options{Store: fake, Verifier: dev, DevIssuer: dev, Logger: discardLogger()})
	token := issueThroughHTTP(t, handler)
	for _, path := range []string{
		"/api/v1/reminders", "/api/v1/reminders?source_kind=task", "/api/v1/reminders?source_kind=task&source_id=" + sourceID + "&extra=1",
	} {
		response := performJSON(t, handler, http.MethodGet, path, "", token)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	fake.reminderErr = store.ErrNotFound
	response := performJSON(t, handler, http.MethodGet, "/api/v1/reminders?source_kind=task&source_id="+sourceID, "", token)
	if response.Code != http.StatusNotFound {
		t.Fatalf("cross-owner status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestNotificationCursorAndReadActions(t *testing.T) {
	dev := mustDevAuth(t)
	firstID := "11111111-1111-4111-8111-111111111111"
	secondID := "22222222-2222-4222-8222-222222222222"
	thirdID := "33333333-3333-4333-8333-333333333333"
	base := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	fake := &fakeStore{unreadCount: 3, notifications: []domain.Notification{
		{ID: firstID, CreatedAt: base}, {ID: secondID, CreatedAt: base.Add(-time.Minute)}, {ID: thirdID, CreatedAt: base.Add(-2 * time.Minute)},
	}}
	handler := New(Options{Store: fake, Verifier: dev, DevIssuer: dev, Logger: discardLogger()})
	token := issueThroughHTTP(t, handler)
	response := performJSON(t, handler, http.MethodGet, "/api/v1/notifications?limit=2", "", token)
	if response.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", response.Code, response.Body.String())
	}
	var listed notificationListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 2 || listed.NextCursor == nil || fake.listNotifyLimit != 3 || listed.UnreadCount != 3 {
		t.Fatalf("list response=%#v limit=%d", listed, fake.listNotifyLimit)
	}
	at, id, err := decodeNotificationCursor(*listed.NextCursor)
	if err != nil || id != secondID || !at.Equal(base.Add(-time.Minute)) {
		t.Fatalf("cursor at=%s id=%s err=%v", at, id, err)
	}

	fake.notification = domain.Notification{ID: firstID, CreatedAt: base}
	fake.unreadCount = 2
	read := performJSON(t, handler, http.MethodPost, "/api/v1/notifications/"+firstID+"/mark-read", `{}`, token)
	if read.Code != http.StatusOK {
		t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
	}
	fake.markedAllCount = 2
	all := performJSON(t, handler, http.MethodPost, "/api/v1/notifications/mark-all-read", "", token)
	if all.Code != http.StatusOK {
		t.Fatalf("all status=%d body=%s", all.Code, all.Body.String())
	}

	fake.notificationErr = store.ErrNotFound
	missing := performJSON(t, handler, http.MethodPost, "/api/v1/notifications/"+thirdID+"/mark-read", "", token)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status=%d body=%s", missing.Code, missing.Body.String())
	}
	fake.notificationErr = errors.New("database failed")
	count := performJSON(t, handler, http.MethodGet, "/api/v1/notifications/unread-count", "", token)
	if count.Code != http.StatusInternalServerError {
		t.Fatalf("count status=%d body=%s", count.Code, count.Body.String())
	}
}
