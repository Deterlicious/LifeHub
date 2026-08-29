package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"lifehub/services/api/internal/auth"
	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/recurrence"
	"lifehub/services/api/internal/store"
	"lifehub/services/api/internal/timeutil"

	"github.com/go-chi/chi/v5"
)

type recurrenceInput struct {
	Frequency string  `json:"frequency"`
	Interval  *int64  `json:"interval"`
	EndsOn    *string `json:"ends_on"`
}

type parsedRecurrence struct {
	Frequency string
	Interval  int
	EndsOn    *time.Time
}

func validateRecurrenceInput(input *recurrenceInput, anchorOn time.Time) (*parsedRecurrence, map[string]string) {
	fields := make(map[string]string)
	if input == nil {
		return nil, fields
	}
	input.Frequency = strings.TrimSpace(input.Frequency)
	interval := 1
	if input.Interval != nil {
		if *input.Interval < 1 || *input.Interval > recurrence.MaxInterval {
			fields["recurrence.interval"] = "Interval harus bilangan bulat antara 1 dan 365."
		} else {
			interval = int(*input.Interval)
		}
	}
	var endsOn *time.Time
	if input.EndsOn != nil {
		trimmed := strings.TrimSpace(*input.EndsOn)
		parsed, err := time.Parse(time.DateOnly, trimmed)
		if err != nil || parsed.Format(time.DateOnly) != trimmed {
			fields["recurrence.ends_on"] = "Gunakan tanggal akhir dengan format YYYY-MM-DD."
		} else {
			endsOn = &parsed
			input.EndsOn = &trimmed
		}
	}
	rule := recurrence.Rule{Frequency: input.Frequency, Interval: interval, EndsOn: endsOn}
	if err := rule.Validate(anchorOn); err != nil {
		if input.Frequency != recurrence.FrequencyDaily && input.Frequency != recurrence.FrequencyWeekly && input.Frequency != recurrence.FrequencyMonthly && input.Frequency != recurrence.FrequencyYearly {
			fields["recurrence.frequency"] = "Frekuensi harus daily, weekly, monthly, atau yearly."
		} else if endsOn != nil && endsOn.Before(anchorOn) {
			fields["recurrence.ends_on"] = "Tanggal akhir tidak boleh sebelum kejadian pertama."
		}
	}
	if len(fields) > 0 {
		return nil, fields
	}
	return &parsedRecurrence{Frequency: input.Frequency, Interval: interval, EndsOn: endsOn}, fields
}

func recurrenceWindow(now time.Time, location *time.Location) (time.Time, time.Time) {
	local := now.In(location)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	return today, today.AddDate(0, 0, 90)
}

func (api *API) listRecurrenceSeries(response http.ResponseWriter, request *http.Request) {
	if field, ok := invalidQuery(request, map[string]struct{}{}); !ok {
		writeValidationError(response, request, map[string]string{field: "Parameter query tidak valid."})
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	items, err := api.store.ListRecurrenceSeries(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "list recurrence series", err)
		return
	}
	writeJSON(response, http.StatusOK, struct {
		Items []domain.RecurrenceSeries `json:"items"`
	}{Items: items})
}

func (api *API) getRecurrenceSeries(response http.ResponseWriter, request *http.Request) {
	seriesID := chi.URLParam(request, "seriesID")
	if !store.ValidUUID(seriesID) {
		writeResourceNotFound(response, request, "Seri berulang tidak ditemukan.")
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	item, err := api.store.GetRecurrenceSeries(request.Context(), principal.UserID, seriesID)
	if errors.Is(err, store.ErrNotFound) {
		writeResourceNotFound(response, request, "Seri berulang tidak ditemukan.")
		return
	}
	if err != nil {
		api.internalError(response, request, "get recurrence series", err)
		return
	}
	writeJSON(response, http.StatusOK, item)
}

func (api *API) patchRecurrenceSeries(response http.ResponseWriter, request *http.Request) {
	seriesID := chi.URLParam(request, "seriesID")
	if !store.ValidUUID(seriesID) {
		writeResourceNotFound(response, request, "Seri berulang tidak ditemukan.")
		return
	}
	var input recurrenceInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	current, err := api.store.GetRecurrenceSeries(request.Context(), principal.UserID, seriesID)
	if errors.Is(err, store.ErrNotFound) {
		writeResourceNotFound(response, request, "Seri berulang tidak ditemukan.")
		return
	}
	if err != nil {
		api.internalError(response, request, "get recurrence series for update", err)
		return
	}
	anchorOn, err := time.Parse(time.DateOnly, current.AnchorOn)
	if err != nil {
		api.internalError(response, request, "parse recurrence anchor", err)
		return
	}
	parsed, fields := validateRecurrenceInput(&input, anchorOn)
	if len(fields) > 0 {
		writeValidationError(response, request, fields)
		return
	}
	location, err := timeutil.LoadLocation(current.Timezone)
	if err != nil {
		api.internalError(response, request, "load recurrence timezone", err)
		return
	}
	fromOn, throughOn := recurrenceWindow(api.clock(), location)
	item, err := api.store.UpdateRecurrenceSeries(request.Context(), domain.UpdateRecurrenceSeriesParams{
		ID: seriesID, UserID: principal.UserID, Frequency: parsed.Frequency, Interval: parsed.Interval,
		EndsOn: parsed.EndsOn, FromOn: fromOn, ThroughOn: throughOn,
	})
	if errors.Is(err, store.ErrNotFound) {
		writeResourceNotFound(response, request, "Seri berulang tidak ditemukan.")
		return
	}
	if errors.Is(err, store.ErrRecurrenceInactive) {
		writeError(response, request, http.StatusConflict, "RECURRENCE_INACTIVE", "Seri berulang ini sudah dihentikan.", nil)
		return
	}
	if err != nil {
		api.internalError(response, request, "update recurrence series", err)
		return
	}
	writeJSON(response, http.StatusOK, item)
}

func (api *API) stopRecurrenceSeries(response http.ResponseWriter, request *http.Request) {
	seriesID := chi.URLParam(request, "seriesID")
	if !store.ValidUUID(seriesID) {
		writeResourceNotFound(response, request, "Seri berulang tidak ditemukan.")
		return
	}
	if err := decodeOptionalEmptyJSON(response, request); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	profile, err := api.store.GetProfile(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "get recurrence profile", err)
		return
	}
	if profile.Timezone == nil {
		writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu untuk menghentikan seri.", nil)
		return
	}
	location, err := timeutil.LoadLocation(*profile.Timezone)
	if err != nil {
		api.internalError(response, request, "load recurrence profile timezone", err)
		return
	}
	today, _ := recurrenceWindow(api.clock(), location)
	err = api.store.StopRecurrenceSeries(request.Context(), principal.UserID, seriesID, today)
	if errors.Is(err, store.ErrNotFound) {
		writeResourceNotFound(response, request, "Seri berulang tidak ditemukan.")
		return
	}
	if err != nil {
		api.internalError(response, request, "stop recurrence series", err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}
