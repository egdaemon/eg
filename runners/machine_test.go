package runners

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestEnviron(t *testing.T) {
	passwd := &fstest.MapFile{Data: []byte("root:x:0:0:root:/root:/bin/bash\negd:x:990:990::/home/egd:/bin/bash\n")}

	t.Run("image environment and root's home", func(t *testing.T) {
		root := fstest.MapFS{
			"etc/environment": &fstest.MapFile{Data: []byte("PATH=/usr/sbin:/usr/bin\nDEBIAN_FRONTEND=noninteractive\n")},
			"etc/passwd":      passwd,
		}

		environ, err := environ(root)
		require.NoError(t, err)
		require.Equal(t, []string{"PATH=/usr/sbin:/usr/bin", "DEBIAN_FRONTEND=noninteractive", "HOME=/root"}, environ)
	})

	t.Run("missing environment is root's home alone", func(t *testing.T) {
		environ, err := environ(fstest.MapFS{"etc/passwd": passwd})
		require.NoError(t, err)
		require.Equal(t, []string{"HOME=/root"}, environ)
	})

	t.Run("missing passwd fails", func(t *testing.T) {
		_, err := environ(fstest.MapFS{})
		require.Error(t, err)
	})

	t.Run("no root account sets no home", func(t *testing.T) {
		environ, err := environ(fstest.MapFS{"etc/passwd": &fstest.MapFile{Data: []byte("egd:x:990:990::/home/egd:/bin/bash\n")}})
		require.NoError(t, err)
		require.Empty(t, environ)
	})
}
