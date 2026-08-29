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
