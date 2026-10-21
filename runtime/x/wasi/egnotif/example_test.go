package egnotif_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/egdaemon/eg/runtime/x/wasi/egnotif"
)

func ExampleNew() {
	notify := egnotif.New(egnotif.Log(os.Stdout, egnotif.LogOptions().Attrs("service", "example")...))

	// nil means the workload completed.
	if err := notify.Fatalln(context.Background(), nil); err != nil {
		fmt.Println("unexpected error", err)
	}
}

func ExampleOptions() {
	notify := egnotif.New(
		egnotif.Log(io.Discard, egnotif.LogOptions()...),
		egnotif.Options().
			Failure("**{{.Workload}} failed** after {{.Elapsed}}: {{.Cause}}").
			Completed("**{{.Workload}} completed** in {{.Elapsed}}").
			Timeout(10*time.Second)...,
	)

	// the failure is notified and returned so the module still fails.
	err := notify.Fatalln(context.Background(), errors.New("tests failed"))
	fmt.Println(err)
	// Output: tests failed
}

func ExampleLogOptions() {
	// logs completed workloads at warn instead of info.
	_ = egnotif.Log(os.Stderr, egnotif.LogOptions().Level(slog.LevelWarn).Attrs("team", "platform")...)
}

func ExampleSMTP() {
	_ = egnotif.SMTP(
		"smtp.example.com:587",
		egnotif.SMTPOptions().
			From("eg@example.com").
			To("team@example.com").
			Subject("eg workload {{if .Failed}}failed{{else}}completed{{end}}: {{.VCSURI}}")...,
	)
}

type stdout struct{}

func (stdout) Send(ctx context.Context, v egnotif.Context) error {
	msg, err := v.Message()
	if err != nil {
		return err
	}

	fmt.Println(msg)
	return nil
}

// any type with a Send method is a deliverer.
func Example_customDeliverer() {
	notify := egnotif.New(stdout{}, egnotif.Options().Completed("done")...)
	_ = notify.Fatalln(context.Background(), nil)
	// Output: done
}
