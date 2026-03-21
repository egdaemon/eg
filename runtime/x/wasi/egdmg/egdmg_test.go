package egdmg_test

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/internal/unsafepretty"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egdmg"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	t.Run("example1", func(t *testing.T) {
		tmpdir := testx.PrivateTemp(t)
		r := &shell.Recorder{}
		rt := shell.Runtime().UnsafeExec(r.Record).As("egd")

		b := egdmg.New("eg", egdmg.OptionRuntime(rt), egdmg.OptionMkisofs)
		require.Error(t, fsx.SymlinkExists(egenv.EphemeralDirectory("Applications")))

		require.NoError(t, egdmg.Build(b, testx.Fixture("example1"))(t.Context(), egtest.Op()))

		require.True(t, func(cmds ...string) bool {
			check := func(cmd, expected string) bool {

				return strings.HasPrefix(strings.TrimSpace(cmd), strings.TrimSpace(expected))
			}

			seq := []string{
				fmt.Sprintf("::sudo:-E -H -u egd -g egd bash -c cp -R %s/ %s/eg/ ", testx.Fixture("example1"), tmpdir),
				"::sudo:-E -H -u egd -g egd bash -c ln -fs /Applications ",
				"::sudo:-E -H -u egd -g egd bash -c mkisofs -D -R -apple -no-pad -V eg -o /workload/.eg.workspace/eg.dmg ",
			}
			if len(cmds) != len(seq) {
				log.Println("invalid number of commands", len(cmds), "vs", len(seq))
				return false
			}

			for idx, v := range cmds {
				if check(v, seq[idx]) {

					continue
				}
				log.Println("invalid command", idx, "\n", unsafepretty.Print(v), "\n", unsafepretty.Print(seq[idx]))
				return false
			}

			return true
		}(r.Results()...), r.Results())
	})

	t.Run("copies archive contents into staging directory", func(t *testing.T) {
		tmpdir := testx.PrivateTemp(t)
		b := egdmg.New("eg", egdmg.OptionRuntime(shell.NewLocal()), egdmg.OptionDmgCmd("true"))
		require.NoError(t, egdmg.Build(b, testx.Fixture("example1"))(t.Context(), egtest.Op()))

		require.NoError(t, fsx.DirExists(filepath.Join(tmpdir, "eg")))
		require.Equal(t, testx.ReadMD5(testx.Fixture("example1", "hello.world.txt")), testx.ReadMD5(filepath.Join(tmpdir, "eg", "hello.world.txt")))
		require.Equal(t, testx.ReadMD5(testx.Fixture("example1", "Contents", "MacOS", "bin")), testx.ReadMD5(filepath.Join(tmpdir, "eg", "Contents", "MacOS", "bin")))
		require.Equal(t, testx.ReadMD5(testx.Fixture("example1", "Contents", "Resources", "icon.icns")), testx.ReadMD5(filepath.Join(tmpdir, "eg", "Contents", "Resources", "icon.icns")))
		require.NotEqual(t, testx.ReadMD5(testx.Fixture("example1", "Contents", "Info.plist")), testx.ReadMD5(filepath.Join(tmpdir, "eg", "Contents", "Info.plist")), testx.ReadString(filepath.Join(tmpdir, "eg", "Contents", "Info.plist")))
		require.Equal(t, "1cbafcbe-a85d-7a8f-5578-a1215753ff1f", testx.ReadMD5(filepath.Join(tmpdir, "eg", "Contents", "Info.plist")), testx.ReadString(filepath.Join(tmpdir, "eg", "Contents", "Info.plist")))
		require.NoError(t, fsx.SymlinkExists(filepath.Join(tmpdir, "eg", "Applications")))
	})

	t.Run("applications symlink inside srcfolder", func(t *testing.T) {
		tmpdir := testx.PrivateTemp(t)
		r := &shell.Recorder{}
		rt := shell.Runtime().UnsafeExec(r.Record).As("egd")

		b := egdmg.New("eg", egdmg.OptionRuntime(rt), egdmg.OptionMkisofs)
		require.NoError(t, egdmg.Build(b, testx.Fixture("example1"))(t.Context(), egtest.Op()))

		cmds := r.Results()
		require.Len(t, cmds, 3)
		require.Contains(t, cmds[1], filepath.Join(tmpdir, "eg", "Applications"))
	})

	t.Run("option output dir sets outputpath", func(t *testing.T) {
		tmpdir := testx.PrivateTemp(t)
		r := &shell.Recorder{}
		rt := shell.Runtime().UnsafeExec(r.Record).As("egd")

		b := egdmg.New("eg", egdmg.OptionRuntime(rt), egdmg.OptionMkisofs, egdmg.OptionOutputDir("/custom/output"))
		require.NoError(t, egdmg.Build(b, testx.Fixture("example1"))(t.Context(), egtest.Op()))

		cmds := r.Results()
		require.Len(t, cmds, 3)
		// cp and symlink should still use the default builddir, not the output dir
		require.Contains(t, cmds[0], filepath.Join(tmpdir, "eg"))
		// mkisofs output should use the custom output dir
		require.Contains(t, cmds[2], filepath.Join("/custom/output", "eg.dmg"))
	})

	t.Run("option output name sets outputname", func(t *testing.T) {
		tmpdir := testx.PrivateTemp(t)
		r := &shell.Recorder{}
		rt := shell.Runtime().UnsafeExec(r.Record).As("egd")

		b := egdmg.New("eg", egdmg.OptionRuntime(rt), egdmg.OptionMkisofs, egdmg.OptionOutputName("custom.dmg"))
		require.NoError(t, egdmg.Build(b, testx.Fixture("example1"))(t.Context(), egtest.Op()))

		cmds := r.Results()
		require.Len(t, cmds, 3)
		// cp and symlink should still use the default builddir
		require.Contains(t, cmds[0], filepath.Join(tmpdir, "eg"))
		// mkisofs output should use the custom name
		require.Contains(t, cmds[2], "custom.dmg")
		// mkisofs output should not use the default name
		require.NotContains(t, cmds[2], "eg.dmg")
	})
}
