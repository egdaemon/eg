package cmdopts_test

import (
	"math"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/stretchr/testify/require"
)

func parseDurationInfFlag(t *testing.T, args []string) (time.Duration, error) {
	t.Helper()

	var cli struct {
		cmdopts.Global
		TTL time.Duration `name:"ttl" type:"durationinf" default:"1h"`
	}
	cli.Context = t.Context()

	parser, err := kong.New(&cli,
		kong.Name("test"),
		kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)),
	)
	require.NoError(t, err)

	_, err = parser.Parse(args)
	return cli.TTL, err
}

func TestParseDurationInf(t *testing.T) {
	t.Run("normal durations", func(t *testing.T) {
		cases := map[string]time.Duration{
			"1h":    time.Hour,
			"30m":   30 * time.Minute,
			"500ms": 500 * time.Millisecond,
		}
		for input, expected := range cases {
			d, err := parseDurationInfFlag(t, []string{"--ttl", input})
			require.NoError(t, err)
			require.Equal(t, expected, d)
		}
	})

	t.Run("infinity aliases", func(t *testing.T) {
		for _, input := range []string{"infinity", "INFINITY", "Infinity"} {
			d, err := parseDurationInfFlag(t, []string{"--ttl", input})
			require.NoError(t, err)
			require.Equal(t, time.Duration(math.MaxInt), d)
		}
	})

	t.Run("invalid duration errors", func(t *testing.T) {
		_, err := parseDurationInfFlag(t, []string{"--ttl", "notaduration"})
		require.Error(t, err)
	})

	t.Run("default value still works", func(t *testing.T) {
		d, err := parseDurationInfFlag(t, []string{})
		require.NoError(t, err)
		require.Equal(t, time.Hour, d)
	})
}
