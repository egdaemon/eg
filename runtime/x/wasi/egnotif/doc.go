/*
Package egnotif sends notifications about the outcome of a workload to messaging platforms.

The package provides the api, platforms are implemented as deliverers which are responsible
for their own configuration. slog and smtp deliverers are builtin since they only require the
standard library. Any type with a Send(ctx context.Context, v Context) error method can be
used as a deliverer, which allows platforms such as chat services to be implemented outside
of this package.

# Basic Usage

Create a notifier when the module starts and pass the result of the work to Fatalln. A nil
error sends the completed message, a non-nil error sends the failure message. The error is
returned so the module still fails when the work fails.

	func Tests(ctx context.Context, _ eg.Op) error {
		return shell.Run(ctx, shell.New("make test"))
	}

	func main() {
		ctx := context.Background()
		notify := egnotif.New(egnotif.Log(os.Stderr, egnotif.LogOptions()...))

		err := notify.Fatalln(ctx, eg.Perform(ctx, eggit.AutoClone, Tests))
		if err != nil {
			log.Fatalln(err)
		}
	}

# Custom Messages

The failure and completed messages are optional, builtin defaults are used when they are not
provided. Messages are markdown text/templates executed against Context.

	notify := egnotif.New(
		egnotif.Log(os.Stderr, egnotif.LogOptions()...),
		egnotif.Options().
			Failure("**{{.Workload}} failed** after {{.Elapsed}}: {{.Cause}}").
			Completed("**{{.Workload}} completed** in {{.Elapsed}}").
			Timeout(10*time.Second)...,
	)

The fields available to templates are:
  - VCSURI: canonical uri of the repository.
  - Commit: the commit hash.
  - RunID: the id of the run.
  - AccountID: the id of the account.
  - Workload: the entry path of the workload, "default" for the default workload.
  - Initiated: when the run started, see EG_COMPUTE_INITIATED_TIMESTAMP.
  - Elapsed: the time elapsed since the run started.
  - Cause: the error when the workload failed, nil otherwise.
  - Failed: reports whether the workload failed.

# Timeout

Delivery is bounded by a timeout, 30 seconds by default, configured with Options().Timeout(d).
A deliverer that does not complete in time returns a context.DeadlineExceeded error which is
joined with the workload's error.

# Email

The SMTP deliverer sends the rendered message to the recipients using net/smtp.

	notify := egnotif.New(egnotif.SMTP(
		"smtp.example.com:587",
		egnotif.SMTPOptions().
			From("eg@example.com").
			To("team@example.com").
			Auth(smtp.PlainAuth("", "user", os.Getenv("SMTP_PASSWORD"), "smtp.example.com")).
			StartTLS(&tls.Config{})...,
	))

# Custom Deliverers

Implement Send to deliver to any other platform. Context.Message renders the failure or
completed template for you.

	type slack struct{ webhook string }

	func (t slack) Send(ctx context.Context, v egnotif.Context) error {
		msg, err := v.Message()
		if err != nil {
			return err
		}

		// post msg to t.webhook, respecting ctx for cancellation.
		return nil
	}
*/
package egnotif
