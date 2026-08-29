package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lifehub/services/api/internal/auth"
	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/store"

	"github.com/go-chi/chi/v5"
)

type reminderScheduleInput struct {
	Kind          patchStringField `json:"kind"`
	MinutesBefore patchInt64Field  `json:"minutes_before"`
	DaysBefore    patchInt64Field  `json:"days_before"`
	TimeLocal     patchStringField `json:"time_local"`
}

type reminderScheduleField struct {
	Set   bool
	Null  bool
	Value reminderScheduleInput
}

func (field *reminderScheduleField) UnmarshalJSON(data []byte) error {
	field.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		field.Null = true
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&field.Value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("schedule must contain one JSON object")
		}
		return err
	}
	return nil
}

type createReminderInput struct {
	SourceKind string                `json:"source_kind"`
	SourceID   string                `json:"source_id"`
	Schedule   reminderScheduleField `json:"schedule"`
}

func (api *API) createReminder(response http.ResponseWriter, request *http.Request) {
	var input createReminderInput
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	fields := make(map[string]string)
	input.SourceKind = strings.TrimSpace(input.SourceKind)
	input.SourceID = strings.TrimSpace(input.SourceID)
	if !validReminderSourceKind(input.SourceKind) {
		fields["source_kind"] = "Sumber harus task, event, bill, atau document."
	}
	scheduleKind, minutesBefore, daysBefore, timeLocal, scheduleFields := validateReminderSchedule(input.Schedule)
	for key, value := range scheduleFields {
		fields[key] = value
	}
	if len(fields) > 0 {
		writeValidationError(response, request, fields)
		return
	}
	if !store.ValidUUID(input.SourceID) {
		writeResourceNotFound(response, request, "Sumber pengingat tidak ditemukan.")
		return
	}
	reminderID, err := store.NewUUID()
	if err != nil {
		api.internalError(response, request, "generate reminder id", err)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	item, err := api.store.CreateReminder(request.Context(), domain.CreateReminderParams{
		ID: reminderID, UserID: principal.UserID, SourceKind: input.SourceKind, SourceID: input.SourceID,
		ScheduleKind: scheduleKind, MinutesBefore: minutesBefore, DaysBefore: daysBefore, TimeLocal: timeLocal,
	})
	if writeReminderStoreError(api, response, request, "create reminder", "Sumber pengingat tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusCreated, item)
}

func (api *API) listReminders(response http.ResponseWriter, request *http.Request) {
	if field, ok := invalidQuery(request, map[string]struct{}{"source_kind": {}, "source_id": {}}); !ok {
		writeValidationError(response, request, map[string]string{field: "Parameter query tidak valid."})
		return
	}
	sourceKind := strings.TrimSpace(request.URL.Query().Get("source_kind"))
	sourceID := strings.TrimSpace(request.URL.Query().Get("source_id"))
	fields := make(map[string]string)
	if !validReminderSourceKind(sourceKind) {
		fields["source_kind"] = "Sumber harus task, event, bill, atau document."
	}
	if sourceID == "" {
		fields["source_id"] = "Source ID wajib diisi bersama source_kind."
	}
	if len(fields) > 0 {
		writeValidationError(response, request, fields)
		return
	}
	if !store.ValidUUID(sourceID) {
		writeResourceNotFound(response, request, "Sumber pengingat tidak ditemukan.")
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	items, err := api.store.ListReminders(request.Context(), principal.UserID, sourceKind, sourceID)
	if writeReminderStoreError(api, response, request, "list reminders", "Sumber pengingat tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusOK, struct {
		Items []domain.Reminder `json:"items"`
	}{Items: items})
}

func (api *API) getReminder(response http.ResponseWriter, request *http.Request) {
	reminderID := chi.URLParam(request, "reminderID")
	if !store.ValidUUID(reminderID) {
		writeResourceNotFound(response, request, "Pengingat tidak ditemukan.")
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	item, err := api.store.GetReminder(request.Context(), principal.UserID, reminderID)
	if writeReminderStoreError(api, response, request, "get reminder", "Pengingat tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusOK, item)
}

func (api *API) patchReminder(response http.ResponseWriter, request *http.Request) {
	reminderID := chi.URLParam(request, "reminderID")
	if !store.ValidUUID(reminderID) {
		writeResourceNotFound(response, request, "Pengingat tidak ditemukan.")
		return
	}
	var input struct {
		Schedule reminderScheduleField `json:"schedule"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	scheduleKind, minutesBefore, daysBefore, timeLocal, fields := validateReminderSchedule(input.Schedule)
	if len(fields) > 0 {
		writeValidationError(response, request, fields)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	item, err := api.store.UpdateReminder(request.Context(), domain.UpdateReminderParams{
		ID: reminderID, UserID: principal.UserID, ScheduleKind: scheduleKind,
		MinutesBefore: minutesBefore, DaysBefore: daysBefore, TimeLocal: timeLocal,
	})
	if writeReminderStoreError(api, response, request, "update reminder", "Pengingat tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusOK, item)
}

func (api *API) deleteReminder(response http.ResponseWriter, request *http.Request) {
	resourceDelete(api, response, request, chi.URLParam(request, "reminderID"), "Pengingat tidak ditemukan.", "delete reminder", api.store.DeleteReminder)
}

func validateReminderSchedule(field reminderScheduleField) (string, *int, *int, *string, map[string]string) {
	fields := make(map[string]string)
	if !field.Set || field.Null {
		fields["schedule"] = "Konfigurasi pengingat wajib diisi."
		return "", nil, nil, nil, fields
	}
	input := field.Value
	if !input.Kind.Set || input.Kind.Null {
		fields["schedule.kind"] = "Jenis jadwal pengingat wajib diisi."
		return "", nil, nil, nil, fields
	}
	kind := strings.TrimSpace(input.Kind.Value)
	switch kind {
	case "before_moment":
		if !input.MinutesBefore.Set || input.MinutesBefore.Null || input.MinutesBefore.Value < 0 || input.MinutesBefore.Value > 525600 {
			fields["schedule.minutes_before"] = "Menit sebelum harus bilangan bulat antara 0 dan 525600."
		}
		if input.DaysBefore.Set {
			fields["schedule.days_before"] = "Tidak digunakan untuk pengingat berbasis waktu."
		}
		if input.TimeLocal.Set {
			fields["schedule.time_local"] = "Tidak digunakan untuk pengingat berbasis waktu."
		}
		if len(fields) > 0 {
			return "", nil, nil, nil, fields
		}
		value := int(input.MinutesBefore.Value)
		return kind, &value, nil, nil, fields
	case "before_date":
		if !input.DaysBefore.Set || input.DaysBefore.Null || input.DaysBefore.Value < 0 || input.DaysBefore.Value > 3650 {
			fields["schedule.days_before"] = "Hari sebelum harus bilangan bulat antara 0 dan 3650."
		}
		if input.MinutesBefore.Set {
			fields["schedule.minutes_before"] = "Tidak digunakan untuk pengingat berbasis tanggal."
		}
		var local string
		if !input.TimeLocal.Set || input.TimeLocal.Null {
			fields["schedule.time_local"] = "Jam lokal wajib diisi dengan format HH:MM."
		} else {
			local = strings.TrimSpace(input.TimeLocal.Value)
			parsed, err := time.Parse("15:04", local)
			if err != nil || parsed.Format("15:04") != local {
				fields["schedule.time_local"] = "Gunakan jam lokal dengan format HH:MM."
			}
		}
		if len(fields) > 0 {
			return "", nil, nil, nil, fields
		}
		days := int(input.DaysBefore.Value)
		return kind, nil, &days, &local, fields
	default:
		fields["schedule.kind"] = "Jenis jadwal harus before_moment atau before_date."
		return "", nil, nil, nil, fields
	}
}

func validReminderSourceKind(value string) bool {
	return value == "task" || value == "event" || value == "bill" || value == "document"
}

func writeReminderStoreError(api *API, response http.ResponseWriter, request *http.Request, operation, notFoundMessage string, err error) bool {
	if errors.Is(err, store.ErrNotFound) {
		writeResourceNotFound(response, request, notFoundMessage)
		return true
	}
	if errors.Is(err, store.ErrReminderSourceUnscheduled) {
		writeError(response, request, http.StatusConflict, "SOURCE_UNSCHEDULABLE", "Sumber tidak memiliki waktu pengingat aktif di masa depan.", nil)
		return true
	}
	if errors.Is(err, store.ErrReminderScheduleMismatch) {
		writeValidationError(response, request, map[string]string{"schedule": "Jenis pengingat tidak sesuai dengan jadwal sumber."})
		return true
	}
	if errors.Is(err, store.ErrReminderInvalidLocalTime) {
		writeValidationError(response, request, map[string]string{"schedule.time_local": "Waktu ini ambigu atau tidak ada karena perubahan zona waktu."})
		return true
	}
	if err != nil {
		api.internalError(response, request, operation, err)
		return true
	}
	return false
}

type notificationCursor struct {
	Version int    `json:"v"`
	At      string `json:"at"`
	ID      string `json:"id"`
}

type notificationListResponse struct {
	Items       []domain.Notification `json:"items"`
	NextCursor  *string               `json:"next_cursor"`
	UnreadCount int                   `json:"unread_count"`
}

func (api *API) listNotifications(response http.ResponseWriter, request *http.Request) {
	if field, ok := invalidQuery(request, map[string]struct{}{"limit": {}, "cursor": {}}); !ok {
		writeValidationError(response, request, map[string]string{field: "Parameter query tidak valid."})
		return
	}
	fields := make(map[string]string)
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
		at, id, err := decodeNotificationCursor(encoded)
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
	items, unread, err := api.store.ListNotifications(request.Context(), principal.UserID, limit+1, afterAt, afterID)
	if err != nil {
		api.internalError(response, request, "list notifications", err)
		return
	}
	var next *string
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		encoded, err := encodeNotificationCursor(last.CreatedAt, last.ID)
		if err != nil {
			api.internalError(response, request, "encode notification cursor", err)
			return
		}
		next = &encoded
	}
	writeJSON(response, http.StatusOK, notificationListResponse{Items: items, NextCursor: next, UnreadCount: unread})
}

func encodeNotificationCursor(at time.Time, id string) (string, error) {
	payload, err := json.Marshal(notificationCursor{Version: 1, At: at.UTC().Format(time.RFC3339Nano), ID: id})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeNotificationCursor(encoded string) (time.Time, string, error) {
	if len(encoded) > 1024 {
		return time.Time{}, "", errors.New("cursor too large")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return time.Time{}, "", err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var cursor notificationCursor
	if err := decoder.Decode(&cursor); err != nil {
		return time.Time{}, "", err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return time.Time{}, "", errors.New("cursor trailing data")
	}
	if cursor.Version != 1 || !store.ValidUUID(cursor.ID) {
		return time.Time{}, "", errors.New("cursor fields")
	}
	at, err := time.Parse(time.RFC3339Nano, cursor.At)
	if err != nil {
		return time.Time{}, "", err
	}
	return at.UTC(), cursor.ID, nil
}

func (api *API) notificationUnreadCount(response http.ResponseWriter, request *http.Request) {
	principal, _ := auth.PrincipalFromContext(request.Context())
	count, err := api.store.NotificationUnreadCount(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "count unread notifications", err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]int{"unread_count": count})
}

func (api *API) markNotificationRead(response http.ResponseWriter, request *http.Request) {
	notificationID := chi.URLParam(request, "notificationID")
	if !store.ValidUUID(notificationID) {
		writeResourceNotFound(response, request, "Notifikasi tidak ditemukan.")
		return
	}
	if err := decodeOptionalEmptyJSON(response, request); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	item, unread, err := api.store.MarkNotificationRead(request.Context(), principal.UserID, notificationID)
	if writeStoreResultError(api, response, request, "mark notification read", "Notifikasi tidak ditemukan.", err) {
		return
	}
	writeJSON(response, http.StatusOK, struct {
		Item        domain.Notification `json:"item"`
		UnreadCount int                 `json:"unread_count"`
	}{Item: item, UnreadCount: unread})
}

func (api *API) markAllNotificationsRead(response http.ResponseWriter, request *http.Request) {
	if err := decodeOptionalEmptyJSON(response, request); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	principal, _ := auth.PrincipalFromContext(request.Context())
	marked, err := api.store.MarkAllNotificationsRead(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "mark all notifications read", err)
		return
	}
	writeJSON(response, http.StatusOK, struct {
		MarkedRead  int `json:"marked_read"`
		UnreadCount int `json:"unread_count"`
	}{MarkedRead: marked, UnreadCount: 0})
}
