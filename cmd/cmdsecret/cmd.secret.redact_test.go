package cmdsecret

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedact(t *testing.T) {
	t.Run("sensitive and additional variables are redacted", func(t *testing.T) {
		in := "GH_TOKEN=ghp_secret\nFOO=foovalue\nPLAIN=visible\nBAR=barvalue\nEG_GIT_AUTH_HTTP_PASSWORD=pw\n"
		var out bytes.Buffer
		require.NoError(t, redact(&out, strings.NewReader(in), "seed", "FOO", "BAR"))
		require.Equal(t, strings.Join([]string{
			"GH_TOKEN=" + digest("seed", "ghp_secret"),
			"FOO=" + digest("seed", "foovalue"),
			"PLAIN=visible",
			"BAR=" + digest("seed", "barvalue"),
			"EG_GIT_AUTH_HTTP_PASSWORD=" + digest("seed", "pw"),
		}, "\n")+"\n", out.String())
	})

	t.Run("nothing to redact passes the environment through", func(t *testing.T) {
		var out bytes.Buffer
		require.NoError(t, redact(&out, strings.NewReader("A=1\nB=2\n"), "seed"))
		require.Equal(t, "A=1\nB=2\n", out.String())
	})
}

func digest(seed, v string) string {
	mac := hmac.New(sha256.New, []byte(seed))
	mac.Write([]byte(v))
	return "redacted:" + hex.EncodeToString(mac.Sum(nil))
}
