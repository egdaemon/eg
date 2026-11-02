package me_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/me"
	"github.com/stretchr/testify/require"
)

func TestUpdateProfile(t *testing.T) {
	t.Run("should patch the profile of the token and return the stored one", func(t *testing.T) {
		var got me.MeProfileUpdateRequest

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/me/", r.URL.Path)
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(b, &got))
			// the profile is determined by the token, it must never be part of the request.
			require.JSONEq(t, `{"profile":{"display":"alice","email":"alice@example.com"}}`, string(b))
			fmt.Fprint(w, `{"profile":{"id":"profile-1","account_id":"account-1","display":"alice","email":"alice@example.com","session_watermark":"ignored"}}`)
		}))
		t.Cleanup(srv.Close)
		c := &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}

		p, err := me.UpdateProfile(t.Context(), c, "alice", "alice@example.com")
		require.NoError(t, err)
		require.Equal(t, "alice", got.GetProfile().GetDisplay())
		require.Equal(t, "profile-1", p.GetId())
		require.Equal(t, "account-1", p.GetAccountId())
		require.Equal(t, "alice@example.com", p.GetEmail())
	})

	t.Run("should return the status of a rejected request", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadRequest) }))
		t.Cleanup(srv.Close)
		c := &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}

		_, err := me.UpdateProfile(t.Context(), c, "", "not an email")
		var status *me.StatusError
		require.ErrorAs(t, err, &status)
		require.Equal(t, http.StatusBadRequest, status.Code)
		require.True(t, me.BadRequest(err))
	})
}

func TestEnableProfile(t *testing.T) {
	t.Run("should enable the profile and return the stored one", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/me/profile/profile-1", r.URL.Path)
			fmt.Fprint(w, `{"profile":{"id":"profile-1","account_id":"account-1","display":"alice","email":"alice@example.com"}}`)
		}))
		t.Cleanup(srv.Close)
		c := &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}

		p, err := me.EnableProfile(t.Context(), c, "profile-1")
		require.NoError(t, err)
		require.Equal(t, "profile-1", p.GetId())
		require.Equal(t, "account-1", p.GetAccountId())
	})

	t.Run("should escape the profile id", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/me/profile/a/b", r.URL.Path)
			require.Equal(t, "/me/profile/a%2Fb", r.URL.EscapedPath())
			fmt.Fprint(w, `{}`)
		}))
		t.Cleanup(srv.Close)
		c := &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}

		_, err := me.EnableProfile(t.Context(), c, "a/b")
		require.NoError(t, err)
	})

	t.Run("should return the status of a rejected request", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
		t.Cleanup(srv.Close)
		c := &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}

		_, err := me.EnableProfile(t.Context(), c, "profile-1")
		var status *me.StatusError
		require.ErrorAs(t, err, &status)
		require.Equal(t, http.StatusForbidden, status.Code)
		require.True(t, me.Forbidden(err))
		require.False(t, me.NotFound(err))
	})

	t.Run("should report an unknown profile", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
		t.Cleanup(srv.Close)
		c := &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}

		_, err := me.EnableProfile(t.Context(), c, "profile-1")
		require.True(t, me.NotFound(err))
	})
}

func TestDisableProfile(t *testing.T) {
	t.Run("should disable the profile and return the stored one", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/me/profile/profile-1", r.URL.Path)
			fmt.Fprint(w, `{"profile":{"id":"profile-1","account_id":"account-1","display":"alice","email":"alice@example.com"}}`)
		}))
		t.Cleanup(srv.Close)
		c := &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}

		p, err := me.DisableProfile(t.Context(), c, "profile-1")
		require.NoError(t, err)
		require.Equal(t, "profile-1", p.GetId())
		require.Equal(t, "account-1", p.GetAccountId())
	})

	t.Run("should escape the profile id", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/me/profile/a/b", r.URL.Path)
			require.Equal(t, "/me/profile/a%2Fb", r.URL.EscapedPath())
			fmt.Fprint(w, `{}`)
		}))
		t.Cleanup(srv.Close)
		c := &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}

		_, err := me.DisableProfile(t.Context(), c, "a/b")
		require.NoError(t, err)
	})

	t.Run("should return the status of a rejected request", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
		t.Cleanup(srv.Close)
		c := &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}

		_, err := me.DisableProfile(t.Context(), c, "profile-1")
		var status *me.StatusError
		require.ErrorAs(t, err, &status)
		require.Equal(t, http.StatusForbidden, status.Code)
		require.True(t, me.Forbidden(err))
		require.False(t, me.NotFound(err))
	})

	t.Run("should report an unknown profile", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
		t.Cleanup(srv.Close)
		c := &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}

		_, err := me.DisableProfile(t.Context(), c, "profile-1")
		require.True(t, me.NotFound(err))
	})
}
