package recurrence

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func formatted(values []time.Time) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.Format(time.DateOnly)
	}
	return result
}

func TestDatesUsesCalendarIntervalsAndInclusiveEnd(t *testing.T) {
	ends := mustDate(t, "2027-01-10")
	tests := []struct {
		name     string
		anchor   string
		from     string
		through  string
		rule     Rule
		expected []string
	}{
		{
			name:   "daily interval",
			anchor: "2026-08-20", from: "2026-08-20", through: "2026-08-27",
			rule:     Rule{Frequency: FrequencyDaily, Interval: 2},
			expected: []string{"2026-08-20", "2026-08-22", "2026-08-24", "2026-08-26"},
		},
		{
			name:   "weekly anchor weekday",
			anchor: "2026-08-20", from: "2026-08-20", through: "2026-09-20",
			rule:     Rule{Frequency: FrequencyWeekly, Interval: 2},
			expected: []string{"2026-08-20", "2026-09-03", "2026-09-17"},
		},
		{
			name:   "inclusive end",
			anchor: "2027-01-01", from: "2027-01-01", through: "2027-01-20",
			rule:     Rule{Frequency: FrequencyDaily, Interval: 3, EndsOn: &ends},
			expected: []string{"2027-01-01", "2027-01-04", "2027-01-07", "2027-01-10"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values, err := Dates(mustDate(t, test.anchor), mustDate(t, test.from), mustDate(t, test.through), test.rule)
			if err != nil {
				t.Fatal(err)
			}
			if got := formatted(values); !reflect.DeepEqual(got, test.expected) {
				t.Fatalf("dates=%v want=%v", got, test.expected)
			}
		})
	}
}

func TestDatesMonthlyClampDoesNotDrift(t *testing.T) {
	values, err := Dates(
		mustDate(t, "2027-01-31"),
		mustDate(t, "2027-01-31"),
		mustDate(t, "2027-05-31"),
		Rule{Frequency: FrequencyMonthly, Interval: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2027-01-31", "2027-02-28", "2027-03-31", "2027-04-30", "2027-05-31"}
	if got := formatted(values); !reflect.DeepEqual(got, want) {
		t.Fatalf("dates=%v want=%v", got, want)
	}
}

func TestDatesYearlyLeapDayReturnsToLeapDay(t *testing.T) {
	values, err := Dates(
		mustDate(t, "2024-02-29"),
		mustDate(t, "2024-02-29"),
		mustDate(t, "2028-02-29"),
		Rule{Frequency: FrequencyYearly, Interval: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2024-02-29", "2025-02-28", "2026-02-28", "2027-02-28", "2028-02-29"}
	if got := formatted(values); !reflect.DeepEqual(got, want) {
		t.Fatalf("dates=%v want=%v", got, want)
	}
}

func TestDatesAvoidsHistoricalBackfillButKeepsAnchor(t *testing.T) {
	values, err := Dates(
		mustDate(t, "2020-01-01"),
		mustDate(t, "2026-08-20"),
		mustDate(t, "2026-08-25"),
		Rule{Frequency: FrequencyDaily, Interval: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2020-01-01", "2026-08-20", "2026-08-21", "2026-08-22", "2026-08-23", "2026-08-24", "2026-08-25"}
	if got := formatted(values); !reflect.DeepEqual(got, want) {
		t.Fatalf("dates=%v want=%v", got, want)
	}
}

func TestDatesKeepsFutureAnchorOutsideMaterializationWindow(t *testing.T) {
	values, err := Dates(
		mustDate(t, "2027-01-31"),
		mustDate(t, "2026-08-27"),
		mustDate(t, "2026-11-25"),
		Rule{Frequency: FrequencyMonthly, Interval: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2027-01-31"}
	if got := formatted(values); !reflect.DeepEqual(got, want) {
		t.Fatalf("dates=%v want=%v", got, want)
	}
}

func TestRuleValidation(t *testing.T) {
	anchor := mustDate(t, "2026-08-20")
	endsBefore := mustDate(t, "2026-08-19")
	for _, rule := range []Rule{
		{Frequency: "hourly", Interval: 1},
		{Frequency: FrequencyDaily, Interval: 0},
		{Frequency: FrequencyDaily, Interval: MaxInterval + 1},
		{Frequency: FrequencyDaily, Interval: 1, EndsOn: &endsBefore},
	} {
		if err := rule.Validate(anchor); !errors.Is(err, ErrInvalidRule) {
			t.Fatalf("rule=%#v error=%v", rule, err)
		}
	}
}
