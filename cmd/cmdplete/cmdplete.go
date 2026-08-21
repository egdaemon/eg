package cmdplete

import (
	"context"
	"log"

	"github.com/egdaemon/eg/workspaces"
	"github.com/posener/complete"
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
	ctx := context.Background()
	seq := workspaces.Workloads(ctx, t.root)
	for d := range seq.Each(ctx) {
		results = append(results, d.Path)
	}

	if err := seq.Err(); err != nil {
		log.Println("unable to predict workloads available", t.root, err)
		return nil
	}

	return results
}
