package application

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"lifehub/services/api/internal/auth"
	"lifehub/services/api/internal/config"
	"lifehub/services/api/internal/httpapi"
	"lifehub/services/api/internal/riverinfra"
	"lifehub/services/api/internal/store"
)

type App struct {
	Handler http.Handler
	storage *store.Store
}

type InitializationError struct {
	Stage string
	Err   error
}

func (err *InitializationError) Error() string {
	return fmt.Sprintf("%s: %v", err.Stage, err.Err)
}

func (err *InitializationError) Unwrap() error {
	return err.Err
}

func New(ctx context.Context, settings config.Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}
	validationContext, validationCancel := context.WithTimeout(ctx, 15*time.Second)
	if err := riverinfra.Validate(validationContext, settings.DatabaseURL); err != nil {
		validationCancel()
		return nil, &InitializationError{Stage: "river-schema", Err: err}
	}
	validationCancel()

	storage, err := store.OpenWithPoolSettings(ctx, settings.DatabaseURL, store.PoolSettings{
		MaxConns: settings.DatabaseMaxConns,
		MinConns: settings.DatabaseMinConns,
	})
	if err != nil {
		return nil, &InitializationError{Stage: "database-pool", Err: err}
	}
	readyContext, readyCancel := context.WithTimeout(ctx, 10*time.Second)
	err = storage.Ready(readyContext)
	readyCancel()
	if err != nil {
		storage.Close()
		return nil, &InitializationError{Stage: "database-readiness", Err: err}
	}

	var (
		verifier  auth.Verifier
		devIssuer httpapi.DevTokenIssuer
	)
	if settings.Production() {
		jwksVerifier, verifyErr := auth.NewJWKSVerifier(
			ctx,
			settings.SupabaseJWKSURL,
			settings.SupabaseIssuer,
			settings.SupabaseAudience,
		)
		if verifyErr != nil {
			storage.Close()
			return nil, &InitializationError{Stage: "jwks", Err: verifyErr}
		}
		verifier = jwksVerifier
	} else {
		devAuth, devErr := auth.NewDevAuth(settings.DevAuthSecret)
		if devErr != nil {
			storage.Close()
			return nil, &InitializationError{Stage: "development-auth", Err: devErr}
		}
		verifier = devAuth
		devIssuer = devAuth
	}

	return &App{
		Handler: httpapi.New(httpapi.Options{
			Store: storage, Verifier: verifier, DevIssuer: devIssuer,
			WebOrigins: settings.WebOrigins, Logger: logger, Production: settings.Production(),
		}),
		storage: storage,
	}, nil
}

func (app *App) Close() {
	if app != nil && app.storage != nil {
		app.storage.Close()
	}
}

func Stage(err error) string {
	if initializationError, ok := err.(*InitializationError); ok {
		return initializationError.Stage
	}
	return "unknown"
}
