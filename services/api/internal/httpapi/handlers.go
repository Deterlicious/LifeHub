package httpapi

import (
	"context"
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
	if err := api.store.Ping(ctx); err != nil {
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

func (api *API) createTask(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Title    string  `json:"title"`
		Notes    *string `json:"notes"`
		Priority string  `json:"priority"`
		DueLocal *string `json:"due_local"`
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

	var dueAt *time.Time
	if input.DueLocal != nil {
		location, err := timeutil.LoadLocation(*profile.Timezone)
		if err != nil {
			api.internalError(response, request, "load stored timezone", err)
			return
		}
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
	taskID, err := store.NewUUID()
	if err != nil {
		api.internalError(response, request, "generate task id", err)
		return
	}
	task, err := api.store.CreateTask(request.Context(), domain.CreateTaskParams{
		ID:       taskID,
		UserID:   principal.UserID,
		Title:    input.Title,
		Notes:    input.Notes,
		Priority: input.Priority,
		DueAt:    dueAt,
	})
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
	date, start, end := timeutil.LocalDayWindow(now, location)
	tasks, err := api.store.ListTodayTasks(request.Context(), principal.UserID, start, end)
	if err != nil {
		api.internalError(response, request, "list today tasks", err)
		return
	}
	writeJSON(response, http.StatusOK, today.Build(date, *profile.Timezone, now, start, end, tasks))
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
