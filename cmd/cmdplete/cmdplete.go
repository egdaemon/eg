package cmdplete

import (
	"log"
	"path/filepath"

	"github.com/egdaemon/eg/astcodec"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/posener/complete"
	"golang.org/x/tools/go/packages"
)

// will initialize data if necessary when prediction is called using the provided init function.
func InitializingPrediction(init func() error, p complete.Predictor) complete.Predictor {
	return initializing{init: init, p: p}
}

type initializing struct {
	init func() error
	p    complete.Predictor
}

func (t initializing) Predict(args complete.Args) (results []string) {
	if err := t.init(); err != nil {
		log.Println("failed to prepare the prediction, return nothing", err)
		return []string(nil)
	}

	return t.p.Predict(args)
}

func NewWorkload(root string) Workload {
	return Workload{
		root: root,
	}
}

type Workload struct {
	root string
}

func (t Workload) Predict(args complete.Args) (results []string) {
	var (
		err  error
		pset []*packages.Package
	)

	pkgc := astcodec.DefaultPkgLoad(
		astcodec.LoadDir(t.root),
		astcodec.AutoFileSet,
		astcodec.DisableGowork, // dont want to do this but until I figure out the issue.
	)

	if pset, err = packages.Load(pkgc, "./..."); err != nil {
		log.Println("unable to predict workloads available", t.root, err)
		return nil
	}

	for _, pkg := range pset {
		var (
			err error
			m   string
		)

		if !pkg.Module.Main {
			continue
		}

		if m, err = filepath.Rel(t.root, pkg.Dir); err != nil {
			debugx.Println("unable to determine path", pkg.Name, pkg.Dir, err)
			continue
		}

		results = append(results, m)
	}

	return results
}
