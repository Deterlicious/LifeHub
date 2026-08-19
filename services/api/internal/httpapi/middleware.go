package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"lifehub/services/api/internal/auth"

	"github.com/go-chi/chi/v5"
)

type requestIDContextKey struct{}

// 499 is the conventional reverse-proxy status for a request the client
// canceled before the server could finish. It keeps expected disconnects out
// of the 5xx error budget; the disconnected client does not receive a body.
const statusClientClosedRequest = 499

func (api *API) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		response.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(request.Context(), requestIDContextKey{}, requestID)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "req_unavailable"
	}
	return "req_" + hex.EncodeToString(bytes[:])
}

func (api *API) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		values := request.Header.Values("Authorization")
		if len(values) != 1 {
			writeError(response, request, http.StatusUnauthorized, "UNAUTHENTICATED", "Sesi tidak valid.", nil)
			return
		}
		parts := strings.Fields(values[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			writeError(response, request, http.StatusUnauthorized, "UNAUTHENTICATED", "Sesi tidak valid.", nil)
			return
		}
		principal, err := api.verifier.Verify(request.Context(), parts[1])
		if err != nil {
			writeError(response, request, http.StatusUnauthorized, "UNAUTHENTICATED", "Sesi tidak valid.", nil)
			return
		}
		ctx := auth.ContextWithPrincipal(request.Context(), principal)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func requestTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			ctx, cancel := context.WithTimeout(request.Context(), timeout)
			defer cancel()
			next.ServeHTTP(response, request.WithContext(ctx))
		})
	}
}

func cors(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			origin := request.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(response, request)
				return
			}
			response.Header().Add("Vary", "Origin")
			if _, ok := allowed[origin]; !ok {
				writeError(response, request, http.StatusForbidden, "ORIGIN_NOT_ALLOWED", "Origin tidak diizinkan.", nil)
				return
			}
			response.Header().Set("Access-Control-Allow-Origin", origin)
			response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			response.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			response.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			response.Header().Set("Access-Control-Max-Age", "600")
			if request.Method == http.MethodOptions {
				response.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(response, request)
		})
	}
}

func (api *API) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		defer func() {
			if recover() != nil {
				api.logger.Error("panic recovered",
					slog.String("request_id", requestIDFromContext(request.Context())),
					slog.String("stack", string(debug.Stack())),
				)
				writeError(response, request, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan. Coba lagi.", nil)
			}
		}()
		next.ServeHTTP(response, request)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *responseRecorder) WriteHeader(status int) {
	if recorder.status == 0 {
		recorder.status = status
	}
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *responseRecorder) Write(body []byte) (int, error) {
	if recorder.status == 0 {
		recorder.status = http.StatusOK
	}
	return recorder.ResponseWriter.Write(body)
}

func (recorder *responseRecorder) Unwrap() http.ResponseWriter {
	return recorder.ResponseWriter
}

func (api *API) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		start := time.Now()
		recorder := &responseRecorder{ResponseWriter: response}
		next.ServeHTTP(recorder, request)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		api.logger.Info("http request",
			slog.String("request_id", requestIDFromContext(request.Context())),
			slog.String("method", request.Method),
			slog.String("route", chi.RouteContext(request.Context()).RoutePattern()),
			slog.Int("status", status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}
