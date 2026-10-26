package egautogentest

import (
	"context"
	"errors"
	"testing"

	"github.com/egdaemon/eg/internal/iterx"
	"github.com/stretchr/testify/require"
)

type fakegen struct {
	seq      iterx.Seq[Fn]
	ensured  []string
	verified []string
	verifyfn func(artifact string) error
}

func (t *fakegen) Seq() iterx.Seq[Fn] {
	return t.seq
}

func (t *fakegen) Prompt(c PromptContext) (string, error) {
	t.ensured = c.Ensure
	return "prompt for " + c.Fn.Name, nil
}

func (t *fakegen) Verify(ctx context.Context, fn Fn, artifact string) error {
	t.verified = append(t.verified, artifact)
	if t.verifyfn != nil {
		return t.verifyfn(artifact)
	}

	return nil
}

func TestGenerate(t *testing.T) {
	fn := Fn{Name: "Greet"}

	t.Run("verifies the response as is", func(t *testing.T) {
		s := script(t, "package sample\n")
		g := &fakegen{}
		o := option{model: "m", client: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"package sample\n"}, g.verified)
		require.Equal(t, "prompt for Greet", s.requests[0][0].Content)
	})

	t.Run("gives the rules to the prompt", func(t *testing.T) {
		s := script(t, "response", "")
		g := &fakegen{}
		o := option{model: "m", ensure: []string{"a", "b"}, client: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"a", "b"}, g.ensured)
	})

	t.Run("does not judge without rules", func(t *testing.T) {
		s := script(t, "response")
		o := option{model: "m", client: s}

		require.NoError(t, o.generate(t.Context(), &fakegen{}, fn))
		require.Len(t, s.requests, 1)
	})

	t.Run("retries when verification fails, up to the attempts", func(t *testing.T) {
		s := script(t, "one", "two", "three")
		g := &fakegen{verifyfn: func(string) error { return errors.New("failing") }}
		o := option{model: "m", attempts: 3, client: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Len(t, g.verified, 3)
		require.Len(t, s.requests, 3)
	})

	t.Run("stops retrying once verification passes", func(t *testing.T) {
		s := script(t, "one", "two")
		failed := false
		g := &fakegen{verifyfn: func(string) error {
			if !failed {
				failed = true
				return errors.New("failing")
			}
			return nil
		}}
		o := option{model: "m", attempts: 3, client: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Len(t, g.verified, 2)
		require.Len(t, s.requests, 2)
	})

	t.Run("verifies the reworked response", func(t *testing.T) {
		s := script(t, "dirty", violationrecord(1, 1, "dirty"), "clean", "")
		g := &fakegen{}
		o := option{model: "m", ensure: []string{"be clean"}, revisions: 1, client: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Equal(t, []string{"clean"}, g.verified)
	})

	t.Run("discards a response still violating the rules without verifying it", func(t *testing.T) {
		s := script(t, "dirty", violationrecord(1, 1, "dirty"))
		g := &fakegen{}
		o := option{model: "m", ensure: []string{"be clean"}, client: s}

		require.NoError(t, o.generate(t.Context(), g, fn))
		require.Empty(t, g.verified)
	})
}
