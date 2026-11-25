package cmdopts

import (
	"fmt"
	"os"
	"strings"

	"github.com/egdaemon/eg/internal/buildinfo"
	"github.com/logrusorgru/aurora"
	"github.com/mattn/go-isatty"
)

type Version struct{}

func (t Version) Run(ctx *Global) (err error) {
	infos, err := buildinfo.Info()
	if err != nil {
		return err
	}

	if _, err = fmt.Println(infos); err != nil {
		return err
	}

	if strings.Contains(infos, ".dirty") {
		au := aurora.NewAurora(isatty.IsTerminal(os.Stdout.Fd()))
		if _, err = fmt.Println(au.Red("unsupported modified build")); err != nil {
			return err
		}
	}

	return nil
}

// BuildInfo see buildinfo.Info.
func BuildInfo() (string, error) { return buildinfo.Info() }

// BuildCommit see buildinfo.Commit.
func BuildCommit() string { return buildinfo.Commit() }

// BuildInfoSafe see buildinfo.Safe.
func BuildInfoSafe() string { return buildinfo.Safe() }

// ModPath see buildinfo.ModPath.
func ModPath() string { return buildinfo.ModPath() }
