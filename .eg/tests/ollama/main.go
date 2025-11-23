package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/x/wasi/eggengolang"
	"github.com/egdaemon/eg/runtime/x/wasi/egollama"
)

const (
	code = `package example

// generates a *consistent* duration based on the input i within the
// provided window. this isn't the best location for these functions.
// but the lack of a better location.
func DynamicHashDuration(window time.Duration, i string) time.Duration {
	if window == 0 {
		return 0
	}

	return time.Duration(DynamicHashWindow(i, uint64(window)))
}

func DynamicHashHour(i string) time.Duration {
	return DynamicHashDuration(60*time.Minute, i)
}

func DynamicHash45m(i string) time.Duration {
	return DynamicHashDuration(45*time.Minute, i)
}

func DynamicHash15m(i string) time.Duration {
	return DynamicHashDuration(15*time.Minute, i)
}

func DynamicHash5m(i string) time.Duration {
	return DynamicHashDuration(5*time.Minute, i)
}

func DynamicHashDay(i string) time.Weekday {
	return time.Weekday(DynamicHashWindow(i, 7))
}

// uint64 to prevent negative values
func DynamicHashWindow(i string, n uint64) uint64 {
	digest := md5.Sum([]byte(i))
	return binary.LittleEndian.Uint64(digest[:]) % n
}

// generates a random duration from the provided range.
func RandomFromRange[T numericx.Integer | time.Duration](r T) T {
	return T(rand.Intn(int(r)))
}
`

	style = `func TestContainerRunnerClone(t *testing.T) {
	t.Run("clone changes should not impact original", func(t *testing.T) {
		o := Container("derp")
		dup := o.Clone().OptionEnv("FOO", "BAR").Command("echo ${FOO}")
		require.Empty(t, o.options)
		require.Empty(t, o.cmd)
		require.Len(t, dup.options, 1)
		require.Equal(t, []string{"echo", "${FOO}"}, dup.cmd)
	})
}`
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eg.Build(eg.DefaultModule()),
		egollama.Prepare(egollama.Runner()),
		eg.Module(
			ctx,
			egollama.Runner(),
			eggengolang.ImproveTestCoverage(
				code,
				"DynamicHashHour",
				style,
				"func() { DynamicHashHour(\"foo\") }",
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
