package runners

import (
	"log"
	"runtime"
	"sync"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/pbnjay/memory"
)

func NewRuntimeResources() RuntimeResources {
	_, vram, err := DetectGPU()
	if err != nil {
		log.Println(errorsx.Wrap(err, "unable to detect gpu, vram capacity defaulting to 0"))
	}

	return RuntimeResources{
		Cores:  uint64(runtime.NumCPU()),
		Memory: memory.TotalMemory(),
		Vram:   vram,
	}
}

func NewRuntimeResourcesFromDequeued(d *Enqueued) RuntimeResources {
	return RuntimeResources{
		Cores:  d.Cores,
		Memory: d.Memory,
		Vram:   d.Vram,
	}
}

type RuntimeResources struct {
	Cores  uint64
	Memory uint64
	Vram   uint64
}

func (t RuntimeResources) Reserve(limits RuntimeResources) RuntimeResources {
	t.Cores += limits.Cores
	t.Memory += limits.Memory
	t.Vram += limits.Vram
	return t
}

func (t RuntimeResources) Release(limits RuntimeResources) RuntimeResources {
	t.Cores -= limits.Cores
	t.Memory -= limits.Memory
	t.Vram -= limits.Vram
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

// Available returns the remaining, unreserved capacity (Limit - Current).
func (t *ResourceManager) Available() RuntimeResources {
	t.m.Lock()
	defer t.m.Unlock()
	return t.Limit.Release(t.Current)
}
