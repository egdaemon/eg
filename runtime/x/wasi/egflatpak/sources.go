package egflatpak

import "path/filepath"

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

func SourceDir(dir string) Source {
	return Source{Type: "dir", Path: dir}
}

func SourceTarball(url, sha256d string) Source {
	return Source{
		Type:        "archive",
		URL:         url,
		Destination: filepath.Base(url),
		SHA256:      sha256d,
	}
}
