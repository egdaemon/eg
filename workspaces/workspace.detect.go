package workspaces

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/go-git/go-git/v6/plumbing/format/gitignore"
)

const (
	// EgrcFile holds the command line arguments of a workload, one per line, and the modes it supports, see ReadDetected.
	EgrcFile = ".egrc"
	// EgrcIgnoreFile holds gitignore style patterns of the workloads to ignore, see Ignored.
	EgrcIgnoreFile = ".egrcignore"
)

// ParseEgrc the arguments of an .egrc file: one argument per line, passed as written (no shell splitting, so a
// flag and its value are `--flag=value`). blank lines and lines starting with # are ignored.
func ParseEgrc(text string) []string {
	args := []string(nil)
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			args = append(args, line)
		}
	}

	return args
}

var envpattern = regexp.MustCompile(`\$(\$|\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))`)

// ExpandEnv replaces $NAME and ${NAME} with the value lookup returns, an unset variable is empty and $$ is a literal $.
func ExpandEnv(text string, lookup func(string) string) string {
	return envpattern.ReplaceAllStringFunc(text, func(match string) string {
		groups := envpattern.FindStringSubmatch(match)
		if groups[1] == "$" {
			return "$"
		}

		return lookup(groups[2] + groups[3])
	})
}

// Mode how a workload is run: the eg compute command it is run with.
type Mode string

const (
	ModeLocal  Mode = "local"
	ModeUpload Mode = "upload"
	ModeServe  Mode = "serve"
	// ModeBaremetal runs the workload directly on the host, inherently unsafe.
	ModeBaremetal Mode = "baremetal"
)

// Detected is one way a workload of a repository runs. a workload that supports several modes is several Detected.
type Detected struct {
	Workload string   // relative to the module directory, blank for the default workload.
	Dir      string   // absolute path of the workload.
	Mode     Mode     // the eg compute command that runs it.
	Args     []string // the arguments of the mode with environment variables expanded.
	Ignored  bool     // an .egrcignore hides the workload, see Ignored.
	// Command the arguments after eg that run the workload this way, the workload's own arguments are flags so they
	// come before the workload. set by DetectWorkloads, which knows the repository.
	Command []string
}

// ReadDetected reads the .egrc beside the workload in dir: one Detected per mode it declares. an .egrc is one
// argument per line, and a line `[local]`, `[upload]`, `[serve]` or `[baremetal]` declares a mode, the lines up to the next
// header are its arguments. lines before the first header are common to every declared mode. without a header, or
// without an .egrc, the workload supports local only.
func ReadDetected(dir string, lookup func(string) string) ([]Detected, error) {
	path := filepath.Join(dir, EgrcFile)
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return []Detected{{Dir: dir, Mode: ModeLocal}}, nil
	}
	if err != nil {
		return nil, errorsx.Wrapf(err, "unable to read %s", path)
	}

	var (
		common  []string
		modes   []Mode
		args    = make(map[Mode][]string)
		current Mode
	)

	for _, line := range ParseEgrc(string(raw)) {
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = Mode(line[1 : len(line)-1])
			switch current {
			case ModeLocal, ModeUpload, ModeServe, ModeBaremetal:
			default:
				return nil, errorsx.Errorf("%s: unknown mode %s, expected [local], [upload], [serve] or [baremetal]", path, line)
			}

			if _, declared := args[current]; !declared {
				modes = append(modes, current)
				args[current] = nil
			}

			continue
		}

		if line = ExpandEnv(line, lookup); current == "" {
			common = append(common, line)
		} else {
			args[current] = append(args[current], line)
		}
	}

	if len(modes) == 0 {
		return []Detected{{Dir: dir, Mode: ModeLocal, Args: common}}, nil
	}

	results := make([]Detected, 0, len(modes))
	for _, mode := range modes {
		results = append(results, Detected{Dir: dir, Mode: mode, Args: slices.Concat(common, args[mode])})
	}

	return results, nil
}

// Ignored reports whether the workload at dir, within the repository at root, is ignored. every directory from the
// root down to the workload may hold an .egrcignore, whose patterns are appended to those of the directories above
// it, so a deeper file can ignore more or, with a leading !, include again what a parent ignored. the syntax is
// gitignore's, patterns are relative to the directory holding the file. an .egrcignore without any pattern
// ignores its own directory and everything below it. as with git, what is below an ignored directory cannot be
// included again.
func Ignored(root, dir string) (bool, error) {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, errorsx.Errorf("%s is not within %s", dir, root)
	}

	var (
		segments = []string(nil)
		patterns = []gitignore.Pattern(nil)
	)

	if rel != "." {
		segments = strings.Split(filepath.ToSlash(rel), "/")
	}

	// the root is level 0, each segment descends one level.
	for level := 0; level <= len(segments); level++ {
		levelpath := filepath.Join(append([]string{root}, segments[:level]...)...)
		raw, err := os.ReadFile(filepath.Join(levelpath, EgrcIgnoreFile))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, errorsx.Wrapf(err, "unable to read %s", filepath.Join(levelpath, EgrcIgnoreFile))
		}

		lines := ParseEgrc(string(raw))
		if len(lines) == 0 && len(bytes.TrimSpace(raw)) == 0 {
			// an empty file ignores its own directory.
			return true, nil
		}

		for _, line := range lines {
			patterns = append(patterns, gitignore.ParsePattern(line, slices.Clone(segments[:level])))
		}
	}

	matcher := gitignore.NewMatcher(patterns)
	for level := 1; level <= len(segments); level++ {
		if matcher.Match(segments[:level], true) {
			return true, nil
		}
	}

	return false, nil
}

// DetectWorkloads the workloads of the repository at root: the main packages within its module directory. ignored
// workloads are kept and marked Ignored, see Ignored, so tools can decide whether to show them. sorted by workload,
// the default workload, the module directory itself, first, which means the module directory is read before the
// first one is yielded.
func DetectWorkloads(root string, lookup func(string) string) iterx.Seq[Detected] {
	return iterx.New(func(ctx context.Context, yield func(Detected) bool) (err error) {
		moduledir := filepath.Join(root, eg.DefaultModuleDirectory())
		if stat, err := os.Stat(moduledir); err != nil || !stat.IsDir() {
			return nil
		}

		var found []Detected

		seq := Workloads(ctx, moduledir)
		for d := range seq.Each(ctx) {
			if d.Name != "main" {
				continue
			}

			ignored, err := Ignored(root, d.Dir)
			if err != nil {
				return err
			}

			ways, err := ReadDetected(d.Dir, lookup)
			if err != nil {
				return err
			}

			for _, detected := range ways {
				detected.Ignored = ignored
				if d.Path != "." {
					detected.Workload = filepath.ToSlash(d.Path)
				}

				detected.Command = detected.command(root)
				found = append(found, detected)
			}
		}

		if err = seq.Err(); err != nil {
			return err
		}

		// the order is only for stable output.
		slices.SortStableFunc(found, func(a, b Detected) int {
			return cmp.Or(strings.Compare(a.Workload, b.Workload), strings.Compare(string(a.Mode), string(b.Mode)))
		})

		for _, d := range found {
			if !yield(d) {
				return nil
			}
		}

		return nil
	})
}

// command the arguments after eg that run the workload of the repository at root this way.
func (t Detected) command(root string) []string {
	command := []string{"compute", string(t.Mode), "--directory", root}
	if t.Mode == ModeServe {
		command = append(command, "--ssh")
	}

	command = append(command, t.Args...)
	if t.Workload != "" {
		command = append(command, t.Workload)
	}

	return command
}
