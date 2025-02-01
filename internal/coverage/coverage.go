package coverage

import "github.com/egdaemon/eg/interp/events"

const Metric = "eg.metrics.coverage"

type Report = events.Coverage_Report

func Batch(rep ...*Report) *events.Coverage {
	return &events.Coverage{
		Reports: rep,
	}
}
