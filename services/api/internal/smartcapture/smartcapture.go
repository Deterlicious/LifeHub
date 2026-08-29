package smartcapture

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/recurrence"
)

const MaxInputLength = 1000

var (
	ErrInvalidInput = errors.New("invalid smart capture input")
	moneyPattern    = regexp.MustCompile(`(?i)\b(\d+(?:[.,]\d+)?)\s*(ribu|rb|juta|jt)\b`)
	timePattern     = regexp.MustCompile(`(?i)\bjam\s+(\d{1,2})(?:[.:](\d{2}))?\s*(pagi|siang|sore|malam)?\b`)
	dayPattern      = regexp.MustCompile(`(?i)\btanggal\s+(\d{1,2})\b`)
	datePattern     = regexp.MustCompile(`(?i)\b(\d{1,2})\s+(januari|februari|maret|april|mei|juni|juli|agustus|september|oktober|november|desember)\s+(\d{4})\b`)
)

type RecurrenceDraft struct {
	Frequency string `json:"frequency"`
	Interval  int    `json:"interval"`
	EndsOn    string `json:"ends_on,omitempty"`
}

type Draft struct {
	Kind        string           `json:"kind"`
	Title       string           `json:"title"`
	Notes       string           `json:"notes,omitempty"`
	Priority    string           `json:"priority,omitempty"`
	DueLocal    string           `json:"due_local,omitempty"`
	AllDay      *bool            `json:"all_day,omitempty"`
	StartsLocal string           `json:"starts_local,omitempty"`
	EndsLocal   string           `json:"ends_local,omitempty"`
	StartsOn    string           `json:"starts_on,omitempty"`
	EndsOn      string           `json:"ends_on,omitempty"`
	Location    string           `json:"location,omitempty"`
	Amount      *int64           `json:"amount,omitempty"`
	Currency    string           `json:"currency,omitempty"`
	Name        string           `json:"name,omitempty"`
	Category    string           `json:"category,omitempty"`
	ExpiresOn   string           `json:"expires_on,omitempty"`
	Recurrence  *RecurrenceDraft `json:"recurrence,omitempty"`
	Confidence  float64          `json:"confidence"`
}

type Output struct {
	Draft       Draft    `json:"draft"`
	Ambiguities []string `json:"ambiguities"`
}

type Provider interface {
	Name() string
	Parse(ctx context.Context, input string, now time.Time, timezone string) (Output, error)
}

type RuleProvider struct{}

func (RuleProvider) Name() string { return "rule" }

func (RuleProvider) Parse(ctx context.Context, input string, now time.Time, timezone string) (Output, error) {
	if err := ctx.Err(); err != nil {
		return Output{}, err
	}
	input = strings.TrimSpace(input)
	if input == "" || len([]rune(input)) > MaxInputLength {
		return Output{}, ErrInvalidInput
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return Output{}, fmt.Errorf("load smart capture timezone: %w", err)
	}
	lower := strings.ToLower(input)
	output := Output{Ambiguities: make([]string, 0)}
	output.Draft.Kind = inferKind(lower)
	output.Draft.Confidence = 0.72
	if strings.Contains(lower, "prioritas tinggi") {
		output.Draft.Priority = domain.PriorityHigh
	} else if output.Draft.Kind == "task" {
		output.Draft.Priority = domain.PriorityNormal
	}

	date, dateKnown := parseDate(lower, now.In(location))
	hour, minute, timeKnown, timeAmbiguous := parseTime(lower)
	if timeAmbiguous {
		output.Ambiguities = append(output.Ambiguities, "Jam belum jelas pagi atau malam; periksa waktunya sebelum menyimpan.")
	}
	if recurrenceDraft := parseRecurrence(lower); recurrenceDraft != nil {
		output.Draft.Recurrence = recurrenceDraft
	}

	switch output.Draft.Kind {
	case "bill":
		output.Draft.Title = cleanTitle(input, "bill")
		output.Draft.Currency = "IDR"
		if amount, ok := parseMoney(lower); ok {
			output.Draft.Amount = &amount
		} else {
			output.Ambiguities = append(output.Ambiguities, "Nominal tagihan belum ditemukan.")
		}
		if !dateKnown {
			if day, ok := parseDayOfMonth(lower); ok {
				date = nextDayOfMonth(now.In(location), day)
				dateKnown = true
			}
		}
		if dateKnown && timeKnown {
			output.Draft.DueLocal = localDateTime(date, hour, minute)
		} else if !dateKnown {
			output.Ambiguities = append(output.Ambiguities, "Tanggal jatuh tempo belum ditemukan.")
		} else if !timeAmbiguous {
			output.Ambiguities = append(output.Ambiguities, "Jam jatuh tempo belum disebutkan.")
		}
	case "event":
		output.Draft.Title = cleanTitle(input, "event")
		allDay := dateKnown && !timeKnown && !timeAmbiguous && strings.Contains(lower, "seharian")
		output.Draft.AllDay = &allDay
		if allDay {
			output.Draft.StartsOn = date.Format(time.DateOnly)
		} else if dateKnown && timeKnown {
			output.Draft.StartsLocal = localDateTime(date, hour, minute)
		} else {
			output.Ambiguities = append(output.Ambiguities, "Tanggal dan jam mulai perlu dilengkapi.")
		}
	case "document":
		output.Draft.Name = cleanTitle(input, "document")
		output.Draft.Title = output.Draft.Name
		output.Draft.Category = inferDocumentCategory(lower)
		if dateKnown {
			output.Draft.ExpiresOn = date.Format(time.DateOnly)
		} else {
			output.Ambiguities = append(output.Ambiguities, "Tanggal kedaluwarsa belum ditemukan.")
		}
		if strings.Contains(lower, "ingatkan") {
			output.Ambiguities = append(output.Ambiguities, "Periksa lalu tambahkan pengingat setelah metadata dokumen disimpan.")
		}
	default:
		output.Draft.Title = cleanTitle(input, "task")
		if dateKnown && timeKnown {
			output.Draft.DueLocal = localDateTime(date, hour, minute)
		} else if (strings.Contains(lower, "besok") || strings.Contains(lower, "lusa")) && !timeAmbiguous {
			output.Ambiguities = append(output.Ambiguities, "Jam tenggat belum disebutkan.")
		}
	}
	if output.Draft.Title == "" {
		output.Draft.Title = "Perlu ditinjau"
		output.Ambiguities = append(output.Ambiguities, "Judul perlu diperjelas.")
	}
	if len(output.Ambiguities) == 0 {
		output.Draft.Confidence = 0.92
	}
	return output, nil
}

