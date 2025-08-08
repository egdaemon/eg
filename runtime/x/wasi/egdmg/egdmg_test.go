package egdmg_test

import (
	"log"
	"strings"
	"testing"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egdmg"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	t.Run("example1", func(t *testing.T) {
		testx.PrivateTemp(t)
		r := &shell.Recorder{}
		rt := shell.Runtime().UnsafeExec(r.Record).As("egd")

		b := egdmg.New("eg", egdmg.OptionRuntime(rt))
		require.Error(t, fsx.SymlinkExists(egenv.EphemeralDirectory("Applications")))

		require.NoError(t, egdmg.Build(b, testx.Fixture("example1"))(t.Context(), egtest.Op()))

		// fsx.PrintFS(os.DirFS(egenv.EphemeralDirectory()))

		// TODO:
		// require.NoError(t, fsx.DirExists(egenv.EphemeralDirectory("eg.app")))
		// require.Equal(t, testx.ReadMD5(testx.Fixture("example1", "hello.world.txt")), testx.ReadMD5(egenv.EphemeralDirectory("eg.app", "hello.world.txt")))
		// require.Equal(t, testx.ReadMD5(testx.Fixture("example1", "Contents", "MacOS", "bin")), testx.ReadMD5(egenv.EphemeralDirectory("eg.app", "Contents", "MacOS", "bin")))
		// require.Equal(t, testx.ReadMD5(testx.Fixture("example1", "Contents", "Resources", "icon.icns")), testx.ReadMD5(egenv.EphemeralDirectory("eg.app", "Contents", "Resources", "icon.icns")))
		// require.NotEqual(t, testx.ReadMD5(testx.Fixture("example1", "Contents", "Info.plist")), testx.ReadMD5(egenv.EphemeralDirectory("eg.app", "Contents", "Info.plist")), testx.ReadString(egenv.EphemeralDirectory("eg.app", "Contents", "Info.plist")))
		// require.Equal(t, "930df5f0-b121-133a-8d2a-51ed2a420683", testx.ReadMD5(egenv.EphemeralDirectory("eg.app", "Contents", "Info.plist")), testx.ReadString(egenv.EphemeralDirectory("eg.app", "Contents", "Info.plist")))
		require.True(t, func(cmds ...string) bool {
			check := func(cmd, expected string) bool {
				return strings.HasPrefix(cmd, expected)
			}

			seq := []string{
				"::sudo:-E -H -u egd -g egd bash -c rsync -avL ",
				"::sudo:-E -H -u egd -g egd bash -c ln -fs /Applications ",
				"::sudo:-E -H -u egd -g egd bash -c mkisofs -D -R -apple -no-pad -V eg.app -o /workload/.eg.workspace/eg.dmg ",
			}
			if len(cmds) != len(seq) {
				log.Println("invalid number of commands", len(cmds), "vs", len(seq))
				return false
			}

			for idx, v := range cmds {
				if check(v, seq[idx]) {

					continue
				}
				log.Println("invalid command\n", v, "\n", seq[idx])
				return false
			}

			return true
		}(r.Results()...), r.Results())
	})
}
