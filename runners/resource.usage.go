package runners

import (
	"runtime"
	"sync"

	"github.com/pbnjay/memory"
)

func NewRuntimeResources() RuntimeResources {
	return RuntimeResources{
		Cores:  uint64(runtime.NumCPU()),
		Memory: memory.TotalMemory(),
	}
}

func NewRuntimeResourcesFromDequeued(d *Enqueued) RuntimeResources {
	return RuntimeResources{
		Cores:  d.Cores,
		Memory: d.Memory,
	}
}

type RuntimeResources struct {
	Cores  uint64
	Memory uint64
}

func (t RuntimeResources) Reserve(limits RuntimeResources) RuntimeResources {
	t.Cores += limits.Cores
	t.Memory += limits.Memory
	return t
}

func (t RuntimeResources) Release(limits RuntimeResources) RuntimeResources {
	t.Cores -= limits.Cores
	t.Memory -= limits.Memory
	return t
}

func NewResourceManager(limits RuntimeResources) *ResourceManager {
	return &ResourceManager{
		Limit:     limits,
		completed: make(chan struct{}, 1),
	}
}

type ResourceManager struct {
	m         sync.RWMutex
	Limit     RuntimeResources
	Current   RuntimeResources
	completed chan struct{}
}

func (t *ResourceManager) Completed() <-chan struct{} {
	return t.completed
}

func (t *ResourceManager) Reserve(limits RuntimeResources) RuntimeResources {
	t.m.Lock()
	defer t.m.Unlock()

	t.Current = t.Current.Reserve(limits)
	return t.Current
}

func (t *ResourceManager) Release(limits RuntimeResources) RuntimeResources {
	t.m.Lock()
	defer t.m.Unlock()

	t.Current = t.Current.Release(limits)
	select {
	case t.completed <- struct{}{}:
	default:
	}
	return t.Current
}

func (t *ResourceManager) Snapshot() RuntimeResources {
	t.m.Lock()
	defer t.m.Unlock()
	return t.Current
}
