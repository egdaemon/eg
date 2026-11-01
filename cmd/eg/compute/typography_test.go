package compute

import (
	"fmt"
	"testing"
	"time"

	"github.com/egdaemon/eg/compute"
	"github.com/stretchr/testify/require"
)

func TestTypographyRunnerCompact(t *testing.T) {
	const id = "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"

	require.Equal(t, id, fmt.Sprintf("%v", TypographyRunnerCompact(&compute.Compute{Id: id})))
	require.Equal(t, id, fmt.Sprintf("%v", TypographyRunnerCompact(&compute.Compute{Id: id, Description: "  "})))
	require.Equal(t, "0190a1b2 - box", fmt.Sprintf("%v", TypographyRunnerCompact(&compute.Compute{Id: id, Description: "box"})))
	require.Equal(t, "0190a1b2 - bo", fmt.Sprintf("%.13v", TypographyRunnerCompact(&compute.Compute{Id: id, Description: "box"})))
	require.Equal(t, "", fmt.Sprintf("%v", TypographyRunnerCompact(nil)))
	require.Equal(t, "0190a1b2 - box  |", fmt.Sprintf("%-16v|", TypographyRunnerCompact(&compute.Compute{Id: id, Description: "box"})))
}

func TestTypographyRunnerPlatform(t *testing.T) {
	require.Equal(t, "linux/amd64", fmt.Sprintf("%v", TypographyRunnerPlatform(&compute.Compute{Os: "linux", Arch: "amd64"})))
	require.Equal(t, "/", fmt.Sprintf("%v", TypographyRunnerPlatform(nil)))
}

func TestTypographyRunnerLabels(t *testing.T) {
	require.Equal(t, "x,y", fmt.Sprintf("%v", TypographyRunnerLabels(&compute.Compute{Labels: []string{"x", "y"}})))
	require.Equal(t, "", fmt.Sprintf("%v", TypographyRunnerLabels(&compute.Compute{})))
	require.Equal(t, "", fmt.Sprintf("%v", TypographyRunnerLabels(nil)))
}

func TestTypographyRunnerLastSeen(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	require.Equal(t, ts.Local().Format(time.DateTime), fmt.Sprintf("%v", TypographyRunnerLastSeen(&compute.Compute{UpdatedAt: ts.Format(time.RFC3339Nano)})))
	require.Equal(t, "garbage", fmt.Sprintf("%v", TypographyRunnerLastSeen(&compute.Compute{UpdatedAt: "garbage"})))
	require.Equal(t, "", fmt.Sprintf("%v", TypographyRunnerLastSeen(nil)))
}

func TestTypographyBytes(t *testing.T) {
	require.Equal(t, "0B", fmt.Sprintf("%v", TypographyBytes(0)))
	require.Equal(t, "512B", fmt.Sprintf("%v", TypographyBytes(512)))
	require.Equal(t, "512B", fmt.Sprintf("%.3v", TypographyBytes(512)))
	require.Equal(t, "1.00KiB", fmt.Sprintf("%v", TypographyBytes(1<<10)))
	require.Equal(t, "1.00MiB", fmt.Sprintf("%v", TypographyBytes(1<<20)))
	require.Equal(t, "2.00GiB", fmt.Sprintf("%v", TypographyBytes(2<<30)))
	require.Equal(t, "3.00TiB", fmt.Sprintf("%v", TypographyBytes(3<<40)))
	require.Equal(t, "4.32GiB", fmt.Sprintf("%v", TypographyBytes(4638564311)))
	require.Equal(t, "4GiB", fmt.Sprintf("%.0v", TypographyBytes(4638564311)))
	require.Equal(t, "4.3GiB", fmt.Sprintf("%.1v", TypographyBytes(4638564311)))
	require.Equal(t, "4.32GiB", fmt.Sprintf("%.2v", TypographyBytes(4638564311)))
	require.Equal(t, "4.320GiB", fmt.Sprintf("%.3v", TypographyBytes(4638564311)))
	require.Equal(t, "1024KiB", fmt.Sprintf("%.0v", TypographyBytes(1<<20-1)))
	require.Equal(t, "  2.00GiB", fmt.Sprintf("%9v", TypographyBytes(2<<30)))
	require.Equal(t, "2.00GiB  |", fmt.Sprintf("%-9v|", TypographyBytes(2<<30)))
	require.Equal(t, "   4.3GiB", fmt.Sprintf("%9.1v", TypographyBytes(4638564311)))
}
