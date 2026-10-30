package authn_test

import (
	"fmt"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestEndpointSSHAuth(t *testing.T) {
	t.Run("returns expected endpoint", func(t *testing.T) {
		host := eg.EnvAPIHostDefault()
		want := oauth2.Endpoint{
			AuthURL:   fmt.Sprintf("%s/oauth2/ssh/auth", host),
			TokenURL:  fmt.Sprintf("%s/oauth2/ssh/token", host),
			AuthStyle: oauth2.AuthStyleInHeader,
		}
		require.Equal(t, want, authn.EndpointSSHAuth())
	})
}
