package computeapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/gofrs/uuid/v5"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	circuitv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	pbv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/pb"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// recordingbackoff is a backoff.Strategy returning a small constant delay while recording the attempts it was asked about.
type recordingbackoff struct {
	delay    time.Duration
	attempts []int64
}

func (t *recordingbackoff) Backoff(attempt int64) time.Duration {
	t.attempts = append(t.attempts, attempt)
	return t.delay
}

func TestUpload(t *testing.T) {
	enq := &Enqueued{Id: uuid.Must(uuid.NewV7()).String(), AccountId: "acct", Entry: "main.wasm"}

	t.Run("retries when the runner declines and succeeds once it accepts", func(t *testing.T) {
		var (
			uploads    atomic.Int32
			candidates atomic.Int32
		)

		runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer runner.Close()

		runner.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
			defer s.Close()

			r, err := http.ReadRequest(bufio.NewReader(s))
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, r.Body)

			rec := httptest.NewRecorder()
			if uploads.Add(1) < 3 {
				rec.WriteHeader(http.StatusServiceUnavailable)
			} else {
				rec.WriteHeader(http.StatusAccepted)
				require.NoError(t, json.NewEncoder(rec).Encode(proto.Clone(enq).(*Enqueued)))
			}
			_ = rec.Result().Write(s)
		})

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/c/q/candidates", r.URL.Path)
			candidates.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"id":"runner","p2pid":"` + runner.ID().String() + `","token":"t"}]}`))
		}))
		defer srv.Close()
		dst, err := url.Parse(srv.URL)
		require.NoError(t, err)

		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		addrs := make([]string, 0, len(runner.Addrs()))
		for _, a := range runner.Addrs() {
			addrs = append(addrs, a.String())
		}
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(runner.ID().String()), func(p *lan.Peer) {
			p.Addrs = lan.EncodeList(addrs)
		})).Scan(&lan.Peer{}))

		delays := &recordingbackoff{delay: time.Millisecond}
		err = Upload(
			t.Context(),
			&http.Client{Transport: httpx.RewriteHostTransport(dst, nil)},
			db,
			enq,
			strings.NewReader("kernel"),
			strings.NewReader("environ"),
			UploadPriorities(1, 0),
			UploadP2POnly(true),
			UploadFollow(false),
			UploadBackoff(delays),
		)
		require.NoError(t, err)
		require.EqualValues(t, 3, uploads.Load())
		require.EqualValues(t, 3, candidates.Load(), "candidates are identified again for every attempt")
		require.Equal(t, []int64{0, 1}, delays.attempts)
	})

	t.Run("fails once the attempts are exhausted", func(t *testing.T) {
		var uploads atomic.Int32

		runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer runner.Close()

		runner.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
			defer s.Close()

			r, err := http.ReadRequest(bufio.NewReader(s))
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, r.Body)
			uploads.Add(1)

			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusServiceUnavailable)
			_ = rec.Result().Write(s)
		})

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"id":"runner","p2pid":"` + runner.ID().String() + `","token":"t"}]}`))
		}))
		defer srv.Close()
		dst, err := url.Parse(srv.URL)
		require.NoError(t, err)

		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		addrs := make([]string, 0, len(runner.Addrs()))
		for _, a := range runner.Addrs() {
			addrs = append(addrs, a.String())
		}
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(runner.ID().String()), func(p *lan.Peer) {
			p.Addrs = lan.EncodeList(addrs)
		})).Scan(&lan.Peer{}))

		delays := &recordingbackoff{delay: time.Millisecond}
		err = Upload(
			t.Context(),
			&http.Client{Transport: httpx.RewriteHostTransport(dst, nil)},
			db,
			enq,
			strings.NewReader("kernel"),
			strings.NewReader("environ"),
			UploadPriorities(1, 0),
			UploadP2POnly(true),
			UploadFollow(false),
			UploadBackoff(delays),
			UploadMaxAttempts(3),
		)
		require.ErrorContains(t, err, "after 3 attempts")
		require.EqualValues(t, 3, uploads.Load())
		require.Equal(t, []int64{0, 1}, delays.attempts)
	})

	t.Run("free tier never uses the cluster", func(t *testing.T) {
		var (
			clusters atomic.Int32
			tokens   []string
		)

		runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer runner.Close()

		runner.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
			defer s.Close()

			r, err := http.ReadRequest(bufio.NewReader(s))
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, r.Body)
			tokens = append(tokens, r.FormValue("token"))

			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusAccepted)
			require.NoError(t, json.NewEncoder(rec).Encode(proto.Clone(enq).(*Enqueued)))
			_ = rec.Result().Write(s)
		})

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/c/q/candidates" {
				clusters.Add(1)
			}
			w.WriteHeader(http.StatusPaymentRequired)
		}))
		defer srv.Close()
		dst, err := url.Parse(srv.URL)
		require.NoError(t, err)

		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		addrs := make([]string, 0, len(runner.Addrs()))
		for _, a := range runner.Addrs() {
			addrs = append(addrs, a.String())
		}
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(runner.ID().String()), func(p *lan.Peer) {
			p.Addrs = lan.EncodeList(addrs)
		})).Scan(&lan.Peer{}))

		err = Upload(
			t.Context(),
			&http.Client{Transport: httpx.RewriteHostTransport(dst, nil)},
			db,
			enq,
			strings.NewReader("kernel"),
			strings.NewReader("environ"),
			UploadEndpoint(srv.URL+"/c/q/"),
			UploadPriorities(1, 0),
			UploadFollow(false),
		)
		require.NoError(t, err)
		require.Equal(t, []string{""}, tokens, "free tier uploads carry no candidate token")
		require.EqualValues(t, 0, clusters.Load())
	})

	t.Run("paid accounts fall back to the cluster in the same attempt", func(t *testing.T) {
		var clusters atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/c/q/candidates" {
				_, _ = w.Write([]byte(`{"items":[]}`))
				return
			}

			clusters.Add(1)
			require.NoError(t, json.NewEncoder(w).Encode(&EnqueuedCreateResponse{Enqueued: proto.Clone(enq).(*Enqueued)}))
		}))
		defer srv.Close()
		dst, err := url.Parse(srv.URL)
		require.NoError(t, err)

		delays := &recordingbackoff{delay: time.Millisecond}
		err = Upload(
			t.Context(),
			&http.Client{Transport: httpx.RewriteHostTransport(dst, nil)},
			nil,
			enq,
			strings.NewReader("kernel"),
			strings.NewReader("environ"),
			UploadEndpoint(srv.URL+"/c/q/upload"),
			UploadPriorities(1, 0),
			UploadFollow(false),
			UploadBackoff(delays),
		)
		require.NoError(t, err)
		require.EqualValues(t, 1, clusters.Load())
		require.Empty(t, delays.attempts)
	})

	t.Run("cancellation ends the wait between attempts", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[]}`))
		}))
		defer srv.Close()
		dst, err := url.Parse(srv.URL)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		err = Upload(
			ctx,
			&http.Client{Transport: httpx.RewriteHostTransport(dst, nil)},
			nil,
			enq,
			strings.NewReader("kernel"),
			strings.NewReader("environ"),
			UploadPriorities(1, 0),
			UploadP2POnly(true),
			UploadBackoff(&recordingbackoff{delay: time.Hour}),
		)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
	t.Run("retries when the runner resets the stream", func(t *testing.T) {
		var uploads atomic.Int32

		runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer runner.Close()

		runner.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
			r, err := http.ReadRequest(bufio.NewReader(s))
			if err != nil {
				_ = s.Reset()
				return
			}
			_, _ = io.Copy(io.Discard, r.Body)

			if uploads.Add(1) < 3 {
				_ = s.Reset()
				return
			}
			defer s.Close()

			rec := httptest.NewRecorder()
			rec.WriteHeader(http.StatusAccepted)
			require.NoError(t, json.NewEncoder(rec).Encode(proto.Clone(enq).(*Enqueued)))
			_ = rec.Result().Write(s)
		})

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"id":"runner","p2pid":"` + runner.ID().String() + `","token":"t"}]}`))
		}))
		defer srv.Close()
		dst, err := url.Parse(srv.URL)
		require.NoError(t, err)

		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		addrs := make([]string, 0, len(runner.Addrs()))
		for _, a := range runner.Addrs() {
			addrs = append(addrs, a.String())
		}
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(runner.ID().String()), func(p *lan.Peer) {
			p.Addrs = lan.EncodeList(addrs)
		})).Scan(&lan.Peer{}))

		delays := &recordingbackoff{delay: time.Millisecond}
		err = Upload(
			t.Context(),
			&http.Client{Transport: httpx.RewriteHostTransport(dst, nil)},
			db,
			enq,
			strings.NewReader("kernel"),
			strings.NewReader("environ"),
			UploadPriorities(1, 0),
			UploadP2POnly(true),
			UploadFollow(false),
			UploadBackoff(delays),
		)
		require.NoError(t, err)
		require.EqualValues(t, 3, uploads.Load())
		require.Equal(t, []int64{0, 1}, delays.attempts)
	})

	t.Run("a cluster that requires payment is not retried and the runner failure is reported", func(t *testing.T) {
		var (
			uploads  atomic.Int32
			clusters atomic.Int32
		)

		runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer runner.Close()

		runner.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
			r, err := http.ReadRequest(bufio.NewReader(s))
			if err == nil {
				_, _ = io.Copy(io.Discard, r.Body)
			}
			uploads.Add(1)
			_ = s.Reset()
		})

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/c/q/candidates" {
				clusters.Add(1)
				w.WriteHeader(http.StatusPaymentRequired)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"id":"runner","p2pid":"` + runner.ID().String() + `","token":"t"}]}`))
		}))
		defer srv.Close()
		dst, err := url.Parse(srv.URL)
		require.NoError(t, err)

		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		addrs := make([]string, 0, len(runner.Addrs()))
		for _, a := range runner.Addrs() {
			addrs = append(addrs, a.String())
		}
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(runner.ID().String()), func(p *lan.Peer) {
			p.Addrs = lan.EncodeList(addrs)
		})).Scan(&lan.Peer{}))

		err = Upload(
			t.Context(),
			&http.Client{Transport: httpx.RewriteHostTransport(dst, nil)},
			db,
			enq,
			strings.NewReader("kernel"),
			strings.NewReader("environ"),
			UploadEndpoint(srv.URL+"/c/q/upload"),
			UploadPriorities(1, 0),
			UploadFollow(false),
			UploadBackoff(&recordingbackoff{delay: time.Millisecond}),
			UploadMaxAttempts(3),
		)
		require.ErrorContains(t, err, "after 3 attempts")
		require.ErrorContains(t, err, "local network upload failed")
		require.NotContains(t, err.Error(), "Payment Required")
		require.EqualValues(t, 3, uploads.Load())
		require.EqualValues(t, 1, clusters.Load(), "the cluster is only asked once")
	})
}

func TestRetryable(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "stream reset", err: fmt.Errorf("unable to write request over stream: %w", network.ErrReset), want: true},
		{name: "remote stream reset", err: fmt.Errorf("unable to write request over stream: %w", &network.StreamError{Remote: true}), want: true},
		{name: "reservation error", err: fmt.Errorf("dial: %w", circuitv2.ReservationError{Status: pbv2.Status_RESERVATION_REFUSED}), want: true},
		{name: "no reservation", err: errors.New("error opening relay circuit: NO_RESERVATION (204)"), want: true},
		{name: "unable to open stream", err: fmt.Errorf("unable to open stream to peer: failed to dial: all dials failed\n  * [/p2p-circuit] error opening relay circuit: NO_RESERVATION (204)"), want: true},
		{name: "rejected upload", err: errors.New("runner rejected upload with status 400: bad"), want: false},
		{name: "invalid peer", err: errors.New(`invalid peer id "x"`), want: false},
	} {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, retryable(c.err))
		})
	}
}
