package egrunnermacos

import (
	"strings"
	"testing"
)

func TestRunnerOptions(t *testing.T) {
	t.Run("PullFrom + Command + env composition", func(t *testing.T) {
		r := New("smoke").
			PullFrom("ghcr.io/cirruslabs/macos-sequoia-base:latest").
			OptionWorkingDirectory("/eg.mnt/work").
			OptionEnv("PATH", "/usr/local/bin").
			OptionEnvVar("CI").
			Command("uname -a")

		var opts []string
		for _, o := range r.options {
			opts = append(opts, o...)
		}
		joined := strings.Join(opts, " ")

		for _, want := range []string{"-w", "/eg.mnt/work", "PATH=/usr/local/bin", "CI"} {
			if !strings.Contains(joined, want) {
				t.Fatalf("missing %q in %q", want, joined)
			}
		}
		if got := strings.Join(r.cmd, " "); got != "uname -a" {
			t.Fatalf("expected uname -a, got %q", got)
		}
		if r.image != "ghcr.io/cirruslabs/macos-sequoia-base:latest" {
			t.Fatalf("expected image, got %q", r.image)
		}
	})

	t.Run("ToModuleRunner carries options", func(t *testing.T) {
		r := New("smoke").PullFrom("img").OptionEnv("FOO", "BAR")
		mr := r.ToModuleRunner()
		if mr.image != "img" {
			t.Fatalf("module runner lost image")
		}
		if len(mr.options) != 1 {
			t.Fatalf("module runner lost options: %v", mr.options)
		}
	})
}
