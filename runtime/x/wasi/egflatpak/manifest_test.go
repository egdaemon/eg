package egflatpak_test

import (
	"fmt"
	"testing"

	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/x/wasi/egflatpak"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestManifestExample1(t *testing.T) {
	m := egflatpak.New(
		"org.egdaemon.example1",
		"example",
		egflatpak.Option().Modules(
			egflatpak.ModuleCopy("/dne"),
		)...,
	).Manifest
	encoded, err := yaml.Marshal(m)
	require.NoError(t, err)
	// log.Println("content", string(encoded))
	require.Equal(t, testx.ReadMD5(testx.Fixture(fmt.Sprintf("%s.yml", m.ID))), md5x.FormatString(md5x.Digest(encoded)))
}

func TestManifestExample2(t *testing.T) {
	m := egflatpak.Manifest{
		ID:      "org.egdaemon.example2",
		Runtime: egflatpak.Runtime{ID: "org.gnome.Platform", Version: "45"},
		SDK:     egflatpak.SDK{ID: "org.gnome.Sdk"},
		Command: "echo hello world",
	}

	encoded, err := yaml.Marshal(m)
	require.NoError(t, err)
	// log.Println("content", string(encoded))
	require.Equal(t, testx.ReadMD5(testx.Fixture(fmt.Sprintf("%s.yml", m.ID))), md5x.FormatString(md5x.Digest(encoded)))
}

func TestManifestSourceDirectory(t *testing.T) {
	m := egflatpak.New(
		"org.egdaemon.example3",
		"example",
		egflatpak.Option().Modules(
			egflatpak.NewModule("download", "simple",
				egflatpak.ModuleOptions().Commands(
					"install -D somefile.bin /app/bin/somefile.bin",
				).Sources(
					egflatpak.SourceFile(
						"https://example.com/somefile.bin",
						egflatpak.SourceOptions().Directory("custom-dest")...,
					),
				)...,
			),
		)...,
	).Manifest
	encoded, err := yaml.Marshal(m)
	require.NoError(t, err)
	// log.Println("content", string(encoded))
	require.Equal(t, testx.ReadMD5(testx.Fixture(fmt.Sprintf("%s.yml", m.ID))), md5x.FormatString(md5x.Digest(encoded)))
}
