package egflatpak

import (
	"path/filepath"

	"github.com/egdaemon/eg/internal/langx"
)

type soption = func(*Source)
type soptions []soption

func SourceOptions() soptions {
	return soptions(nil)
}

func (t soptions) Arch(a ...string) soptions {
	return append(t, func(s *Source) {
		s.Architectures = a
	})
}

func (t soptions) Destination(a string) soptions {
	return append(t, func(s *Source) {
		s.Destination = a
	})
}

func (t soptions) Mirrors(a ...string) soptions {
	return append(t, func(s *Source) {
		s.Mirrors = a
	})
}

func (t soptions) Tag(a string) soptions {
	return append(t, func(s *Source) {
		s.Tag = a
	})
}

func SourceDir(dir string, options ...soption) Source {
	return langx.Clone(Source{Type: "dir", Path: dir}, options...)
}

func SourceTarball(url, sha256d string, options ...soption) Source {
	return langx.Clone(Source{
		Type:        "archive",
		URL:         url,
		Destination: filepath.Base(url),
		SHA256:      sha256d,
	}, options...)
}

func SourceGit(url, commit string, options ...soption) Source {
	return langx.Clone(Source{
		Type:   "git",
		URL:    url,
		Commit: commit,
	}, options...)
}

func SourceShell(options ...soption) Source {
	return langx.Clone(Source{
		Type: "shell",
	}, options...)
}
