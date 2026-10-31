package authn_test

import (
	"encoding/json"
	"testing"

	"github.com/egdaemon/eg/authn"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestDecodeLoginOptions(t *testing.T) {
	t.Run("reads the resolved domain and account", func(t *testing.T) {
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"id": "domain-1", "account_id": "account-1"}).SignedString([]byte("secret"))
		require.NoError(t, err)
		response, err := json.Marshal(map[string]string{"token": signed})
		require.NoError(t, err)

		opts, err := authn.DecodeLoginOptions(response)
		require.NoError(t, err)
		require.Equal(t, "domain-1", opts.ID)
		require.Equal(t, "account-1", opts.AccountID)
	})

	t.Run("blank claims signal no account was resolved", func(t *testing.T) {
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"redirect": "/"}).SignedString([]byte("secret"))
		require.NoError(t, err)
		response, err := json.Marshal(map[string]string{"token": signed})
		require.NoError(t, err)

		opts, err := authn.DecodeLoginOptions(response)
		require.NoError(t, err)
		require.Empty(t, opts.ID)
		require.Empty(t, opts.AccountID)
	})

	t.Run("does not verify the signature", func(t *testing.T) {
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"account_id": "account-1"}).SignedString([]byte("some other secret"))
		require.NoError(t, err)
		response, err := json.Marshal(map[string]string{"token": signed})
		require.NoError(t, err)

		opts, err := authn.DecodeLoginOptions(response)
		require.NoError(t, err)
		require.Equal(t, "account-1", opts.AccountID)
	})

	t.Run("invalid response", func(t *testing.T) {
		_, err := authn.DecodeLoginOptions([]byte("not json"))
		require.Error(t, err)
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := authn.DecodeLoginOptions([]byte(`{"token":"nope"}`))
		require.Error(t, err)
	})

	t.Run("missing token", func(t *testing.T) {
		_, err := authn.DecodeLoginOptions([]byte(`{}`))
		require.Error(t, err)
	})
}
