package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lifehub/services/api/internal/auth"
	"lifehub/services/api/internal/smartcapture"
	"lifehub/services/api/internal/timeutil"
)

const smartCaptureTimeout = 2 * time.Second

func (api *API) parseSmartCapture(response http.ResponseWriter, request *http.Request) {
	principal, _ := auth.PrincipalFromContext(request.Context())
	allowed, remaining, retryAfter := api.smartCaptureLimiter.Allow(principal.UserID)
	response.Header().Set("RateLimit-Limit", "20")
	response.Header().Set("RateLimit-Remaining", strconv.Itoa(remaining))
	if !allowed {
		response.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Round(time.Second)/time.Second)))
		writeError(response, request, http.StatusTooManyRequests, "RATE_LIMITED", "Terlalu banyak permintaan draf. Coba lagi sebentar.", nil)
		return
	}

	var input struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(response, request, &input); err != nil {
		writeDecodeError(response, request, err)
		return
	}
	input.Text = strings.TrimSpace(input.Text)
	if input.Text == "" || len([]rune(input.Text)) > smartcapture.MaxInputLength {
		writeValidationError(response, request, map[string]string{
			"text": "Tulis kebutuhan dalam 1 sampai 1.000 karakter.",
		})
		return
	}

	profile, err := api.store.GetProfile(request.Context(), principal.UserID)
	if err != nil {
		api.internalError(response, request, "get smart capture profile", err)
		return
	}
	if profile.Timezone == nil {
		writeError(response, request, http.StatusConflict, "PROFILE_INCOMPLETE", "Pilih zona waktu sebelum membuat draf.", nil)
		return
	}
	if _, err := timeutil.LoadLocation(*profile.Timezone); err != nil {
		api.internalError(response, request, "load smart capture timezone", err)
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), smartCaptureTimeout)
	defer cancel()
	output, err := api.smartCaptureProvider.Parse(ctx, input.Text, api.clock(), *profile.Timezone)
	if err != nil {
		if errors.Is(err, context.Canceled) && request.Context().Err() != nil {
			api.internalError(response, request, "parse smart capture canceled", request.Context().Err())
			return
		}
		writeError(response, request, http.StatusServiceUnavailable, "SMART_CAPTURE_UNAVAILABLE", "Draf pintar sedang tidak tersedia. Form manual tetap dapat digunakan.", nil)
		return
	}
	if err := smartcapture.ValidateOutput(output); err != nil {
		api.logger.Error("smart capture provider returned invalid output", "provider", api.smartCaptureProvider.Name(), "error", err)
		writeError(response, request, http.StatusServiceUnavailable, "SMART_CAPTURE_UNAVAILABLE", "Draf pintar sedang tidak tersedia. Form manual tetap dapat digunakan.", nil)
		return
	}

	writeJSON(response, http.StatusOK, struct {
		Draft       smartcapture.Draft `json:"draft"`
		Ambiguities []string           `json:"ambiguities"`
		Provider    string             `json:"provider"`
	}{
		Draft:       output.Draft,
		Ambiguities: output.Ambiguities,
		Provider:    api.smartCaptureProvider.Name(),
	})
}
