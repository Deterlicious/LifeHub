package config

import "testing"

func TestProductionRequiresHTTPSJWKS(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("WEB_ORIGIN", "https://lifehub.example")
	t.Setenv("SUPABASE_JWKS_URL", "http://example.supabase.co/auth/v1/.well-known/jwks.json")
	t.Setenv("SUPABASE_ISSUER", "https://example.supabase.co/auth/v1")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted an insecure JWKS URL")
	}
}

func TestDevelopmentDisablesSupabaseRequirement(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("WEB_ORIGIN", "http://localhost:3000")
	t.Setenv("DEV_AUTH_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("SUPABASE_JWKS_URL", "")
	t.Setenv("SUPABASE_ISSUER", "")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.Production() || len(config.DevAuthSecret) < 32 {
		t.Fatalf("unexpected config: %#v", config)
	}
}

func TestDevelopmentAuthRejectsNonLoopbackBinding(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("HTTP_ADDR", "0.0.0.0:8080")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("WEB_ORIGIN", "http://localhost:3000")
	t.Setenv("DEV_AUTH_SECRET", "0123456789abcdef0123456789abcdef")
	if _, err := Load(); err == nil {
		t.Fatal("development auth accepted a non-loopback listener")
	}
}

func TestProductionUsesPlatformPortWhenHTTPAddressIsUnset(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("PORT", "10000")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("WEB_ORIGIN", "https://lifehub.example")
	t.Setenv("SUPABASE_JWKS_URL", "https://example.supabase.co/auth/v1/.well-known/jwks.json")
	t.Setenv("SUPABASE_ISSUER", "https://example.supabase.co/auth/v1")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.HTTPAddr != "0.0.0.0:10000" {
		t.Fatalf("HTTPAddr=%q", config.HTTPAddr)
	}
}

func TestPlatformPortRejectsInvalidValue(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("PORT", "70000")
	if _, err := Load(); err == nil {
		t.Fatal("invalid platform port accepted")
	}
}

func TestProductionUsesNetlifyURLAsSameOriginFallback(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("WEB_ORIGIN", "")
	t.Setenv("URL", "https://lifehub-example.netlify.app")
	t.Setenv("SUPABASE_JWKS_URL", "https://example.supabase.co/auth/v1/.well-known/jwks.json")
	t.Setenv("SUPABASE_ISSUER", "https://example.supabase.co/auth/v1")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.WebOrigins) != 1 || config.WebOrigins[0] != "https://lifehub-example.netlify.app" {
		t.Fatalf("WebOrigins=%#v", config.WebOrigins)
	}
}

func TestDatabasePoolBoundsSupportServerlessMinimumZero(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("DEV_AUTH_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DATABASE_MAX_CONNS", "2")
	t.Setenv("DATABASE_MIN_CONNS", "0")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.DatabaseMaxConns != 2 || config.DatabaseMinConns != 0 {
		t.Fatalf("pool bounds max=%d min=%d", config.DatabaseMaxConns, config.DatabaseMinConns)
	}

	t.Setenv("DATABASE_MIN_CONNS", "3")
	if _, err := Load(); err == nil {
		t.Fatal("minimum greater than maximum was accepted")
	}
}
