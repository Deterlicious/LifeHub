package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/smartcapture"
)

type failingSmartCaptureProvider struct {
	err error
}

func (failingSmartCaptureProvider) Name() string { return "failing" }

func (provider failingSmartCaptureProvider) Parse(context.Context, string, time.Time, string) (smartcapture.Output, error) {
	return smartcapture.Output{}, provider.err
}

func TestSmartCaptureCreatesEditableDraftWithoutWriting(t *testing.T) {
	dev := mustDevAuth(t)
	timezone := "Asia/Jakarta"
	fake := &fakeStore{profile: domain.Profile{UserID: "user", Timezone: &timezone, Locale: "id-ID", Currency: "IDR"}}
	handler := New(Options{
		Store: fake, Verifier: dev, DevIssuer: dev, Logger: discardLogger(),
		Clock: func() time.Time { return time.Date(2026, time.August, 20, 3, 0, 0, 0, time.UTC) },
	})
	token := issueThroughHTTP(t, handler)

	response := performJSON(t, handler, http.MethodPost, "/api/v1/smart-capture/parse",
		`{"text":"Bayar internet 350 ribu tanggal 15 tiap bulan"}`, token)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Draft       smartcapture.Draft `json:"draft"`
		Ambiguities []string           `json:"ambiguities"`
		Provider    string             `json:"provider"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Provider != "rule" || payload.Draft.Kind != "bill" || payload.Draft.Title != "Internet" || payload.Draft.Amount == nil || *payload.Draft.Amount != 350_000 {
		t.Fatalf("payload=%#v", payload)
	}
	if payload.Draft.Recurrence == nil || payload.Draft.Recurrence.Frequency != "monthly" || len(payload.Ambiguities) == 0 {
		t.Fatalf("draft did not preserve recurrence/review requirements: %#v", payload)
	}
	if fake.created.ID != "" || fake.createdBill.ID != "" || fake.createdEvent.ID != "" || fake.createdDocument.ID != "" {
		t.Fatalf("smart capture unexpectedly performed a domain write: %#v", fake)
	}
}

func TestSmartCaptureValidatesProfileAndRequest(t *testing.T) {
	dev := mustDevAuth(t)
	tests := []struct {
		name    string
		profile domain.Profile
		body    string
		status  int
		code    string
	}{
		{name: "empty", profile: profileWithTimezone("Asia/Jakarta"), body: `{"text":"  "}`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "unknown field", profile: profileWithTimezone("Asia/Jakarta"), body: `{"text":"Tugas","save":true}`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "timezone missing", profile: domain.Profile{UserID: "user"}, body: `{"text":"Tugas"}`, status: http.StatusConflict, code: "PROFILE_INCOMPLETE"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := New(Options{Store: &fakeStore{profile: test.profile}, Verifier: dev, DevIssuer: dev, Logger: discardLogger()})
			token := issueThroughHTTP(t, handler)
			response := performJSON(t, handler, http.MethodPost, "/api/v1/smart-capture/parse", test.body, token)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			assertErrorCode(t, response.Body.Bytes(), test.code)
		})
	}
}

func TestSmartCaptureRejectsUnavailableOrMalformedProvider(t *testing.T) {
	dev := mustDevAuth(t)
	providers := []smartcapture.Provider{
		failingSmartCaptureProvider{err: errors.New("provider unavailable")},
		failingSmartCaptureProvider{err: context.DeadlineExceeded},
		smartcapture.MockProvider{Output: smartcapture.Output{Draft: smartcapture.Draft{Kind: "unknown", Title: "Invalid", Confidence: 1}}},
	}
	for _, provider := range providers {
		handler := New(Options{
			Store: &fakeStore{profile: profileWithTimezone("Asia/Jakarta")}, Verifier: dev, DevIssuer: dev,
			SmartCaptureProvider: provider, Logger: discardLogger(),
		})
		token := issueThroughHTTP(t, handler)
		response := performJSON(t, handler, http.MethodPost, "/api/v1/smart-capture/parse", `{"text":"Buat tugas"}`, token)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("provider=%s status=%d body=%s", provider.Name(), response.Code, response.Body.String())
		}
		assertErrorCode(t, response.Body.Bytes(), "SMART_CAPTURE_UNAVAILABLE")
	}
}

func TestSmartCaptureRateLimitIsScopedAfterAuthentication(t *testing.T) {
	dev := mustDevAuth(t)
	handler := New(Options{
		Store: &fakeStore{profile: profileWithTimezone("Asia/Jakarta")}, Verifier: dev, DevIssuer: dev,
		Logger: discardLogger(), Clock: func() time.Time { return time.Date(2026, time.August, 27, 1, 0, 0, 0, time.UTC) },
	})
	token := issueThroughHTTP(t, handler)
	for attempt := 1; attempt <= 21; attempt++ {
		response := performJSON(t, handler, http.MethodPost, "/api/v1/smart-capture/parse", `{"text":"Buat tugas"}`, token)
		if attempt <= 20 && response.Code != http.StatusOK {
			t.Fatalf("attempt=%d status=%d body=%s", attempt, response.Code, response.Body.String())
		}
		if attempt == 21 {
			if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "" {
				t.Fatalf("rate limited status=%d headers=%#v", response.Code, response.Header())
			}
			assertErrorCode(t, response.Body.Bytes(), "RATE_LIMITED")
		}
	}
}

func profileWithTimezone(value string) domain.Profile {
	return domain.Profile{UserID: "user", Timezone: &value, Locale: "id-ID", Currency: "IDR"}
}
