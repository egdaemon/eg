package compute_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/jsonx"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestTokenEncoding(t *testing.T) {
	t.Run("encodes the jwt claim names with 64 bit integers as strings", func(t *testing.T) {
		encoded, err := json.Marshal(&compute.Token{
			Id:               "a",
			AccountId:        "b",
			ProfileId:        "c",
			SessionId:        "d",
			Issued:           1761434790000,
			Expires:          1761438390000,
			NotBefore:        1761434780000,
			Bearer:           "bearer",
			ComputeRead:      true,
			ComputeRemaining: 9,
		})
		require.NoError(t, err)
		require.JSONEq(t, `{"jti":"a","iss":"b","sub":"c","sid":"d","iat":"1761434790000","exp":"1761438390000","nbf":"1761434780000","bearer":"bearer","compute_read":true,"compute_remaining":"9"}`, string(encoded))
	})

	t.Run("decodes api responses where integers are strings", func(t *testing.T) {
		const doc = `{"token":{"jti":"a","iss":"b","sub":"c","iat":"1761434790000","exp":"1761438390000","nbf":"1761434780000","compute_remaining":"9"}}`

		var out struct {
			Token compute.Token `json:"token"`
		}
		require.NoError(t, jsonx.Unmarshal([]byte(doc), &out))
		require.Equal(t, "a", out.Token.Id)
		require.EqualValues(t, 1761438390000, out.Token.Expires)
		require.EqualValues(t, 9, out.Token.ComputeRemaining)
	})

	t.Run("round trips", func(t *testing.T) {
		in := compute.Token{Id: "a", AccountId: "b", ProfileId: "c", SessionId: "d", Issued: 1, Expires: 2, NotBefore: 3, ComputeShared: true, ComputeRemaining: 1 << 60}
		encoded, err := json.Marshal(&in)
		require.NoError(t, err)

		var out compute.Token
		require.NoError(t, json.Unmarshal(encoded, &out))
		require.Equal(t, in.Id, out.Id)
		require.Equal(t, in.Expires, out.Expires)
		require.Equal(t, in.ComputeRemaining, out.ComputeRemaining)
		require.True(t, out.ComputeShared)
	})
}

// the jwt library decodes the claims of a token with encoding/json, and the issued token carries
// its numeric dates and permissions as json numbers.
func TestTokenDecodesJWTClaims(t *testing.T) {
	t.Run("numeric claims", func(t *testing.T) {
		const doc = `{"jti":"a","iss":"b","sub":"c","iat":1761434790,"exp":1761438390,"nbf":1761434780,"compute_read":true,"compute_remaining":9}`

		var out compute.Token
		require.NoError(t, json.Unmarshal([]byte(doc), &out))
		require.EqualValues(t, 1761438390, out.Expires)
		require.EqualValues(t, 1761434780, out.NotBefore)
		require.EqualValues(t, 9, out.ComputeRemaining)
		require.True(t, out.ComputeRead)
	})

	t.Run("a token signed as claims parses back into a token", func(t *testing.T) {
		now := time.Now()
		secret := []byte("secret")
		in := compute.Token{
			Id:               "a",
			AccountId:        "b",
			ProfileId:        "c",
			Issued:           now.Add(-time.Minute).Unix(),
			NotBefore:        now.Add(-time.Minute).Unix(),
			Expires:          now.Add(time.Hour).Unix(),
			ComputeRead:      true,
			ComputeRemaining: 9,
		}

		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS512, &in).SignedString(secret)
		require.NoError(t, err)

		var out compute.Token
		_, err = jwt.ParseWithClaims(signed, &out, func(*jwt.Token) (any, error) { return secret, nil })
		require.NoError(t, err)
		require.Equal(t, in.Expires, out.Expires)
		require.EqualValues(t, 9, out.ComputeRemaining)
		require.True(t, out.ComputeRead)
	})

	t.Run("signed token parsed into a token", func(t *testing.T) {
		now := time.Now()
		type claims struct {
			jwt.RegisteredClaims
			ComputeRead      bool   `json:"compute_read"`
			ComputeRemaining uint64 `json:"compute_remaining"`
		}

		secret := []byte("secret")
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS512, claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "a",
				Issuer:    "b",
				Subject:   "c",
				IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
				NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			},
			ComputeRead:      true,
			ComputeRemaining: 9,
		}).SignedString(secret)
		require.NoError(t, err)

		var out compute.Token
		_, err = jwt.ParseWithClaims(signed, &out, func(*jwt.Token) (any, error) { return secret, nil })
		require.NoError(t, err)
		require.Equal(t, "b", out.AccountId)
		require.Equal(t, "c", out.ProfileId)
		require.True(t, out.ComputeRead)
		require.EqualValues(t, 9, out.ComputeRemaining)
	})
}