type MockProvider struct {
	Output Output
	Err    error
}

func (MockProvider) Name() string { return "mock" }

func (provider MockProvider) Parse(ctx context.Context, _ string, _ time.Time, _ string) (Output, error) {
	if err := ctx.Err(); err != nil {
		return Output{}, err
	}
	return provider.Output, provider.Err
}

func ValidateOutput(output Output) error {
	draft := output.Draft
	if strings.TrimSpace(draft.Title) == "" || len(draft.Title) > 200 || len(draft.Notes) > 5000 || draft.Confidence < 0 || draft.Confidence > 1 {
		return ErrInvalidInput
	}
	switch draft.Kind {
	case "task", "event", "bill", "document":
	default:
		return ErrInvalidInput
	}
	if draft.Amount != nil && (*draft.Amount < 1 || *draft.Amount > domain.MaxBillAmount) {
		return ErrInvalidInput
	}
	if draft.Recurrence != nil {
		anchor := time.Now().UTC()
		var endsOn *time.Time
		if draft.Recurrence.EndsOn != "" {
			parsed, err := time.Parse(time.DateOnly, draft.Recurrence.EndsOn)
			if err != nil || parsed.Format(time.DateOnly) != draft.Recurrence.EndsOn {
				return ErrInvalidInput
			}
			endsOn = &parsed
		}
		if err := (recurrence.Rule{Frequency: draft.Recurrence.Frequency, Interval: draft.Recurrence.Interval, EndsOn: endsOn}).Validate(anchor); err != nil {
			return ErrInvalidInput
		}
	}
	for _, value := range []string{draft.StartsOn, draft.EndsOn, draft.ExpiresOn} {
		if value == "" {
			continue
		}
		if parsed, err := time.Parse(time.DateOnly, value); err != nil || parsed.Format(time.DateOnly) != value {
			return ErrInvalidInput
		}
	}
	for _, value := range []string{draft.DueLocal, draft.StartsLocal, draft.EndsLocal} {
		if value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02T15:04:05", value); err != nil {
			return ErrInvalidInput
		}
	}
	return nil
}

func inferKind(lower string) string {
	if strings.Contains(lower, "bayar") || strings.Contains(lower, "tagihan") || moneyPattern.MatchString(lower) {
		return "bill"
	}
	if strings.Contains(lower, "meeting") || strings.Contains(lower, "rapat") || strings.Contains(lower, "jadwal") || strings.Contains(lower, "acara") {
		return "event"
	}
	if strings.Contains(lower, "habis") || strings.Contains(lower, "kedaluwarsa") || strings.Contains(lower, "expired") {
		return "document"
	}
	return "task"
}

func parseMoney(lower string) (int64, bool) {
	match := moneyPattern.FindStringSubmatch(lower)
	if len(match) == 0 {
		return 0, false
	}
	number, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", "."), 64)
	if err != nil {
		return 0, false
	}
	multiplier := float64(1_000)
	if match[2] == "juta" || match[2] == "jt" {
		multiplier = 1_000_000
	}
	amount := int64(number * multiplier)
	return amount, amount > 0 && amount <= domain.MaxBillAmount
}

