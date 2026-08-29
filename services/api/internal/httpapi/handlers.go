package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"lifehub/services/api/internal/auth"
	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/store"
	"lifehub/services/api/internal/timeutil"
	"lifehub/services/api/internal/today"

	"github.com/go-chi/chi/v5"
)

func (api *API) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *API) ready(response http.ResponseWriter, request *http.Request) {
	ctx, cancel := contextWithTimeout(request, 2*time.Second)
	defer cancel()
	if err := api.store.Ready(ctx); err != nil {
		writeError(response, request, http.StatusServiceUnavailable, "NOT_READY", "Layanan belum siap.", nil)
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
}

func (api *API) devSession(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	token, err := api.devIssuer.Issue(input.Email)
	if err != nil {
		writeError(response, request, http.StatusBadRequest, "VALIDATION_ERROR", "Periksa data yang belum valid.", map[string]string{
			"email": "Email tidak valid.",
		})
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"access_token": token})
}

func (api *API) getProfile(response http.ResponseWriter, request *http.Request) {
	principal, _ := auth.PrincipalFromContext(request.Context())
	profile, err := api.store.GetProfile(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "get profile", err)
		return
	}
	writeJSON(response, http.StatusOK, profile)
}

func (api *API) patchProfile(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Timezone string `json:"timezone"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	input.Timezone = strings.TrimSpace(input.Timezone)
	if _, err := timeutil.LoadLocation(input.Timezone); err != nil {
		writeError(response, request, http.StatusBadRequest, "VALIDATION_ERROR", "Periksa data yang belum valid.", map[string]string{
			"timezone": "Gunakan zona waktu IANA yang valid.",
		})
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	profile, err := api.store.UpdateProfileTimezone(request.Context(), principal.UserID, input.Timezone)
	if err != nil {
		api.internalError(response, request, "update profile", err)
		return
	}
	writeJSON(response, http.StatusOK, profile)
}

const deleteDataConfirmation = "HAPUS DATA LIFEHUB"

func (api *API) deleteProfileData(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Confirmation string `json:"confirmation"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	if input.Confirmation != deleteDataConfirmation {
		writeValidationError(response, request, map[string]string{
			"confirmation": "Ketik HAPUS DATA LIFEHUB untuk mengonfirmasi.",
		})
		return
	}

	principal, _ := auth.PrincipalFromContext(request.Context())
	if err := api.store.DeleteUserData(request.Context(), principal.UserID); err != nil {
		api.internalError(response, request, "delete user data", err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (api *API) createTask(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Title      string           `json:"title"`
		Notes      *string          `json:"notes"`
		Priority   string           `json:"priority"`
		DueLocal   *string          `json:"due_local"`
		Recurrence *recurrenceInput `json:"recurrence"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	fields := validateTaskInput(&input.Title, &input.Notes, &input.Priority)
	if len(fields) > 0 {
		writeError(response, request, http.StatusBadRequest, "VALIDATION_ERROR", "Periksa data yang belum valid.", fields)
		return
	}

	principal, _ := auth.PrincipalFromContext(request.Context())
	profile, err := api.store.GetProfile(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "get task profile", err)
		return
	}
	if profile.Timezone == nil {
		writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu sebelum membuat tugas.", nil)
		return
	}

	location, err := timeutil.LoadLocation(*profile.Timezone)
	if err != nil {
		api.internalError(response, request, "load stored timezone", err)
		return
	}
	var dueAt *time.Time
	if input.DueLocal != nil {
		parsed, err := timeutil.ParseLocalWallTime(*input.DueLocal, location)
		if err != nil {
			message := "Waktu lokal tidak valid."
			if errors.Is(err, timeutil.ErrNonexistentTime) || errors.Is(err, timeutil.ErrAmbiguousLocalTime) {
				message = "Waktu ini ambigu atau tidak ada karena perubahan zona waktu."
			}
			writeError(response, request, http.StatusBadRequest, "VALIDATION_ERROR", "Periksa data yang belum valid.", map[string]string{
				"due_local": message,
			})
			return
		}
		dueAt = &parsed
	}
	var recurring *parsedRecurrence
	if input.Recurrence != nil {
		if dueAt == nil {
			writeValidationError(response, request, map[string]string{"due_local": "Tugas berulang memerlukan waktu jatuh tempo."})
			return
		}
		localDue := dueAt.In(location)
		anchorOn := time.Date(localDue.Year(), localDue.Month(), localDue.Day(), 0, 0, 0, 0, time.UTC)
		var recurrenceFields map[string]string
		recurring, recurrenceFields = validateRecurrenceInput(input.Recurrence, anchorOn)
		if len(recurrenceFields) > 0 {
			writeValidationError(response, request, recurrenceFields)
			return
		}
	}
	taskID, err := store.NewUUID()
	if err != nil {
		api.internalError(response, request, "generate task id", err)
		return
	}
	taskParams := domain.CreateTaskParams{
		ID:       taskID,
		UserID:   principal.UserID,
		Title:    input.Title,
		Notes:    input.Notes,
		Priority: input.Priority,
		DueAt:    dueAt,
	}
	var task domain.Task
	if recurring == nil {
		task, err = api.store.CreateTask(request.Context(), taskParams)
	} else {
		seriesID, idErr := store.NewUUID()
		if idErr != nil {
			api.internalError(response, request, "generate task recurrence id", idErr)
			return
		}
		localDue := dueAt.In(location)
		anchorOn := time.Date(localDue.Year(), localDue.Month(), localDue.Day(), 0, 0, 0, 0, time.UTC)
		fromOn, throughOn := recurrenceWindow(api.clock(), location)
		task, err = api.store.CreateRecurringTask(request.Context(), domain.CreateRecurringTaskParams{
			SeriesID: seriesID, Task: taskParams, Frequency: recurring.Frequency, Interval: recurring.Interval,
			AnchorOn: anchorOn, EndsOn: recurring.EndsOn, Timezone: *profile.Timezone,
			TimeLocal: localDue.Format("15:04:05"), FromOn: fromOn, ThroughOn: throughOn,
		})
	}
	if err != nil {
		api.internalError(response, request, "create task", err)
		return
	}
	writeJSON(response, http.StatusCreated, task)
}

func (api *API) completeTask(response http.ResponseWriter, request *http.Request) {
	taskID := chi.URLParam(request, "taskID")
	if !store.ValidUUID(taskID) {
		writeError(response, request, http.StatusNotFound, "NOT_FOUND", "Tugas tidak ditemukan.", nil)
		return
	}
	if err := decodeOptionalEmptyJSON(response, request); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	task, err := api.store.CompleteTask(request.Context(), principal.UserID, taskID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(response, request, http.StatusNotFound, "NOT_FOUND", "Tugas tidak ditemukan.", nil)
		return
	}
	if err != nil {
		api.internalError(response, request, "complete task", err)
		return
	}
	writeJSON(response, http.StatusOK, task)
}

type createEventInput struct {
	Title       string           `json:"title"`
	Notes       *string          `json:"notes"`
	Location    *string          `json:"location"`
	AllDay      *bool            `json:"all_day"`
	StartsLocal *string          `json:"starts_local"`
	EndsLocal   *string          `json:"ends_local"`
	StartsOn    *string          `json:"starts_on"`
	EndsOn      *string          `json:"ends_on"`
	Recurrence  *recurrenceInput `json:"recurrence"`
}

func (api *API) createEvent(response http.ResponseWriter, request *http.Request) {
	var input createEventInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	fields := validateEventInput(&input)
	if len(fields) > 0 {
		writeError(response, request, http.StatusBadRequest, "VALIDATION_ERROR", "Periksa data yang belum valid.", fields)
		return
	}

	principal, _ := auth.PrincipalFromContext(request.Context())
	profile, err := api.store.GetProfile(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "get event profile", err)
		return
	}
	if profile.Timezone == nil {
		writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu sebelum membuat jadwal.", nil)
		return
	}
	location, err := timeutil.LoadLocation(*profile.Timezone)
	if err != nil {
		api.internalError(response, request, "load stored timezone", err)
		return
	}

	params := domain.CreateEventParams{
		UserID:   principal.UserID,
		Title:    input.Title,
		Notes:    input.Notes,
		Location: input.Location,
		AllDay:   *input.AllDay,
		Timezone: *profile.Timezone,
	}
	if params.AllDay {
		startsOn, parseErr := time.Parse(time.DateOnly, strings.TrimSpace(*input.StartsOn))
		if parseErr != nil {
			fields["starts_on"] = "Gunakan tanggal dengan format YYYY-MM-DD."
		} else {
			params.StartsOn = &startsOn
		}
		if input.EndsOn != nil {
			endsOn, endErr := time.Parse(time.DateOnly, strings.TrimSpace(*input.EndsOn))
			if endErr != nil {
				fields["ends_on"] = "Gunakan tanggal dengan format YYYY-MM-DD."
			} else {
				params.EndsOn = &endsOn
			}
		}
		if params.StartsOn != nil && params.EndsOn != nil && params.EndsOn.Before(*params.StartsOn) {
			fields["ends_on"] = "Tanggal selesai tidak boleh sebelum tanggal mulai."
		}
	} else {
		startsAt, parseErr := timeutil.ParseLocalWallTime(*input.StartsLocal, location)
		if parseErr != nil {
			fields["starts_local"] = localWallTimeMessage(parseErr)
		} else {
			params.StartsAt = &startsAt
		}
		if input.EndsLocal != nil {
			endsAt, endErr := timeutil.ParseLocalWallTime(*input.EndsLocal, location)
			if endErr != nil {
				fields["ends_local"] = localWallTimeMessage(endErr)
			} else {
				params.EndsAt = &endsAt
			}
		}
		if params.StartsAt != nil && params.EndsAt != nil && !params.EndsAt.After(*params.StartsAt) {
			fields["ends_local"] = "Waktu selesai harus setelah waktu mulai."
		}
	}
	if len(fields) > 0 {
		writeError(response, request, http.StatusBadRequest, "VALIDATION_ERROR", "Periksa data yang belum valid.", fields)
		return
	}

	eventID, err := store.NewUUID()
	if err != nil {
		api.internalError(response, request, "generate event id", err)
		return
	}
	params.ID = eventID
	var event domain.Event
	if input.Recurrence == nil {
		event, err = api.store.CreateEvent(request.Context(), params)
	} else {
		var anchorOn time.Time
		var timeLocal *string
		var durationSeconds *int64
		allDaySpan := 0
		if params.AllDay {
			anchorOn = *params.StartsOn
			if params.EndsOn != nil {
				allDaySpan = int(params.EndsOn.Sub(*params.StartsOn) / (24 * time.Hour))
			}
		} else {
			localStart := params.StartsAt.In(location)
			anchorOn = time.Date(localStart.Year(), localStart.Month(), localStart.Day(), 0, 0, 0, 0, time.UTC)
			value := localStart.Format("15:04:05")
			timeLocal = &value
			if params.EndsAt != nil {
				value := int64(params.EndsAt.Sub(*params.StartsAt) / time.Second)
				durationSeconds = &value
			}
		}
		recurring, recurrenceFields := validateRecurrenceInput(input.Recurrence, anchorOn)
		if len(recurrenceFields) > 0 {
			writeValidationError(response, request, recurrenceFields)
			return
		}
		seriesID, idErr := store.NewUUID()
		if idErr != nil {
			api.internalError(response, request, "generate event recurrence id", idErr)
			return
		}
		fromOn, throughOn := recurrenceWindow(api.clock(), location)
		event, err = api.store.CreateRecurringEvent(request.Context(), domain.CreateRecurringEventParams{
			SeriesID: seriesID, Event: params, Frequency: recurring.Frequency, Interval: recurring.Interval,
			AnchorOn: anchorOn, EndsOn: recurring.EndsOn, FromOn: fromOn, ThroughOn: throughOn,
			TimeLocal: timeLocal, DurationSeconds: durationSeconds, AllDaySpan: allDaySpan,
		})
	}
	if err != nil {
		api.internalError(response, request, "create event", err)
		return
	}
	writeJSON(response, http.StatusCreated, event)
}

func (api *API) getToday(response http.ResponseWriter, request *http.Request) {
	principal, _ := auth.PrincipalFromContext(request.Context())
	profile, err := api.store.GetProfile(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "get today profile", err)
		return
	}
	if profile.Timezone == nil {
		writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu untuk membuka Today.", nil)
		return
	}
	location, err := timeutil.LoadLocation(*profile.Timezone)
	if err != nil {
		api.internalError(response, request, "load stored timezone", err)
		return
	}
	now := api.clock()
	date, start, end, err := timeutil.LocalDayWindow(now, location)
	if err != nil {
		api.internalError(response, request, "resolve today date window", err)
		return
	}
	localDate, err := time.Parse(time.DateOnly, date)
	if err != nil {
		api.internalError(response, request, "parse today date", err)
		return
	}
	horizonLocalDate := localDate.AddDate(0, 0, 30)
	horizonDate := horizonLocalDate.Format(time.DateOnly)
	if err := api.store.MaterializeRecurrences(request.Context(), principal.UserID, localDate, localDate.AddDate(0, 0, 90)); err != nil {
		api.internalError(response, request, "materialize Today recurrences", err)
		return
	}
	horizonEnd, err := timeutil.LocalDateEnd(horizonLocalDate, location)
	if err != nil {
		api.internalError(response, request, "resolve today horizon", err)
		return
	}
	tasks, err := api.store.ListTodayTasks(request.Context(), principal.UserID, start, end, horizonEnd)
	if err != nil {
		api.internalError(response, request, "list today tasks", err)
		return
	}
	events, err := api.store.ListTodayEvents(request.Context(), principal.UserID, date, horizonDate, start, horizonEnd)
	if err != nil {
		api.internalError(response, request, "list today events", err)
		return
	}
	bills, err := api.store.ListTodayBills(request.Context(), principal.UserID, start, end, horizonEnd)
	if err != nil {
		api.internalError(response, request, "list today bills", err)
		return
	}
	documents, err := api.store.ListTodayDocuments(request.Context(), principal.UserID, horizonDate)
	if err != nil {
		api.internalError(response, request, "list today documents", err)
		return
	}
	writeJSON(response, http.StatusOK, today.Build(date, horizonDate, *profile.Timezone, now, start, end, horizonEnd, location, tasks, events, bills, documents))
}

func validateEventInput(input *createEventInput) map[string]string {
	fields := make(map[string]string)
	input.Title = strings.TrimSpace(input.Title)
	if utf8.RuneCountInString(input.Title) == 0 || utf8.RuneCountInString(input.Title) > 200 {
		fields["title"] = "Judul wajib diisi dan maksimal 200 karakter."
	}
	normalizeOptionalText(&input.Notes, 5000, "notes", "Catatan maksimal 5000 karakter.", fields)
	normalizeOptionalText(&input.Location, 500, "location", "Lokasi maksimal 500 karakter.", fields)
	if input.AllDay == nil {
		fields["all_day"] = "Tentukan apakah jadwal berlangsung sepanjang hari."
		return fields
	}
	if *input.AllDay {
		if input.StartsLocal != nil {
			fields["starts_local"] = "Tidak digunakan untuk jadwal sepanjang hari."
		}
		if input.EndsLocal != nil {
			fields["ends_local"] = "Tidak digunakan untuk jadwal sepanjang hari."
		}
		if input.StartsOn == nil {
			fields["starts_on"] = "Tanggal mulai wajib diisi."
		}
	} else {
		if input.StartsOn != nil {
			fields["starts_on"] = "Tidak digunakan untuk jadwal dengan waktu."
		}
		if input.EndsOn != nil {
			fields["ends_on"] = "Tidak digunakan untuk jadwal dengan waktu."
		}
		if input.StartsLocal == nil {
			fields["starts_local"] = "Waktu mulai wajib diisi."
		}
	}
	return fields
}

func normalizeOptionalText(value **string, maximum int, field, message string, fields map[string]string) {
	if *value == nil {
		return
	}
	trimmed := strings.TrimSpace(**value)
	if trimmed == "" {
		*value = nil
		return
	}
	if utf8.RuneCountInString(trimmed) > maximum {
		fields[field] = message
		return
	}
	*value = &trimmed
}

func localWallTimeMessage(err error) string {
	if errors.Is(err, timeutil.ErrNonexistentTime) || errors.Is(err, timeutil.ErrAmbiguousLocalTime) {
		return "Waktu ini ambigu atau tidak ada karena perubahan zona waktu."
	}
	return "Waktu lokal tidak valid."
}

type createBillInput struct {
	Title      string           `json:"title"`
	Notes      *string          `json:"notes"`
	Amount     *int64           `json:"amount"`
	Currency   *string          `json:"currency"`
	DueLocal   *string          `json:"due_local"`
	Recurrence *recurrenceInput `json:"recurrence"`
}

func (api *API) createBill(response http.ResponseWriter, request *http.Request) {
	var input createBillInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	fields := validateBillInput(&input)
	if len(fields) > 0 {
		writeError(response, request, http.StatusBadRequest, "VALIDATION_ERROR", "Periksa data yang belum valid.", fields)
		return
	}

	principal, _ := auth.PrincipalFromContext(request.Context())
	profile, err := api.store.GetProfile(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "get bill profile", err)
		return
	}
	if profile.Timezone == nil {
		writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu sebelum membuat tagihan.", nil)
		return
	}
	location, err := timeutil.LoadLocation(*profile.Timezone)
	if err != nil {
		api.internalError(response, request, "load stored timezone", err)
		return
	}
	dueAt, err := timeutil.ParseLocalWallTime(*input.DueLocal, location)
	if err != nil {
		writeError(response, request, http.StatusBadRequest, "VALIDATION_ERROR", "Periksa data yang belum valid.", map[string]string{
			"due_local": localWallTimeMessage(err),
		})
		return
	}

	billID, err := store.NewUUID()
	if err != nil {
		api.internalError(response, request, "generate bill id", err)
		return
	}
	currency := "IDR"
	if input.Currency != nil {
		currency = *input.Currency
	}
	billParams := domain.CreateBillParams{
		ID:       billID,
		UserID:   principal.UserID,
		Title:    input.Title,
		Notes:    input.Notes,
		Amount:   *input.Amount,
		Currency: currency,
		DueAt:    dueAt,
	}
	var bill domain.Bill
	if input.Recurrence == nil {
		bill, err = api.store.CreateBill(request.Context(), billParams)
	} else {
		localDue := dueAt.In(location)
		anchorOn := time.Date(localDue.Year(), localDue.Month(), localDue.Day(), 0, 0, 0, 0, time.UTC)
		recurring, recurrenceFields := validateRecurrenceInput(input.Recurrence, anchorOn)
		if len(recurrenceFields) > 0 {
			writeValidationError(response, request, recurrenceFields)
			return
		}
		seriesID, idErr := store.NewUUID()
		if idErr != nil {
			api.internalError(response, request, "generate bill recurrence id", idErr)
			return
		}
		fromOn, throughOn := recurrenceWindow(api.clock(), location)
		bill, err = api.store.CreateRecurringBill(request.Context(), domain.CreateRecurringBillParams{
			SeriesID: seriesID, Bill: billParams, Frequency: recurring.Frequency, Interval: recurring.Interval,
			AnchorOn: anchorOn, EndsOn: recurring.EndsOn, Timezone: *profile.Timezone,
			TimeLocal: localDue.Format("15:04:05"), FromOn: fromOn, ThroughOn: throughOn,
		})
	}
	if err != nil {
		api.internalError(response, request, "create bill", err)
		return
	}
	writeJSON(response, http.StatusCreated, bill)
}

func (api *API) markBillPaid(response http.ResponseWriter, request *http.Request) {
	billID := chi.URLParam(request, "billID")
	if !store.ValidUUID(billID) {
		writeError(response, request, http.StatusNotFound, "NOT_FOUND", "Tagihan tidak ditemukan.", nil)
		return
	}
	if err := decodeOptionalEmptyJSON(response, request); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	bill, err := api.store.MarkBillPaid(request.Context(), principal.UserID, billID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(response, request, http.StatusNotFound, "NOT_FOUND", "Tagihan tidak ditemukan.", nil)
		return
	}
	if err != nil {
		api.internalError(response, request, "mark bill paid", err)
		return
	}
	writeJSON(response, http.StatusOK, bill)
}

func validateBillInput(input *createBillInput) map[string]string {
	fields := make(map[string]string)
	input.Title = strings.TrimSpace(input.Title)
	if utf8.RuneCountInString(input.Title) == 0 || utf8.RuneCountInString(input.Title) > 200 {
		fields["title"] = "Judul wajib diisi dan maksimal 200 karakter."
	}
	normalizeOptionalText(&input.Notes, 5000, "notes", "Catatan maksimal 5000 karakter.", fields)
	if input.Amount == nil || *input.Amount < 1 || *input.Amount > domain.MaxBillAmount {
		fields["amount"] = "Nominal harus berupa bilangan bulat antara 1 dan 9007199254740991."
	}
	if input.Currency != nil {
		currency := strings.TrimSpace(*input.Currency)
		if !validCurrency(currency) {
			fields["currency"] = "Mata uang harus terdiri dari tiga huruf kapital, misalnya IDR."
		} else {
			input.Currency = &currency
		}
	}
	if input.DueLocal == nil {
		fields["due_local"] = "Waktu jatuh tempo wajib diisi."
	}
	return fields
}

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for index := range len(value) {
		if value[index] < 'A' || value[index] > 'Z' {
			return false
		}
	}
	return true
}

type createDocumentInput struct {
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Notes     *string `json:"notes"`
	ExpiresOn *string `json:"expires_on"`
}

type patchStringField struct {
	Set   bool
	Null  bool
	Value string
}

func (field *patchStringField) UnmarshalJSON(data []byte) error {
	field.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		field.Null = true
		field.Value = ""
		return nil
	}
	field.Null = false
	return json.Unmarshal(data, &field.Value)
}

type patchDocumentInput struct {
	Name      patchStringField `json:"name"`
	Category  patchStringField `json:"category"`
	Notes     patchStringField `json:"notes"`
	ExpiresOn patchStringField `json:"expires_on"`
}

func (api *API) createDocument(response http.ResponseWriter, request *http.Request) {
	var input createDocumentInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	expiresOn, fields := validateCreateDocumentInput(&input)
	if len(fields) > 0 {
		writeError(response, request, http.StatusBadRequest, "VALIDATION_ERROR", "Periksa data yang belum valid.", fields)
		return
	}

	principal, _ := auth.PrincipalFromContext(request.Context())
	profile, err := api.store.GetProfile(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "get document profile", err)
		return
	}
	if profile.Timezone == nil {
		writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu sebelum membuat dokumen.", nil)
		return
	}
	localDate, err := api.documentLocalDate(profile)
	if err != nil {
		api.internalError(response, request, "load document timezone", err)
		return
	}
	documentID, err := store.NewUUID()
	if err != nil {
		api.internalError(response, request, "generate document id", err)
		return
	}
	document, err := api.store.CreateDocument(request.Context(), domain.CreateDocumentParams{
		ID: documentID, UserID: principal.UserID, Name: input.Name, Category: input.Category,
		Notes: input.Notes, ExpiresOn: expiresOn,
	})
	if err != nil {
		api.internalError(response, request, "create document", err)
		return
	}
	writeJSON(response, http.StatusCreated, enrichDocument(document, localDate))
}

func (api *API) listDocuments(response http.ResponseWriter, request *http.Request) {
	principal, _ := auth.PrincipalFromContext(request.Context())
	profile, err := api.store.GetProfile(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "get document profile", err)
		return
	}
	if profile.Timezone == nil {
		writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu untuk melihat dokumen.", nil)
		return
	}
	localDate, err := api.documentLocalDate(profile)
	if err != nil {
		api.internalError(response, request, "load document timezone", err)
		return
	}
	documents, err := api.store.ListDocuments(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "list documents", err)
		return
	}
	for index := range documents {
		documents[index] = enrichDocument(documents[index], localDate)
	}
	writeJSON(response, http.StatusOK, documents)
}

func (api *API) getDocument(response http.ResponseWriter, request *http.Request) {
	documentID := chi.URLParam(request, "documentID")
	if !store.ValidUUID(documentID) {
		writeError(response, request, http.StatusNotFound, "NOT_FOUND", "Dokumen tidak ditemukan.", nil)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	profile, err := api.store.GetProfile(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "get document profile", err)
		return
	}
	if profile.Timezone == nil {
		writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu untuk melihat dokumen.", nil)
		return
	}
	localDate, err := api.documentLocalDate(profile)
	if err != nil {
		api.internalError(response, request, "load document timezone", err)
		return
	}
	document, err := api.store.GetDocument(request.Context(), principal.UserID, documentID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(response, request, http.StatusNotFound, "NOT_FOUND", "Dokumen tidak ditemukan.", nil)
		return
	}
	if err != nil {
		api.internalError(response, request, "get document", err)
		return
	}
	writeJSON(response, http.StatusOK, enrichDocument(document, localDate))
}

func (api *API) patchDocument(response http.ResponseWriter, request *http.Request) {
	documentID := chi.URLParam(request, "documentID")
	if !store.ValidUUID(documentID) {
		writeError(response, request, http.StatusNotFound, "NOT_FOUND", "Dokumen tidak ditemukan.", nil)
		return
	}
	var input patchDocumentInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	params, fields := validatePatchDocumentInput(&input)
	if len(fields) > 0 {
		writeError(response, request, http.StatusBadRequest, "VALIDATION_ERROR", "Periksa data yang belum valid.", fields)
		return
	}

	principal, _ := auth.PrincipalFromContext(request.Context())
	profile, err := api.store.GetProfile(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "get document profile", err)
		return
	}
	if profile.Timezone == nil {
		writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu sebelum mengubah dokumen.", nil)
		return
	}
	localDate, err := api.documentLocalDate(profile)
	if err != nil {
		api.internalError(response, request, "load document timezone", err)
		return
	}
	params.ID = documentID
	params.UserID = principal.UserID
	document, err := api.store.UpdateDocument(request.Context(), params)
	if errors.Is(err, store.ErrNotFound) {
		writeError(response, request, http.StatusNotFound, "NOT_FOUND", "Dokumen tidak ditemukan.", nil)
		return
	}
	if err != nil {
		api.internalError(response, request, "update document", err)
		return
	}
	writeJSON(response, http.StatusOK, enrichDocument(document, localDate))
}

func (api *API) deleteDocument(response http.ResponseWriter, request *http.Request) {
	documentID := chi.URLParam(request, "documentID")
	if !store.ValidUUID(documentID) {
		writeError(response, request, http.StatusNotFound, "NOT_FOUND", "Dokumen tidak ditemukan.", nil)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	if err := api.store.DeleteDocument(request.Context(), principal.UserID, documentID); errors.Is(err, store.ErrNotFound) {
		writeError(response, request, http.StatusNotFound, "NOT_FOUND", "Dokumen tidak ditemukan.", nil)
		return
	} else if err != nil {
		api.internalError(response, request, "delete document", err)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func validateCreateDocumentInput(input *createDocumentInput) (time.Time, map[string]string) {
	fields := make(map[string]string)
	input.Name = strings.TrimSpace(input.Name)
	if utf8.RuneCountInString(input.Name) == 0 || utf8.RuneCountInString(input.Name) > 200 {
		fields["name"] = "Nama wajib diisi dan maksimal 200 karakter."
	}
	input.Category = strings.TrimSpace(input.Category)
	if !validDocumentCategory(input.Category) {
		fields["category"] = "Kategori harus identity, license, insurance, education, work, atau other."
	}
	normalizeOptionalText(&input.Notes, 5000, "notes", "Catatan maksimal 5000 karakter.", fields)
	var expiresOn time.Time
	if input.ExpiresOn == nil {
		fields["expires_on"] = "Tanggal kedaluwarsa wajib diisi."
	} else {
		parsed, err := time.Parse(time.DateOnly, strings.TrimSpace(*input.ExpiresOn))
		if err != nil {
			fields["expires_on"] = "Gunakan tanggal dengan format YYYY-MM-DD."
		} else {
			expiresOn = parsed
		}
	}
	return expiresOn, fields
}

func validatePatchDocumentInput(input *patchDocumentInput) (domain.UpdateDocumentParams, map[string]string) {
	fields := make(map[string]string)
	params := domain.UpdateDocumentParams{}
	if !input.Name.Set && !input.Category.Set && !input.Notes.Set && !input.ExpiresOn.Set {
		fields["body"] = "Kirim setidaknya satu field yang ingin diubah."
		return params, fields
	}
	if input.Name.Set {
		if input.Name.Null {
			fields["name"] = "Nama tidak boleh null."
		} else {
			value := strings.TrimSpace(input.Name.Value)
			if utf8.RuneCountInString(value) == 0 || utf8.RuneCountInString(value) > 200 {
				fields["name"] = "Nama wajib diisi dan maksimal 200 karakter."
			} else {
				params.Name = &value
			}
		}
	}
	if input.Category.Set {
		if input.Category.Null {
			fields["category"] = "Kategori tidak boleh null."
		} else {
			value := strings.TrimSpace(input.Category.Value)
			if !validDocumentCategory(value) {
				fields["category"] = "Kategori harus identity, license, insurance, education, work, atau other."
			} else {
				params.Category = &value
			}
		}
	}
	if input.Notes.Set {
		params.NotesSet = true
		if !input.Notes.Null {
			value := strings.TrimSpace(input.Notes.Value)
			if utf8.RuneCountInString(value) > 5000 {
				fields["notes"] = "Catatan maksimal 5000 karakter."
			} else if value != "" {
				params.Notes = &value
			}
		}
	}
	if input.ExpiresOn.Set {
		if input.ExpiresOn.Null {
			fields["expires_on"] = "Tanggal kedaluwarsa tidak boleh null."
		} else {
			value, err := time.Parse(time.DateOnly, strings.TrimSpace(input.ExpiresOn.Value))
			if err != nil {
				fields["expires_on"] = "Gunakan tanggal dengan format YYYY-MM-DD."
			} else {
				params.ExpiresOn = &value
			}
		}
	}
	return params, fields
}

func validDocumentCategory(value string) bool {
	switch value {
	case "identity", "license", "insurance", "education", "work", "other":
		return true
	default:
		return false
	}
}

func (api *API) documentLocalDate(profile domain.Profile) (time.Time, error) {
	if profile.Timezone == nil {
		return time.Time{}, errors.New("document profile timezone is incomplete")
	}
	location, err := timeutil.LoadLocation(*profile.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	local := api.clock().In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC), nil
}

func enrichDocument(document domain.Document, localDate time.Time) domain.Document {
	expiresOn, err := time.Parse(time.DateOnly, document.ExpiresOn)
	if err != nil {
		return document
	}
	document.DaysUntilExpiry = int(expiresOn.Unix()/86400 - localDate.Unix()/86400)
	switch {
	case document.DaysUntilExpiry < 0:
		document.Status = "expired"
	case document.DaysUntilExpiry <= 30:
		document.Status = "expiring"
	default:
		document.Status = "valid"
	}
	return document
}

func validateTaskInput(title *string, notes **string, priority *string) map[string]string {
	fields := make(map[string]string)
	*title = strings.TrimSpace(*title)
	if utf8.RuneCountInString(*title) == 0 || utf8.RuneCountInString(*title) > 200 {
		fields["title"] = "Judul wajib diisi dan maksimal 200 karakter."
	}
	if *notes != nil {
		value := strings.TrimSpace(**notes)
		if value == "" {
			*notes = nil
		} else if utf8.RuneCountInString(value) > 5000 {
			fields["notes"] = "Catatan maksimal 5000 karakter."
		} else {
			*notes = &value
		}
	}
	*priority = strings.TrimSpace(*priority)
	if *priority == "" {
		*priority = domain.PriorityNormal
	}
	if *priority != domain.PriorityLow && *priority != domain.PriorityNormal && *priority != domain.PriorityHigh {
		fields["priority"] = "Prioritas harus low, normal, atau high."
	}
	return fields
}

func writeDecodeError(response http.ResponseWriter, request *http.Request, err error) {
	if isBodyTooLarge(err) {
		writeError(response, request, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Data terlalu besar.", nil)
		return
	}
	fields := map[string]string{"body": "Gunakan JSON yang valid."}
	if isUnknownField(err) {
		fields["body"] = "Terdapat field yang tidak dikenal."
	}
	writeError(response, request, http.StatusBadRequest, "VALIDATION_ERROR", "Periksa data yang belum valid.", fields)
}

func (api *API) internalError(response http.ResponseWriter, request *http.Request, operation string, err error) {
	if errors.Is(err, context.Canceled) {
		api.logger.Info("request canceled by client",
			"request_id", requestIDFromContext(request.Context()),
			"operation", operation,
		)
		response.WriteHeader(statusClientClosedRequest)
		return
	}
	api.logger.Error("request failed",
		"request_id", requestIDFromContext(request.Context()),
		"operation", operation,
		"error_type", fmt.Sprintf("%T", err),
	)
	writeError(response, request, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan. Coba lagi.", nil)
}

func contextWithTimeout(request *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(request.Context(), timeout)
}
