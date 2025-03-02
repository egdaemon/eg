package egflatpak

import "path/filepath"

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
