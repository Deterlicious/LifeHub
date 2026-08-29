package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"lifehub/services/api/internal/agenda"
	"lifehub/services/api/internal/auth"
	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/store"
	"lifehub/services/api/internal/timeutil"

	"github.com/go-chi/chi/v5"
)

type patchBoolField struct {
	Set   bool
	Null  bool
	Value bool
}

func (field *patchBoolField) UnmarshalJSON(data []byte) error {
	field.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		field.Null = true
		field.Value = false
		return nil
	}
	field.Null = false
	return json.Unmarshal(data, &field.Value)
}

type patchInt64Field struct {
	Set   bool
	Null  bool
	Value int64
}

func (field *patchInt64Field) UnmarshalJSON(data []byte) error {
	field.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		field.Null = true
		field.Value = 0
		return nil
	}
	field.Null = false
	return json.Unmarshal(data, &field.Value)
}

type patchTaskInput struct {
	Title    patchStringField `json:"title"`
	Notes    patchStringField `json:"notes"`
	Priority patchStringField `json:"priority"`
	DueLocal patchStringField `json:"due_local"`
}

func (api *API) getTask(response http.ResponseWriter, request *http.Request) {
	taskID := chi.URLParam(request, "taskID")
	if !store.ValidUUID(taskID) {
		writeResourceNotFound(response, request, "Tugas tidak ditemukan.")
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	task, err := api.store.GetTask(request.Context(), principal.UserID, taskID)
	if writeStoreResultError(api, response, request, "get task", "Tugas tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusOK, task)
}

func (api *API) patchTask(response http.ResponseWriter, request *http.Request) {
	taskID := chi.URLParam(request, "taskID")
	if !store.ValidUUID(taskID) {
		writeResourceNotFound(response, request, "Tugas tidak ditemukan.")
		return
	}
	var input patchTaskInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	params, fields := validatePatchTaskInput(&input)
	if len(fields) > 0 {
		writeValidationError(response, request, fields)
		return
	}

	principal, _ := auth.PrincipalFromContext(request.Context())
	if _, err := api.store.GetTask(request.Context(), principal.UserID, taskID); err != nil {
		if writeStoreResultError(api, response, request, "get task before update", "Tugas tidak ditemukan.", err) {
			return
		}
	}
	if input.DueLocal.Set && !input.DueLocal.Null {
		profile, err := api.store.GetProfile(request.Context(), principal.UserID)
		if err != nil {
			api.internalError(response, request, "get task profile", err)
			return
		}
		if profile.Timezone == nil {
			writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu sebelum mengubah jadwal tugas.", nil)
			return
		}
		location, err := timeutil.LoadLocation(*profile.Timezone)
		if err != nil {
			api.internalError(response, request, "load stored timezone", err)
			return
		}
		dueAt, err := timeutil.ParseLocalWallTime(input.DueLocal.Value, location)
		if err != nil {
			writeValidationError(response, request, map[string]string{"due_local": localWallTimeMessage(err)})
			return
		}
		params.DueAt = &dueAt
	}
	params.ID = taskID
	params.UserID = principal.UserID
	task, err := api.store.UpdateTask(request.Context(), params)
	if writeStoreResultError(api, response, request, "update task", "Tugas tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusOK, task)
}

func validatePatchTaskInput(input *patchTaskInput) (domain.UpdateTaskParams, map[string]string) {
	params := domain.UpdateTaskParams{}
	fields := make(map[string]string)
	if !input.Title.Set && !input.Notes.Set && !input.Priority.Set && !input.DueLocal.Set {
		fields["body"] = "Kirim setidaknya satu field yang ingin diubah."
		return params, fields
	}
	if input.Title.Set {
		if input.Title.Null {
			fields["title"] = "Judul tidak boleh null."
		} else {
			value := strings.TrimSpace(input.Title.Value)
			if utf8.RuneCountInString(value) == 0 || utf8.RuneCountInString(value) > 200 {
				fields["title"] = "Judul wajib diisi dan maksimal 200 karakter."
			} else {
				params.Title = &value
			}
		}
	}
	if input.Notes.Set {
		params.NotesSet = true
		params.Notes = normalizedPatchText(input.Notes, 5000, "notes", "Catatan maksimal 5000 karakter.", fields)
	}
	if input.Priority.Set {
		if input.Priority.Null {
			fields["priority"] = "Prioritas tidak boleh null."
		} else {
			value := strings.TrimSpace(input.Priority.Value)
			if value != domain.PriorityLow && value != domain.PriorityNormal && value != domain.PriorityHigh {
				fields["priority"] = "Prioritas harus low, normal, atau high."
			} else {
				params.Priority = &value
			}
		}
	}
	if input.DueLocal.Set {
		params.DueAtSet = true
	}
	return params, fields
}

func (api *API) deleteTask(response http.ResponseWriter, request *http.Request) {
	resourceDelete(api, response, request, chi.URLParam(request, "taskID"), "Tugas tidak ditemukan.", "delete task", api.store.DeleteTask)
}

func (api *API) uncompleteTask(response http.ResponseWriter, request *http.Request) {
	taskID := chi.URLParam(request, "taskID")
	if !store.ValidUUID(taskID) {
		writeResourceNotFound(response, request, "Tugas tidak ditemukan.")
		return
	}
	if err := decodeOptionalEmptyJSON(response, request); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	task, err := api.store.UncompleteTask(request.Context(), principal.UserID, taskID)
	if writeStoreResultError(api, response, request, "uncomplete task", "Tugas tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusOK, task)
}

type eventListResponse struct {
	From     string         `json:"from"`
	To       string         `json:"to"`
	Timezone string         `json:"timezone"`
	Items    []domain.Event `json:"items"`
}

func (api *API) listEvents(response http.ResponseWriter, request *http.Request) {
	principal, _ := auth.PrincipalFromContext(request.Context())
	dateRange, ok := api.requestDateRange(response, request, principal.UserID, 0, 30, map[string]struct{}{"from": {}, "to": {}})
	if !ok {
		return
	}
	events, err := api.store.ListEvents(request.Context(), principal.UserID, dateRange.From, dateRange.To, dateRange.Start, dateRange.End)
	if err != nil {
		api.internalError(response, request, "list events", err)
		return
	}
	agenda.SortEvents(events, dateRange.From, dateRange.Location)
	writeJSON(response, http.StatusOK, eventListResponse{dateRange.From, dateRange.To, dateRange.Timezone, events})
}

func (api *API) getEvent(response http.ResponseWriter, request *http.Request) {
	eventID := chi.URLParam(request, "eventID")
	if !store.ValidUUID(eventID) {
		writeResourceNotFound(response, request, "Jadwal tidak ditemukan.")
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	event, err := api.store.GetEvent(request.Context(), principal.UserID, eventID)
	if writeStoreResultError(api, response, request, "get event", "Jadwal tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusOK, event)
}

type patchEventInput struct {
	Title       patchStringField `json:"title"`
	Notes       patchStringField `json:"notes"`
	Location    patchStringField `json:"location"`
	AllDay      patchBoolField   `json:"all_day"`
	StartsLocal patchStringField `json:"starts_local"`
	EndsLocal   patchStringField `json:"ends_local"`
	StartsOn    patchStringField `json:"starts_on"`
	EndsOn      patchStringField `json:"ends_on"`
}

func (api *API) patchEvent(response http.ResponseWriter, request *http.Request) {
	eventID := chi.URLParam(request, "eventID")
	if !store.ValidUUID(eventID) {
		writeResourceNotFound(response, request, "Jadwal tidak ditemukan.")
		return
	}
	var input patchEventInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	params, fields, scheduleSet := validatePatchEventInput(&input)
	if len(fields) > 0 {
		writeValidationError(response, request, fields)
		return
	}

	principal, _ := auth.PrincipalFromContext(request.Context())
	if _, err := api.store.GetEvent(request.Context(), principal.UserID, eventID); err != nil {
		if writeStoreResultError(api, response, request, "get event before update", "Jadwal tidak ditemukan.", err) {
			return
		}
	}
	if scheduleSet {
		profile, err := api.store.GetProfile(request.Context(), principal.UserID)
		if err != nil {
			api.internalError(response, request, "get event profile", err)
			return
		}
		if profile.Timezone == nil {
			writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu sebelum mengubah waktu jadwal.", nil)
			return
		}
		location, err := timeutil.LoadLocation(*profile.Timezone)
		if err != nil {
			api.internalError(response, request, "load stored timezone", err)
			return
		}
		params.ScheduleSet = true
		params.AllDay = input.AllDay.Value
		params.Timezone = *profile.Timezone
		if params.AllDay {
			startsOn, _ := time.Parse(time.DateOnly, strings.TrimSpace(input.StartsOn.Value))
			params.StartsOn = &startsOn
			if input.EndsOn.Set && !input.EndsOn.Null {
				endsOn, _ := time.Parse(time.DateOnly, strings.TrimSpace(input.EndsOn.Value))
				params.EndsOn = &endsOn
			}
		} else {
			startsAt, err := timeutil.ParseLocalWallTime(input.StartsLocal.Value, location)
			if err != nil {
				fields["starts_local"] = localWallTimeMessage(err)
			} else {
				params.StartsAt = &startsAt
			}
			if input.EndsLocal.Set && !input.EndsLocal.Null {
				endsAt, err := timeutil.ParseLocalWallTime(input.EndsLocal.Value, location)
				if err != nil {
					fields["ends_local"] = localWallTimeMessage(err)
				} else {
					params.EndsAt = &endsAt
				}
			}
			if params.StartsAt != nil && params.EndsAt != nil && !params.EndsAt.After(*params.StartsAt) {
				fields["ends_local"] = "Waktu selesai harus setelah waktu mulai."
			}
		}
		if len(fields) > 0 {
			writeValidationError(response, request, fields)
			return
		}
	}
	params.ID = eventID
	params.UserID = principal.UserID
	event, err := api.store.UpdateEvent(request.Context(), params)
	if writeStoreResultError(api, response, request, "update event", "Jadwal tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusOK, event)
}

func validatePatchEventInput(input *patchEventInput) (domain.UpdateEventParams, map[string]string, bool) {
	params := domain.UpdateEventParams{}
	fields := make(map[string]string)
	scheduleSet := input.AllDay.Set || input.StartsLocal.Set || input.EndsLocal.Set || input.StartsOn.Set || input.EndsOn.Set
	if !input.Title.Set && !input.Notes.Set && !input.Location.Set && !scheduleSet {
		fields["body"] = "Kirim setidaknya satu field yang ingin diubah."
		return params, fields, false
	}
	if input.Title.Set {
		if input.Title.Null {
			fields["title"] = "Judul tidak boleh null."
		} else {
			value := strings.TrimSpace(input.Title.Value)
			if utf8.RuneCountInString(value) == 0 || utf8.RuneCountInString(value) > 200 {
				fields["title"] = "Judul wajib diisi dan maksimal 200 karakter."
			} else {
				params.Title = &value
			}
		}
	}
	if input.Notes.Set {
		params.NotesSet = true
		params.Notes = normalizedPatchText(input.Notes, 5000, "notes", "Catatan maksimal 5000 karakter.", fields)
	}
	if input.Location.Set {
		params.LocationSet = true
		params.Location = normalizedPatchText(input.Location, 500, "location", "Lokasi maksimal 500 karakter.", fields)
	}
	if !scheduleSet {
		return params, fields, false
	}
	if !input.AllDay.Set || input.AllDay.Null {
		fields["all_day"] = "Kirim all_day bersama penggantian jadwal lengkap."
		return params, fields, true
	}
	if input.AllDay.Value {
		if input.StartsLocal.Set {
			fields["starts_local"] = "Tidak digunakan untuk jadwal sepanjang hari."
		}
		if input.EndsLocal.Set {
			fields["ends_local"] = "Tidak digunakan untuk jadwal sepanjang hari."
		}
		if !input.StartsOn.Set || input.StartsOn.Null {
			fields["starts_on"] = "Tanggal mulai wajib diisi."
		} else if _, err := time.Parse(time.DateOnly, strings.TrimSpace(input.StartsOn.Value)); err != nil {
			fields["starts_on"] = "Gunakan tanggal dengan format YYYY-MM-DD."
		}
		if input.EndsOn.Set && !input.EndsOn.Null {
			end, endErr := time.Parse(time.DateOnly, strings.TrimSpace(input.EndsOn.Value))
			start, startErr := time.Parse(time.DateOnly, strings.TrimSpace(input.StartsOn.Value))
			if endErr != nil {
				fields["ends_on"] = "Gunakan tanggal dengan format YYYY-MM-DD."
			} else if startErr == nil && end.Before(start) {
				fields["ends_on"] = "Tanggal selesai tidak boleh sebelum tanggal mulai."
			}
		}
	} else {
		if input.StartsOn.Set {
			fields["starts_on"] = "Tidak digunakan untuk jadwal dengan waktu."
		}
		if input.EndsOn.Set {
			fields["ends_on"] = "Tidak digunakan untuk jadwal dengan waktu."
		}
		if !input.StartsLocal.Set || input.StartsLocal.Null {
			fields["starts_local"] = "Waktu mulai wajib diisi."
		}
	}
	return params, fields, true
}

func (api *API) deleteEvent(response http.ResponseWriter, request *http.Request) {
	resourceDelete(api, response, request, chi.URLParam(request, "eventID"), "Jadwal tidak ditemukan.", "delete event", api.store.DeleteEvent)
}

type billCursor struct {
	Version int    `json:"v"`
	State   string `json:"state"`
	At      string `json:"at"`
	ID      string `json:"id"`
}

type billListResponse struct {
	Items      []domain.Bill `json:"items"`
	NextCursor *string       `json:"next_cursor"`
}

func (api *API) listBills(response http.ResponseWriter, request *http.Request) {
	if field, ok := invalidQuery(request, map[string]struct{}{"state": {}, "limit": {}, "cursor": {}}); !ok {
		writeValidationError(response, request, map[string]string{field: "Parameter query tidak valid."})
		return
	}
	state := request.URL.Query().Get("state")
	if state == "" {
		state = "unpaid"
	}
	fields := make(map[string]string)
	if state != "unpaid" && state != "paid" {
		fields["state"] = "State harus unpaid atau paid."
	}
	limit := 50
	if value := request.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			fields["limit"] = "Limit harus berupa bilangan bulat antara 1 dan 100."
		} else {
			limit = parsed
		}
	}
	var afterAt *time.Time
	var afterID *string
	if encoded := request.URL.Query().Get("cursor"); encoded != "" {
		at, id, err := decodeBillCursor(encoded, state)
		if err != nil {
			fields["cursor"] = "Cursor tidak valid."
		} else {
			afterAt, afterID = &at, &id
		}
	}
	if len(fields) > 0 {
		writeValidationError(response, request, fields)
		return
	}

	principal, _ := auth.PrincipalFromContext(request.Context())
	bills, err := api.store.ListBills(request.Context(), principal.UserID, state, limit+1, afterAt, afterID)
	if err != nil {
		api.internalError(response, request, "list bills", err)
		return
	}
	var next *string
	if len(bills) > limit {
		bills = bills[:limit]
		last := bills[len(bills)-1]
		at := last.DueAt
		if state == "paid" && last.PaidAt != nil {
			at = *last.PaidAt
		}
		encoded, err := encodeBillCursor(state, at, last.ID)
		if err != nil {
			api.internalError(response, request, "encode bill cursor", err)
			return
		}
		next = &encoded
	}
	writeJSON(response, http.StatusOK, billListResponse{Items: bills, NextCursor: next})
}

func encodeBillCursor(state string, at time.Time, id string) (string, error) {
	payload, err := json.Marshal(billCursor{Version: 1, State: state, At: at.UTC().Format(time.RFC3339Nano), ID: id})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeBillCursor(encoded, state string) (time.Time, string, error) {
	if len(encoded) > 1024 {
		return time.Time{}, "", errors.New("cursor too large")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return time.Time{}, "", err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var cursor billCursor
	if err := decoder.Decode(&cursor); err != nil {
		return time.Time{}, "", err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return time.Time{}, "", errors.New("cursor trailing data")
	}
	if cursor.Version != 1 || cursor.State != state || !store.ValidUUID(cursor.ID) {
		return time.Time{}, "", errors.New("cursor fields")
	}
	at, err := time.Parse(time.RFC3339Nano, cursor.At)
	if err != nil {
		return time.Time{}, "", err
	}
	return at.UTC(), cursor.ID, nil
}

func (api *API) getBill(response http.ResponseWriter, request *http.Request) {
	billID := chi.URLParam(request, "billID")
	if !store.ValidUUID(billID) {
		writeResourceNotFound(response, request, "Tagihan tidak ditemukan.")
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	bill, err := api.store.GetBill(request.Context(), principal.UserID, billID)
	if writeStoreResultError(api, response, request, "get bill", "Tagihan tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusOK, bill)
}

type patchBillInput struct {
	Title    patchStringField `json:"title"`
	Notes    patchStringField `json:"notes"`
	Amount   patchInt64Field  `json:"amount"`
	Currency patchStringField `json:"currency"`
	DueLocal patchStringField `json:"due_local"`
}

func (api *API) patchBill(response http.ResponseWriter, request *http.Request) {
	billID := chi.URLParam(request, "billID")
	if !store.ValidUUID(billID) {
		writeResourceNotFound(response, request, "Tagihan tidak ditemukan.")
		return
	}
	var input patchBillInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	params, fields := validatePatchBillInput(&input)
	if len(fields) > 0 {
		writeValidationError(response, request, fields)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	if _, err := api.store.GetBill(request.Context(), principal.UserID, billID); err != nil {
		if writeStoreResultError(api, response, request, "get bill before update", "Tagihan tidak ditemukan.", err) {
			return
		}
	}
	if input.DueLocal.Set {
		profile, err := api.store.GetProfile(request.Context(), principal.UserID)
		if err != nil {
			api.internalError(response, request, "get bill profile", err)
			return
		}
		if profile.Timezone == nil {
			writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu sebelum mengubah jatuh tempo tagihan.", nil)
			return
		}
		location, err := timeutil.LoadLocation(*profile.Timezone)
		if err != nil {
			api.internalError(response, request, "load stored timezone", err)
			return
		}
		dueAt, err := timeutil.ParseLocalWallTime(input.DueLocal.Value, location)
		if err != nil {
			writeValidationError(response, request, map[string]string{"due_local": localWallTimeMessage(err)})
			return
		}
		params.DueAt = &dueAt
	}
	params.ID = billID
	params.UserID = principal.UserID
	bill, err := api.store.UpdateBill(request.Context(), params)
	if writeStoreResultError(api, response, request, "update bill", "Tagihan tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusOK, bill)
}

func validatePatchBillInput(input *patchBillInput) (domain.UpdateBillParams, map[string]string) {
	params := domain.UpdateBillParams{}
	fields := make(map[string]string)
	if !input.Title.Set && !input.Notes.Set && !input.Amount.Set && !input.Currency.Set && !input.DueLocal.Set {
		fields["body"] = "Kirim setidaknya satu field yang ingin diubah."
		return params, fields
	}
	if input.Title.Set {
		if input.Title.Null {
			fields["title"] = "Judul tidak boleh null."
		} else {
			value := strings.TrimSpace(input.Title.Value)
			if utf8.RuneCountInString(value) == 0 || utf8.RuneCountInString(value) > 200 {
				fields["title"] = "Judul wajib diisi dan maksimal 200 karakter."
			} else {
				params.Title = &value
			}
		}
	}
	if input.Notes.Set {
		params.NotesSet = true
		params.Notes = normalizedPatchText(input.Notes, 5000, "notes", "Catatan maksimal 5000 karakter.", fields)
	}
	if input.Amount.Set {
		if input.Amount.Null || input.Amount.Value < 1 || input.Amount.Value > domain.MaxBillAmount {
			fields["amount"] = "Nominal harus berupa bilangan bulat antara 1 dan 9007199254740991."
		} else {
			params.Amount = &input.Amount.Value
		}
	}
	if input.Currency.Set {
		if input.Currency.Null {
			fields["currency"] = "Mata uang tidak boleh null."
		} else {
			value := strings.TrimSpace(input.Currency.Value)
			if !validCurrency(value) {
				fields["currency"] = "Mata uang harus terdiri dari tiga huruf kapital, misalnya IDR."
			} else {
				params.Currency = &value
			}
		}
	}
	if input.DueLocal.Set && input.DueLocal.Null {
		fields["due_local"] = "Waktu jatuh tempo tidak boleh null."
	}
	return params, fields
}

func (api *API) deleteBill(response http.ResponseWriter, request *http.Request) {
	resourceDelete(api, response, request, chi.URLParam(request, "billID"), "Tagihan tidak ditemukan.", "delete bill", api.store.DeleteBill)
}

func (api *API) markBillUnpaid(response http.ResponseWriter, request *http.Request) {
	billID := chi.URLParam(request, "billID")
	if !store.ValidUUID(billID) {
		writeResourceNotFound(response, request, "Tagihan tidak ditemukan.")
		return
	}
	if err := decodeOptionalEmptyJSON(response, request); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	bill, err := api.store.MarkBillUnpaid(request.Context(), principal.UserID, billID)
	if writeStoreResultError(api, response, request, "mark bill unpaid", "Tagihan tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusOK, bill)
}

func normalizedPatchText(field patchStringField, maximum int, key, message string, fields map[string]string) *string {
	if field.Null {
		return nil
	}
	value := strings.TrimSpace(field.Value)
	if utf8.RuneCountInString(value) > maximum {
		fields[key] = message
		return nil
	}
	if value == "" {
		return nil
	}
	return &value
}

func writeValidationError(response http.ResponseWriter, request *http.Request, fields map[string]string) {
	writeError(response, request, http.StatusBadRequest, "VALIDATION_ERROR", "Periksa data yang belum valid.", fields)
}

func writeResourceNotFound(response http.ResponseWriter, request *http.Request, message string) {
	writeError(response, request, http.StatusNotFound, "NOT_FOUND", message, nil)
}

func writeStoreResultError(api *API, response http.ResponseWriter, request *http.Request, operation, notFoundMessage string, err error) bool {
	if errors.Is(err, store.ErrNotFound) {
		writeResourceNotFound(response, request, notFoundMessage)
		return true
	}
	if err != nil {
		api.internalError(response, request, operation, err)
		return true
	}
	return false
}

func resourceDelete(api *API, response http.ResponseWriter, request *http.Request, id, notFoundMessage, operation string, remove func(context.Context, string, string) error) {
	if !store.ValidUUID(id) {
		writeResourceNotFound(response, request, notFoundMessage)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	if writeStoreResultError(api, response, request, operation, notFoundMessage, remove(request.Context(), principal.UserID, id)) {
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func invalidQuery(request *http.Request, allowed map[string]struct{}) (string, bool) {
	for key, values := range request.URL.Query() {
		if _, ok := allowed[key]; !ok || len(values) != 1 {
			return key, false
		}
	}
	return "", true
}
