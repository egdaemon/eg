package egtarball

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTarflag(t *testing.T) {
	t.Run("tar.xz is xz", func(t *testing.T) {
		assert.Equal(t, "-J", tarcompressionflag("myarchive.tar.xz"))
		assert.Equal(t, "-J", tarcompressionflag("path/to/archive.tar.xz"))
	})
	t.Run("txz is xz", func(t *testing.T) {
		assert.Equal(t, "-J", tarcompressionflag("myarchive.txz"))
	})
	t.Run("tar.gz is gzip", func(t *testing.T) {
		assert.Equal(t, "-z", tarcompressionflag("myarchive.tar.gz"))
		assert.Equal(t, "-z", tarcompressionflag("path/to/archive.tar.gz"))
	})
	t.Run("tgz is gzip", func(t *testing.T) {
		assert.Equal(t, "-z", tarcompressionflag("myarchive.tgz"))
	})
	t.Run("extension is case insensitive", func(t *testing.T) {
		assert.Equal(t, "-z", tarcompressionflag("MYARCHIVE.TAR.GZ"))
		assert.Equal(t, "-J", tarcompressionflag("MYARCHIVE.TAR.XZ"))
	})
	t.Run("only the suffix counts", func(t *testing.T) {
		assert.Equal(t, "-J", tarcompressionflag("my.tar.gz.archive"))
		assert.Equal(t, "-z", tarcompressionflag("my.tar.xz.tar.gz"))
	})
	t.Run("unrecognized extensions keep the historical xz default", func(t *testing.T) {
		assert.Equal(t, "-J", tarcompressionflag("myarchive"))
		assert.Equal(t, "-J", tarcompressionflag("myarchive.tar"))
		assert.Equal(t, "-J", tarcompressionflag(""))
	})
	t.Run("resolved git pattern keeps its extension", func(t *testing.T) {
		assert.Equal(t, "-z", tarcompressionflag(Targz(GitPattern("example"))))
		assert.Equal(t, "-J", tarcompressionflag(Tarxz(GitPattern("example"))))
	})
}
