package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

const (
	devIssuer   = "lifehub-dev"
	devAudience = "lifehub-web"
)

var ErrInvalidToken = errors.New("invalid access token")

type Principal struct {
	UserID string
	Email  string
}

type Verifier interface {
	Verify(ctx context.Context, accessToken string) (Principal, error)
}

type claims struct {
	Email string `json:"email,omitempty"`
	jwt.RegisteredClaims
}

type JWKSVerifier struct {
	keys     keyfunc.Keyfunc
	issuer   string
	audience string
	clock    func() time.Time
}

func NewJWKSVerifier(ctx context.Context, jwksURL, issuer, audience string) (*JWKSVerifier, error) {
	if strings.TrimSpace(jwksURL) == "" || strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" {
		return nil, fmt.Errorf("jwks url, issuer, and audience are required")
	}
	keys, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{jwksURL}, keyfunc.Override{
		HTTPTimeout:     5 * time.Second,
		RefreshInterval: 10 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("load jwks: %w", err)
	}
	return &JWKSVerifier{
		keys:     keys,
		issuer:   issuer,
		audience: audience,
		clock:    time.Now,
	}, nil
}

func (v *JWKSVerifier) Verify(ctx context.Context, accessToken string) (Principal, error) {
	tokenClaims := &claims{}
	token, err := jwt.ParseWithClaims(
		accessToken,
		tokenClaims,
		v.keys.KeyfuncCtx(ctx),
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg(), jwt.SigningMethodES256.Alg()}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
		jwt.WithTimeFunc(v.clock),
	)
	if err != nil || token == nil || !token.Valid {
		return Principal{}, ErrInvalidToken
	}
	userID, err := canonicalUUID(tokenClaims.Subject)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	return Principal{UserID: userID, Email: tokenClaims.Email}, nil
}

type DevAuth struct {
	secret []byte
	clock  func() time.Time
}

func NewDevAuth(secret string) (*DevAuth, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("development auth secret must be at least 32 bytes")
	}
	return &DevAuth{secret: []byte(secret), clock: time.Now}, nil
}

func (a *DevAuth) Issue(email string) (string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return "", err
	}
	now := a.clock()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    devIssuer,
			Audience:  jwt.ClaimStrings{devAudience},
			Subject:   deterministicDevUUID(email),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)),
		},
	})
	signed, err := token.SignedString(a.secret)
	if err != nil {
		return "", fmt.Errorf("sign development token: %w", err)
	}
	return signed, nil
}

func (a *DevAuth) Verify(_ context.Context, accessToken string) (Principal, error) {
	tokenClaims := &claims{}
	token, err := jwt.ParseWithClaims(
		accessToken,
		tokenClaims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return a.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(devIssuer),
		jwt.WithAudience(devAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(5*time.Second),
		jwt.WithTimeFunc(a.clock),
	)
	if err != nil || token == nil || !token.Valid {
		return Principal{}, ErrInvalidToken
	}
	userID, err := canonicalUUID(tokenClaims.Subject)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	return Principal{UserID: userID, Email: tokenClaims.Email}, nil
}

type principalContextKey struct{}

func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 254 {
		return "", fmt.Errorf("invalid email")
	}
	address, err := mail.ParseAddress(value)
	if err != nil || strings.ToLower(address.Address) != value {
		return "", fmt.Errorf("invalid email")
	}
	return value, nil
}

func deterministicDevUUID(email string) string {
	sum := sha256.Sum256([]byte("lifehub-development-user:" + email))
	bytes := sum[:16]
	bytes[6] = (bytes[6] & 0x0f) | 0x50
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return formatUUID(bytes)
}

func canonicalUUID(value string) (string, error) {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return "", fmt.Errorf("invalid uuid")
	}
	compact := strings.ReplaceAll(value, "-", "")
	decoded, err := hex.DecodeString(compact)
	if err != nil || len(decoded) != 16 {
		return "", fmt.Errorf("invalid uuid")
	}
	return formatUUID(decoded), nil
}

func formatUUID(value []byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}
