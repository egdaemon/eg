package computeapi

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/schedules"
	"github.com/stretchr/testify/require"
)

func TestEnqueueFromSchedule(t *testing.T) {
	t.Run("maps the schedule the way the scheduler does", func(t *testing.T) {
		enq := EnqueueFromSchedule(&schedules.Schedule{
			Id:          "s1",
			AccountId:   "a1",
			Description: "nightly",
			VcsUri:      "git@example.com:o/r.git",
			Mimetype:    "application/git",
			Entry:       "release/eg/debian",
			Cores:       4,
			Memory:      8 << 30,
			Vram:        2,
			Ttl:         uint64(time.Hour),
			Labels:      []string{"gpu"},
			SecretUris:  []string{"gcpsm://p/s"},
		})

		require.Empty(t, enq.GetId())
		require.Empty(t, enq.GetAccountId())
		require.Equal(t, "scheduled workload - nightly", enq.GetDescription())
		require.Equal(t, "git@example.com:o/r.git", enq.GetVcsUri())
		require.Equal(t, "application/git", enq.GetMimetype())
		require.Equal(t, "release/eg/debian", enq.GetEntry())
		require.Equal(t, uint64(4), enq.GetCores())
		require.Equal(t, uint64(8<<30), enq.GetMemory())
		require.Equal(t, uint64(2), enq.GetVram())
		require.Equal(t, uint64(time.Hour/time.Millisecond), enq.GetTtl())
		require.Equal(t, "amd64", enq.GetArch())
		require.Equal(t, "linux", enq.GetOs())
		require.Equal(t, []string{"gpu"}, enq.GetLabels())
		require.Equal(t, []string{"gcpsm://p/s"}, enq.GetSecretUris())
	})

	t.Run("keeps the architecture and os of the schedule", func(t *testing.T) {
		enq := EnqueueFromSchedule(&schedules.Schedule{Arch: "arm64", Os: "darwin"})
		require.Equal(t, "arm64", enq.GetArch())
		require.Equal(t, "darwin", enq.GetOs())
	})
}
