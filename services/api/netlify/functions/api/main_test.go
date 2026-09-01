package main

import "testing"

func TestNormalizePathSupportsDirectAndRewrittenInvocations(t *testing.T) {
	tests := map[string]string{
		"/api/v1/today":                                "/api/v1/today",
		"/.netlify/functions/api":                      "/",
		"/.netlify/functions/api/api/v1/notifications": "/api/v1/notifications",
	}
	for input, want := range tests {
		if got := normalizePath(input); got != want {
			t.Fatalf("normalizePath(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestUnavailableResponseIsPrivateAndRetryable(t *testing.T) {
	response := unavailableResponse()
	if response.StatusCode != 503 || response.Headers["Cache-Control"] != "no-store" || response.Headers["Retry-After"] == "" {
		t.Fatalf("response=%#v", response)
	}
}
