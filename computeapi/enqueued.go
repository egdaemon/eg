package computeapi

import (
	"fmt"
	"time"

	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/schedules"
)

// EnqueueFromSchedule the workload the scheduler enqueues for the schedule when it is due, enqueue it with PushClient.Enqueue
// to run it now. the id and account are left for the control plane to assign. the schedule's ttl is nanoseconds
// while the queue takes milliseconds, and an unset architecture and os default to amd64 and linux.
func EnqueueFromSchedule(s *schedules.Schedule) *Enqueued {
	return &Enqueued{
		Description: fmt.Sprintf("scheduled workload - %s", s.GetDescription()),
		Cores:       s.GetCores(),
		Memory:      s.GetMemory(),
		Vram:        s.GetVram(),
		Ttl:         uint64(time.Duration(s.GetTtl()) / time.Millisecond),
		Arch:        stringsx.FirstNonBlank(s.GetArch(), "amd64"),
		Os:          stringsx.FirstNonBlank(s.GetOs(), "linux"),
		VcsUri:      s.GetVcsUri(),
		Mimetype:    s.GetMimetype(),
		Labels:      append([]string{}, s.GetLabels()...),
		SecretUris:  append([]string{}, s.GetSecretUris()...),
		Entry:       s.GetEntry(),
	}
}
