package httpapi

import (
	"errors"
	"net/http"
	"time"

	"lifehub/services/api/internal/agenda"
	"lifehub/services/api/internal/auth"
	"lifehub/services/api/internal/timeutil"
)

type localDateRange struct {
	From     string
	To       string
	Timezone string
	Today    string
	Now      time.Time
	Start    time.Time
	End      time.Time
	Location *time.Location
}

func (api *API) getAgenda(response http.ResponseWriter, request *http.Request) {
	principal, _ := auth.PrincipalFromContext(request.Context())
	dateRange, ok := api.requestDateRange(response, request, principal.UserID, 1, 30, map[string]struct{}{"from": {}, "to": {}})
	if !ok {
		return
	}
	fromOn, err := time.Parse(time.DateOnly, dateRange.From)
	if err != nil {
		api.internalError(response, request, "parse Agenda recurrence start", err)
		return
	}
	toOn, err := time.Parse(time.DateOnly, dateRange.To)
	if err != nil {
		api.internalError(response, request, "parse Agenda recurrence end", err)
		return
	}
	if err := api.store.MaterializeRecurrences(request.Context(), principal.UserID, fromOn, toOn); err != nil {
		api.internalError(response, request, "materialize Agenda recurrences", err)
		return
	}
	tasks, err := api.store.ListAgendaTasks(request.Context(), principal.UserID, dateRange.Start, dateRange.End)
	if err != nil {
		api.internalError(response, request, "list agenda tasks", err)
		return
	}
	events, err := api.store.ListEvents(request.Context(), principal.UserID, dateRange.From, dateRange.To, dateRange.Start, dateRange.End)
	if err != nil {
		api.internalError(response, request, "list agenda events", err)
		return
	}
	bills, err := api.store.ListAgendaBills(request.Context(), principal.UserID, dateRange.Start, dateRange.End)
	if err != nil {
		api.internalError(response, request, "list agenda bills", err)
		return
	}
	documents, err := api.store.ListAgendaDocuments(request.Context(), principal.UserID, dateRange.From, dateRange.To)
	if err != nil {
		api.internalError(response, request, "list agenda documents", err)
		return
	}
	payload := agenda.Build(
		dateRange.From,
		dateRange.To,
		dateRange.Today,
		dateRange.Now,
		dateRange.Location,
		dateRange.Timezone,
		tasks,
		events,
		bills,
		documents,
	)
	writeJSON(response, http.StatusOK, payload)
}

func (api *API) requestDateRange(response http.ResponseWriter, request *http.Request, userID string, defaultFromOffset, defaultToOffset int, allowed map[string]struct{}) (localDateRange, bool) {
	if field, ok := invalidQuery(request, allowed); !ok {
		writeValidationError(response, request, map[string]string{field: "Parameter query tidak valid."})
		return localDateRange{}, false
	}
	profile, err := api.store.GetProfile(request.Context(), userID)
	if err != nil {
		api.internalError(response, request, "get date range profile", err)
		return localDateRange{}, false
	}
	if profile.Timezone == nil {
		writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu untuk membuka rentang tanggal.", nil)
		return localDateRange{}, false
	}
	location, err := timeutil.LoadLocation(*profile.Timezone)
	if err != nil {
		api.internalError(response, request, "load stored timezone", err)
		return localDateRange{}, false
	}

	query := request.URL.Query()
	fromValue, fromSet := query["from"]
	toValue, toSet := query["to"]
	fields := make(map[string]string)
	if fromSet != toSet {
		if !fromSet {
			fields["from"] = "Kirim from dan to secara bersamaan."
		}
		if !toSet {
			fields["to"] = "Kirim from dan to secara bersamaan."
		}
		writeValidationError(response, request, fields)
		return localDateRange{}, false
	}

	now := api.clock()
	localNow := now.In(location)
	todayDate := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	fromDate := todayDate.AddDate(0, 0, defaultFromOffset)
	toDate := todayDate.AddDate(0, 0, defaultToOffset)
	if fromSet {
		fromDate, err = time.Parse(time.DateOnly, fromValue[0])
		if err != nil {
			fields["from"] = "Gunakan tanggal dengan format YYYY-MM-DD."
		}
		toDate, err = time.Parse(time.DateOnly, toValue[0])
		if err != nil {
			fields["to"] = "Gunakan tanggal dengan format YYYY-MM-DD."
		}
	}
	if len(fields) == 0 {
		days := int(toDate.Sub(fromDate)/(24*time.Hour)) + 1
		if days < 1 {
			fields["to"] = "Tanggal akhir tidak boleh sebelum tanggal mulai."
		} else if days > 31 {
			fields["to"] = "Rentang maksimal 31 tanggal kalender."
		}
	}
	if len(fields) > 0 {
		writeValidationError(response, request, fields)
		return localDateRange{}, false
	}

	start, err := timeutil.LocalDateStart(fromDate, location)
	if err != nil {
		if errors.Is(err, timeutil.ErrSkippedLocalDate) {
			writeValidationError(response, request, map[string]string{"from": "Tanggal ini tidak ada di zona waktu profil."})
		} else {
			api.internalError(response, request, "resolve range start", err)
		}
		return localDateRange{}, false
	}
	if _, err := timeutil.LocalDateStart(toDate, location); err != nil {
		if errors.Is(err, timeutil.ErrSkippedLocalDate) {
			writeValidationError(response, request, map[string]string{"to": "Tanggal ini tidak ada di zona waktu profil."})
		} else {
			api.internalError(response, request, "validate range end date", err)
		}
		return localDateRange{}, false
	}
	end, err := timeutil.LocalDateEnd(toDate, location)
	if err != nil {
		api.internalError(response, request, "resolve range end", err)
		return localDateRange{}, false
	}
	return localDateRange{
		From:     fromDate.Format(time.DateOnly),
		To:       toDate.Format(time.DateOnly),
		Timezone: *profile.Timezone,
		Today:    todayDate.Format(time.DateOnly),
		Now:      now,
		Start:    start,
		End:      end,
		Location: location,
	}, true
}
