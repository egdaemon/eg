package cmdopts

import (
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
)

type Version struct{}

func (t Version) Run(ctx *Global) (err error) {
	var (
		ok    bool
		info  *debug.BuildInfo
		ts    time.Time
		id    string
		dirty bool
	)

	if info, ok = debug.ReadBuildInfo(); ok {
		au := aurora.NewAurora(isatty.IsTerminal(os.Stdout.Fd()))
		for _, v := range info.Settings {
			switch v.Key {
			case "vcs.modified":
				if dirty, err = strconv.ParseBool(v.Value); err != nil {
					return err
				}
			case "vcs.revision":
				id = v.Value
			case "vcs.time":
				if ts, err = time.Parse(time.RFC3339, v.Value); err != nil {
					return err
				}
			}
		}

		if _, err = fmt.Println(info.Main.Path, ts.Format("2006-01-02"), id); err != nil {
			return err
		}

		if dirty {
			if _, err = fmt.Println(au.Red("unsupported modified build")); err != nil {
				return err
			}
		}

		return nil
	}

	return errors.New("unable to detect build information")
}
