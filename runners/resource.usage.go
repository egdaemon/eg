package runners

import (
	"context"
	"math"
	"runtime"
	"sync"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/pbnjay/memory"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

func NewRuntimeResources() *RuntimeResources {
	return &RuntimeResources{
		Cores:  uint64(runtime.NumCPU()),
		Memory: memory.TotalMemory(),
	}
}

func NewRuntimeResourcesFromDequeued(d *Enqueued) *RuntimeResources {
	return &RuntimeResources{
		Cores:  d.Cores,
		Memory: d.Memory,
	}
}

type RuntimeResources struct {
	m      sync.RWMutex
	Cores  uint64
	Memory uint64
}

func (t *RuntimeResources) Reserve(limits *RuntimeResources) {
	t.m.Lock()
	defer t.m.Unlock()
	t.Cores -= limits.Cores
	t.Memory -= limits.Memory
}

func (t *RuntimeResources) Release(limits *RuntimeResources) {
	t.m.Lock()
	defer t.m.Unlock()
	t.Cores += limits.Cores
	t.Memory += limits.Memory
}

func calculateLoad(loads []float64) (_ uint64) {
	var (
		sum float64
	)

	for _, l := range loads {
		sum += l
	}

	return uint64(math.Ceil(sum))
}

func determineload(ctx context.Context) (_ *RuntimeResources, err error) {
	var (
		l RuntimeResources
	)

	if cpu, err := cpu.PercentWithContext(ctx, 0, false); err != nil {
		return nil, errorsx.Wrap(err, "unable to retrieve compute")
	} else {
		l.Cores = calculateLoad(cpu)
	}

	if usage, err := mem.VirtualMemoryWithContext(ctx); err != nil {
		return nil, errorsx.Wrap(err, "unable to retrieve memory usage")
	} else {
		l.Memory = usage.Available
	}

	return &l, nil
}