func parseTime(lower string) (int, int, bool, bool) {
	match := timePattern.FindStringSubmatch(lower)
	if len(match) == 0 {
		return 0, 0, false, false
	}
	hour, _ := strconv.Atoi(match[1])
	minute := 0
	if match[2] != "" {
		minute, _ = strconv.Atoi(match[2])
	}
	if hour > 23 || minute > 59 {
		return 0, 0, false, true
	}
	period := match[3]
	if period == "" && hour > 0 && hour < 12 {
		return 0, 0, false, true
	}
	if period == "pagi" && hour == 12 {
		hour = 0
	}
	if (period == "siang" || period == "sore" || period == "malam") && hour < 12 {
		hour += 12
	}
	return hour, minute, true, false
}

func parseDate(lower string, localNow time.Time) (time.Time, bool) {
	base := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location())
	if strings.Contains(lower, "lusa") {
		return base.AddDate(0, 0, 2), true
	}
	if strings.Contains(lower, "besok") {
		return base.AddDate(0, 0, 1), true
	}
	match := datePattern.FindStringSubmatch(lower)
	if len(match) == 0 {
		return time.Time{}, false
	}
	day, _ := strconv.Atoi(match[1])
	year, _ := strconv.Atoi(match[3])
	months := map[string]time.Month{
		"januari": 1, "februari": 2, "maret": 3, "april": 4, "mei": 5, "juni": 6,
		"juli": 7, "agustus": 8, "september": 9, "oktober": 10, "november": 11, "desember": 12,
	}
	date := time.Date(year, months[match[2]], day, 0, 0, 0, 0, localNow.Location())
	if date.Day() != day || date.Month() != months[match[2]] {
		return time.Time{}, false
	}
	return date, true
}

func parseDayOfMonth(lower string) (int, bool) {
	match := dayPattern.FindStringSubmatch(lower)
	if len(match) == 0 {
		return 0, false
	}
	day, _ := strconv.Atoi(match[1])
	return day, day >= 1 && day <= 31
}

func nextDayOfMonth(localNow time.Time, day int) time.Time {
	year, month := localNow.Year(), localNow.Month()
	if localNow.Day() > day || day > daysInMonth(year, month) {
		month++
		if month > 12 {
			month = 1
			year++
		}
	}
	last := daysInMonth(year, month)
	if day > last {
		day = last
	}
	return time.Date(year, month, day, 0, 0, 0, 0, localNow.Location())
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func parseRecurrence(lower string) *RecurrenceDraft {
	frequency := ""
	switch {
	case strings.Contains(lower, "tiap hari") || strings.Contains(lower, "harian"):
		frequency = recurrence.FrequencyDaily
	case strings.Contains(lower, "tiap minggu") || strings.Contains(lower, "mingguan"):
		frequency = recurrence.FrequencyWeekly
	case strings.Contains(lower, "tiap bulan") || strings.Contains(lower, "bulanan"):
		frequency = recurrence.FrequencyMonthly
	case strings.Contains(lower, "tiap tahun") || strings.Contains(lower, "tahunan"):
		frequency = recurrence.FrequencyYearly
	}
	if frequency == "" {
		return nil
	}
	return &RecurrenceDraft{Frequency: frequency, Interval: 1}
}

func inferDocumentCategory(lower string) string {
	if strings.Contains(lower, "sim") || strings.Contains(lower, "izin") || strings.Contains(lower, "lisensi") {
		return "license"
	}
	if strings.Contains(lower, "asuransi") || strings.Contains(lower, "polis") {
		return "insurance"
	}
	return "other"
}

func localDateTime(date time.Time, hour, minute int) string {
	return fmt.Sprintf("%sT%02d:%02d:00", date.Format(time.DateOnly), hour, minute)
}

func cleanTitle(input, kind string) string {
	value := input
	patterns := []*regexp.Regexp{
		moneyPattern, timePattern, datePattern, dayPattern,
		regexp.MustCompile(`(?i)\b(besok|lusa|tiap\s+(hari|minggu|bulan|tahun)|harian|mingguan|bulanan|tahunan|prioritas\s+tinggi|seharian)\b`),
		regexp.MustCompile(`(?i)\bingatkan\s+\d+\s+hari\s+sebelumnya\b`),
	}
	for _, pattern := range patterns {
		value = pattern.ReplaceAllString(value, " ")
	}
	switch kind {
	case "bill":
		value = regexp.MustCompile(`(?i)^\s*(bayar|tagihan)\s+`).ReplaceAllString(value, "")
	case "event":
		value = regexp.MustCompile(`(?i)^\s*(jadwal|acara)\s+`).ReplaceAllString(value, "")
	case "document":
		value = regexp.MustCompile(`(?i)\b(habis|kedaluwarsa|expired)\b`).ReplaceAllString(value, " ")
	}
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}
	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
