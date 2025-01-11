package daemons

import (
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"golang.org/x/crypto/ssh"
	"golang.org/x/time/rate"
)

// manages system usage to saturate the system but maintain performance.
type SystemUsage struct {
	maximum   *cmdopts.RuntimeResources
	available *cmdopts.RuntimeResources
}

func (t *SystemUsage) Current() cmdopts.RuntimeResources {
	return *t.available
}

func NewSystemUsage(gctx *cmdopts.Global, runtimecfg *cmdopts.RuntimeResources, aid, machineid string, s ssh.Signer) *SystemUsage {
	r := rate.NewLimiter(rate.Every(envx.Duration(20*time.Second, eg.EnvScheduleSystemLoadFreq)), 1)

	for err := r.Wait(gctx.Context); err == nil; err = r.Wait(gctx.Context) {

	}

	return nil
}
