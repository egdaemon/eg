package jwtx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt/v4"

	"github.com/egdaemon/eg/internal/httpx"
)

const (
	mdkey = "authorization"
)

type JWTSecretSource func() []byte

func NotAuthorized() error {
	return errorsx.New("not authorized")
}

type Option func(*jwt.RegisteredClaims)

func ClaimsOptionAuthzExpiration() Option {
	return ClaimsOptionExpiration(time.Hour)
}

func ClaimsOptionAuthnExpiration() Option {
	return ClaimsOptionExpiration(7 * 24 * time.Hour)
}

func ClaimsOptionExpiration(d time.Duration) Option {
	return func(rc *jwt.RegisteredClaims) {
		rc.ExpiresAt = jwt.NewNumericDate(rc.IssuedAt.Time.Add(d))
	}
}

func ClaimsOptionIssued(ts time.Time) Option {
	return func(rc *jwt.RegisteredClaims) {
		rc.IssuedAt = jwt.NewNumericDate(ts)
	}
}

func ClaimsOptionNotBefore(ts time.Time) Option {
	return func(rc *jwt.RegisteredClaims) {
		rc.NotBefore = jwt.NewNumericDate(ts)
	}
}

func ClaimsOptionIssuer(s string) Option {
	return func(rc *jwt.RegisteredClaims) {
		rc.Issuer = s
	}
}

func ClaimsOptionID(s string) Option {
	return func(rc *jwt.RegisteredClaims) {
		rc.ID = s
	}
}

func ClaimsOptionComposed(opts ...Option) Option {
	return func(rc *jwt.RegisteredClaims) {
		for _, opt := range opts {
			opt(rc)
		}
	}
}

func NewJWTClaims(subject string, options ...Option) (c jwt.RegisteredClaims) {
	ts := time.Now()
	c = jwt.RegisteredClaims{
		ID:        uuid.Must(uuid.NewV4()).String(),
		Subject:   subject,
		IssuedAt:  jwt.NewNumericDate(ts),
		NotBefore: jwt.NewNumericDate(ts),
		ExpiresAt: jwt.NewNumericDate(ts.Add(time.Minute)),
	}

	for _, opt := range options {
		opt(&c)
	}

	return c
}

// UnsafeEncodeLoginToken generates a login token using the provided source of entropy. generally code should not
// use this method.
func UnsafeSigned(jwtsecret []byte, t jwt.Claims) (signed string, err error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS512, t).SignedString(jwtsecret)
}

// Signed generates a signed token
func Signed(jwtsecret []byte, t jwt.Claims) (signed string, err error) {
	// since we're passing the crypto rand to the unsafe method it becomes safe.
	return UnsafeSigned(jwtsecret, t)
}

func BearerFromHTTPContext(ctx context.Context, r *http.Request, jwtsecret JWTSecretSource, t jwt.Claims) (_ string, err error) {
	encoded := r.Header.Get(mdkey)
	return encoded, Validate(jwtsecret, encoded, t)
}

func Validate(jwtsecret JWTSecretSource, encoded string, t jwt.Claims) error {
	encoded = strings.NewReplacer(
		"bearer ",
		"",
		"BEARER ",
		"",
	).Replace(encoded)

	token, err := jwt.ParseWithClaims(string(encoded), t, func(token *jwt.Token) (interface{}, error) {
		return jwtsecret(), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS512.Alg()}))

	if err != nil {
		return errorsx.Wrap(err, "unable to parse jwt token")
	}

	if !token.Valid {
		return errorsx.Errorf("invalid token %s", t)
	}

	return nil
}

func EncodeJSON(v any) (_ string, err error) {
	var (
		encoded []byte
	)

	if encoded, err = json.Marshal(v); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(encoded), nil
}

func DecodeJSON(s string, v any) (err error) {
	var (
		decoded []byte
	)

	if decoded, err = base64.URLEncoding.DecodeString(s); err != nil {
		return err
	}

	return json.Unmarshal(decoded, v)
}

type AuthResponse struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

func RetrieveAuthCode(ctx context.Context, chttp *http.Client, uri string) (r AuthResponse, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return r, err
	}

	resp, err := httpx.AsError(chttp.Do(req))
	if err != nil {
		return r, err
	}
	defer httpx.AutoClose(resp)

	if err = json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return r, err
	}

	return r, nil
}

func DebugString(s string) string {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return s
	}
	decoded, _ := jwt.DecodeSegment(parts[1])
	return string(decoded)
}
