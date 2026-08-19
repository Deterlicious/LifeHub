package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestDevAuthIssueAndVerify(t *testing.T) {
	dev, err := NewDevAuth("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	dev.clock = func() time.Time { return now }

	token, err := dev.Issue("User@Example.com")
	if err != nil {
		t.Fatal(err)
	}
	principal, err := dev.Verify(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.Email != "user@example.com" {
		t.Fatalf("email = %q", principal.Email)
	}
	if principal.UserID != deterministicDevUUID("user@example.com") {
		t.Fatalf("unexpected subject %q", principal.UserID)
	}

	dev.clock = func() time.Time { return now.Add(2 * time.Hour) }
	if _, err := dev.Verify(context.Background(), token); err == nil {
		t.Fatal("expired token was accepted")
	}
}

func TestDevAuthRejectsWrongSecret(t *testing.T) {
	first, _ := NewDevAuth("0123456789abcdef0123456789abcdef")
	second, _ := NewDevAuth("abcdef0123456789abcdef0123456789")
	token, err := first.Issue("user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Verify(context.Background(), token); err == nil {
		t.Fatal("token with invalid signature was accepted")
	}
}

func TestJWKSVerifierValidatesRegisteredClaimsAndSubject(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyID := "test-key"
	jwks := map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA",
			"kid": keyID,
			"use": "sig",
			"alg": "RS256",
			"n":   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
		}},
	}
	body, err := json.Marshal(jwks)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write(body)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	verifier, err := NewJWKSVerifier(ctx, server.URL, "https://example.supabase.co/auth/v1", "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	verifier.clock = func() time.Time { return now }

	signed := signRSA(t, privateKey, keyID, claims{
		Email: "user@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://example.supabase.co/auth/v1",
			Audience:  jwt.ClaimStrings{"authenticated"},
			Subject:   "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	})
	if _, err := verifier.Verify(context.Background(), signed); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}

	badSubject := signRSA(t, privateKey, keyID, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://example.supabase.co/auth/v1",
			Audience:  jwt.ClaimStrings{"authenticated"},
			Subject:   "not-a-uuid",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	})
	if _, err := verifier.Verify(context.Background(), badSubject); err == nil {
		t.Fatal("invalid subject was accepted")
	}
}

func TestJWKSVerifierRejectsInvalidProductionTokens(t *testing.T) {
	verifier, privateKey, _, now := newJWKSFixture(t)

	validClaims := func() claims {
		return claims{RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://example.supabase.co/auth/v1",
			Audience:  jwt.ClaimStrings{"authenticated"},
			Subject:   "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		}}
	}

	wrongIssuer := validClaims()
	wrongIssuer.Issuer = "https://attacker.example/auth/v1"
	wrongAudience := validClaims()
	wrongAudience.Audience = jwt.ClaimStrings{"service_role"}
	expired := validClaims()
	expired.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Minute))
	futureIssuedAt := validClaims()
	futureIssuedAt.IssuedAt = jwt.NewNumericDate(now.Add(time.Minute))
	missingExpiry := validClaims()
	missingExpiry.ExpiresAt = nil
	invalidSubject := validClaims()
	invalidSubject.Subject = "not-a-uuid"

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	wrongAlgorithmToken := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims())
	wrongAlgorithmToken.Header["kid"] = "rsa-key"
	wrongAlgorithm, err := wrongAlgorithmToken.SignedString([]byte("not-an-rsa-key"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		token string
	}{
		{name: "wrong issuer", token: signRSA(t, privateKey, "rsa-key", wrongIssuer)},
		{name: "wrong audience", token: signRSA(t, privateKey, "rsa-key", wrongAudience)},
		{name: "expired", token: signRSA(t, privateKey, "rsa-key", expired)},
		{name: "future issued at", token: signRSA(t, privateKey, "rsa-key", futureIssuedAt)},
		{name: "missing expiry", token: signRSA(t, privateKey, "rsa-key", missingExpiry)},
		{name: "invalid subject", token: signRSA(t, privateKey, "rsa-key", invalidSubject)},
		{name: "wrong signature", token: signRSA(t, otherKey, "rsa-key", validClaims())},
		{name: "wrong algorithm", token: wrongAlgorithm},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := verifier.Verify(context.Background(), test.token); err == nil {
				t.Fatal("invalid token was accepted")
			}
		})
	}
}

func TestJWKSVerifierAcceptsES256(t *testing.T) {
	verifier, _, privateKey, now := newJWKSFixture(t)
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims{RegisteredClaims: jwt.RegisteredClaims{
		Issuer:    "https://example.supabase.co/auth/v1",
		Audience:  jwt.ClaimStrings{"authenticated"},
		Subject:   "118a9a57-d9ac-4d3a-91a5-a4a8bb93d270",
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(now),
	}})
	token.Header["kid"] = "ec-key"
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(context.Background(), signed); err != nil {
		t.Fatalf("valid ES256 token rejected: %v", err)
	}
}

func newJWKSFixture(t *testing.T) (*JWKSVerifier, *rsa.PrivateKey, *ecdsa.PrivateKey, time.Time) {
	t.Helper()
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwks := map[string]any{"keys": []map[string]string{
		{
			"kty": "RSA", "kid": "rsa-key", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(rsaKey.PublicKey.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaKey.PublicKey.E)).Bytes()),
		},
		{
			"kty": "EC", "kid": "ec-key", "use": "sig", "alg": "ES256", "crv": "P-256",
			"x": base64.RawURLEncoding.EncodeToString(ecKey.PublicKey.X.FillBytes(make([]byte, 32))),
			"y": base64.RawURLEncoding.EncodeToString(ecKey.PublicKey.Y.FillBytes(make([]byte, 32))),
		},
	}}
	body, err := json.Marshal(jwks)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write(body)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	verifier, err := NewJWKSVerifier(ctx, server.URL, "https://example.supabase.co/auth/v1", "authenticated")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	verifier.clock = func() time.Time { return now }
	return verifier, rsaKey, ecKey, now
}

func signRSA(t *testing.T, key *rsa.PrivateKey, keyID string, tokenClaims claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, tokenClaims)
	token.Header["kid"] = keyID
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
