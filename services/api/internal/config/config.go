package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv           string
	HTTPAddr         string
	DatabaseURL      string
	DatabaseMaxConns int32
	DatabaseMinConns int32
	WebOrigins       []string
	SupabaseJWKSURL  string
	SupabaseIssuer   string
	SupabaseAudience string
	DevAuthSecret    string
}

func (c Config) Production() bool {
	return c.AppEnv == "production"
}

func Load() (Config, error) {
	appEnv := envOrDefault("APP_ENV", "development")
	httpAddr, err := resolveHTTPAddr()
	if err != nil {
		return Config{}, err
	}
	maxConns, err := envInt32("DATABASE_MAX_CONNS", 12, 1, 100)
	if err != nil {
		return Config{}, err
	}
	minConns, err := envInt32("DATABASE_MIN_CONNS", 1, 0, 100)
	if err != nil {
		return Config{}, err
	}
	if minConns > maxConns {
		return Config{}, fmt.Errorf("DATABASE_MIN_CONNS must not exceed DATABASE_MAX_CONNS")
	}
	config := Config{
		AppEnv:           appEnv,
		HTTPAddr:         httpAddr,
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		DatabaseMaxConns: maxConns,
		DatabaseMinConns: minConns,
		WebOrigins:       resolveWebOrigins(appEnv),
		SupabaseJWKSURL:  strings.TrimSpace(os.Getenv("SUPABASE_JWKS_URL")),
		SupabaseIssuer:   strings.TrimSpace(os.Getenv("SUPABASE_ISSUER")),
		SupabaseAudience: envOrDefault("SUPABASE_AUDIENCE", "authenticated"),
		DevAuthSecret:    strings.TrimSpace(os.Getenv("DEV_AUTH_SECRET")),
	}
	if config.AppEnv != "development" && config.AppEnv != "test" && config.AppEnv != "production" {
		return Config{}, fmt.Errorf("APP_ENV must be development, test, or production")
	}
	if config.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if len(config.WebOrigins) == 0 {
		return Config{}, fmt.Errorf("WEB_ORIGIN must contain at least one origin")
	}
	for _, origin := range config.WebOrigins {
		if err := validateOrigin(origin, config.Production()); err != nil {
			return Config{}, fmt.Errorf("WEB_ORIGIN: %w", err)
		}
	}
	if err := validateListenerAddress(config.HTTPAddr); err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR: %w", err)
	}
	if config.Production() {
		if err := validateHTTPSURL("SUPABASE_JWKS_URL", config.SupabaseJWKSURL); err != nil {
			return Config{}, err
		}
		if err := validateHTTPSURL("SUPABASE_ISSUER", config.SupabaseIssuer); err != nil {
			return Config{}, err
		}
		config.DevAuthSecret = ""
	} else {
		if err := validateLoopbackAddress(config.HTTPAddr); err != nil {
			return Config{}, fmt.Errorf("HTTP_ADDR: %w", err)
		}
		if len(config.DevAuthSecret) < 32 {
			return Config{}, fmt.Errorf("DEV_AUTH_SECRET must be at least 32 bytes")
		}
	}
	return config, nil
}

func resolveWebOrigins(appEnv string) []string {
	value := strings.TrimSpace(os.Getenv("WEB_ORIGIN"))
	if value == "" && appEnv == "production" {
		// Netlify supplies URL to both builds and Functions. It is safe to use as
		// the same-origin CORS fallback, while other hosts must still set
		// WEB_ORIGIN explicitly.
		value = strings.TrimSpace(os.Getenv("URL"))
	}
	if value == "" {
		value = "http://localhost:3000"
	}
	return splitOrigins(value)
}

func resolveHTTPAddr() (string, error) {
	if value := strings.TrimSpace(os.Getenv("HTTP_ADDR")); value != "" {
		return value, nil
	}
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		if _, err := parsePort(port); err != nil {
			return "", fmt.Errorf("PORT: %w", err)
		}
		return net.JoinHostPort("0.0.0.0", port), nil
	}
	return "127.0.0.1:8080", nil
}

func validateListenerAddress(value string) error {
	_, port, err := net.SplitHostPort(value)
	if err != nil {
		return fmt.Errorf("must be a host:port address")
	}
	_, err = parsePort(port)
	return err
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("must be an integer from 1 through 65535")
	}
	return port, nil
}

func envInt32(name string, fallback, minimum, maximum int32) (int32, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < int64(minimum) || parsed > int64(maximum) {
		return 0, fmt.Errorf("%s must be an integer from %d through %d", name, minimum, maximum)
	}
	return int32(parsed), nil
}

func validateLoopbackAddress(value string) error {
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		return fmt.Errorf("must be a host:port address")
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("must bind to a loopback address outside production")
	}
	return nil
}

func validateHTTPSURL(name, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("%s must be an absolute https URL", name)
	}
	return nil
}

func validateOrigin(value string, requireHTTPS bool) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%q is not a valid origin", value)
	}
	if parsed.Path != "" {
		return fmt.Errorf("%q must not contain a path", value)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%q must use http or https", value)
	}
	if requireHTTPS && parsed.Scheme != "https" {
		return fmt.Errorf("%q must use https in production", value)
	}
	return nil
}

func splitOrigins(value string) []string {
	var origins []string
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, strings.TrimSuffix(origin, "/"))
		}
	}
	return origins
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
