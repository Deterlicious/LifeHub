package smartcapture

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRuleProviderIndonesianDrafts(t *testing.T) {
	provider := RuleProvider{}
	now := time.Date(2026, time.August, 20, 3, 0, 0, 0, time.UTC)
	tests := []struct {
		name, input, kind, title string
		assert                   func(*testing.T, Output)
	}{
		{
			name: "recurring bill", input: "Bayar internet 350 ribu tanggal 15 tiap bulan", kind: "bill", title: "Internet",
			assert: func(t *testing.T, output Output) {
				if output.Draft.Amount == nil || *output.Draft.Amount != 350_000 || output.Draft.Recurrence == nil || output.Draft.Recurrence.Frequency != "monthly" {
					t.Fatalf("bill draft=%#v", output.Draft)
				}
				if len(output.Ambiguities) == 0 {
					t.Fatal("bill without time should require review")
				}
			},
		},
		{
			name: "event tomorrow", input: "Meeting besok jam 2 siang", kind: "event", title: "Meeting",
			assert: func(t *testing.T, output Output) {
				if output.Draft.StartsLocal != "2026-08-21T14:00:00" || len(output.Ambiguities) != 0 {
					t.Fatalf("event output=%#v", output)
				}
			},
		},
		{
			name: "document expiry", input: "SIM habis 6 November 2026 ingatkan 30 hari sebelumnya", kind: "document", title: "SIM",
			assert: func(t *testing.T, output Output) {
				if output.Draft.ExpiresOn != "2026-11-06" || output.Draft.Category != "license" {
					t.Fatalf("document draft=%#v", output.Draft)
				}
			},
		},
		{
			name: "ambiguous clock", input: "Rapat besok jam 2", kind: "event", title: "Rapat",
			assert: func(t *testing.T, output Output) {
				if output.Draft.StartsLocal != "" || len(output.Ambiguities) == 0 {
					t.Fatalf("ambiguous output=%#v", output)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := provider.Parse(context.Background(), test.input, now, "Asia/Jakarta")
			if err != nil {
				t.Fatal(err)
			}
			if output.Draft.Kind != test.kind || output.Draft.Title != test.title {
				t.Fatalf("draft=%#v, want kind=%s title=%q", output.Draft, test.kind, test.title)
			}
			test.assert(t, output)
			if err := ValidateOutput(output); err != nil {
				t.Fatalf("valid provider output rejected: %v", err)
			}
		})
	}
}

func TestRuleProviderCancellationAndInputBounds(t *testing.T) {
	provider := RuleProvider{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.Parse(ctx, "Tugas", time.Now(), "Asia/Jakarta"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error=%v", err)
	}
	if _, err := provider.Parse(context.Background(), "", time.Now(), "Asia/Jakarta"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty error=%v", err)
	}
}

func TestMockProviderAndOutputValidation(t *testing.T) {
	want := Output{Draft: Draft{Kind: "task", Title: "Mock", Priority: "normal", Confidence: 1}}
	output, err := (MockProvider{Output: want}).Parse(context.Background(), "ignored", time.Now(), "Asia/Jakarta")
	if err != nil || output.Draft.Title != "Mock" {
		t.Fatalf("mock output=%#v error=%v", output, err)
	}
	if err := ValidateOutput(Output{Draft: Draft{Kind: "unknown", Confidence: 1}}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("malformed provider output error=%v", err)
	}
}
