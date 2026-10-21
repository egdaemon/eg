package egnotif

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

// SMTPOption configures the smtp deliverer.
type SMTPOption func(*smtpOption)
type smtpOptions []SMTPOption

type smtpOption struct {
	from     string
	to       []string
	subject  string
	auth     smtp.Auth
	starttls *tls.Config
}

// DefaultSMTPSubject is the text/template used for the subject when no override is provided.
const DefaultSMTPSubject = "eg workload {{if .Failed}}failed{{else}}completed{{end}}: {{.VCSURI}} {{.Workload}}"

// SMTPOptions generates the default options for the smtp deliverer.
func SMTPOptions() smtpOptions {
	return smtpOptions(nil)
}

// From sets the sender address.
func (t smtpOptions) From(v string) smtpOptions {
	return append(t, func(o *smtpOption) {
		o.from = v
	})
}

// To adds recipient addresses.
func (t smtpOptions) To(v ...string) smtpOptions {
	return append(t, func(o *smtpOption) {
		o.to = append(o.to, v...)
	})
}

// Subject overrides the subject text/template.
func (t smtpOptions) Subject(tmpl string) smtpOptions {
	return append(t, func(o *smtpOption) {
		o.subject = tmpl
	})
}

// Auth sets the authentication mechanism, see smtp.PlainAuth.
func (t smtpOptions) Auth(a smtp.Auth) smtpOptions {
	return append(t, func(o *smtpOption) {
		o.auth = a
	})
}

// StartTLS upgrades the connection using cfg before authenticating. if cfg.ServerName is empty
// the host of the address is used.
func (t smtpOptions) StartTLS(cfg *tls.Config) smtpOptions {
	return append(t, func(o *smtpOption) {
		o.starttls = cfg
	})
}

// SMTP delivers notifications by email through the server at addr (host:port).
func SMTP(addr string, options ...SMTPOption) deliverer {
	o := smtpOption{subject: DefaultSMTPSubject}
	for _, opt := range options {
		opt(&o)
	}

	return smtpdeliverer{addr: addr, opts: o}
}

type smtpdeliverer struct {
	addr string
	opts smtpOption
}

func (t smtpdeliverer) Send(ctx context.Context, v Context) (err error) {
	var (
		d    net.Dialer
		conn net.Conn
		c    *smtp.Client
	)

	if t.opts.from == "" || len(t.opts.to) == 0 {
		return fmt.Errorf("smtp requires a from address and at least one recipient")
	}

	host, _, err := net.SplitHostPort(t.addr)
	if err != nil {
		return err
	}

	body, err := v.Message()
	if err != nil {
		return err
	}

	subject, err := v.Render(t.opts.subject)
	if err != nil {
		return err
	}

	if conn, err = d.DialContext(ctx, "tcp", t.addr); err != nil {
		return err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		if err = conn.SetDeadline(deadline); err != nil {
			return err
		}
	}

	if c, err = smtp.NewClient(conn, host); err != nil {
		return err
	}
	defer c.Close()

	if t.opts.starttls != nil {
		cfg := t.opts.starttls.Clone()
		if cfg.ServerName == "" {
			cfg.ServerName = host
		}

		if err = c.StartTLS(cfg); err != nil {
			return err
		}
	}

	if t.opts.auth != nil {
		if err = c.Auth(t.opts.auth); err != nil {
			return err
		}
	}

	if err = c.Mail(t.opts.from); err != nil {
		return err
	}

	for _, rcpt := range t.opts.to {
		if err = c.Rcpt(rcpt); err != nil {
			return err
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	// subject is user templated, strip newlines to prevent header injection.
	subject = strings.NewReplacer("\r", " ", "\n", " ").Replace(subject)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n",
		t.opts.from, strings.Join(t.opts.to, ", "), subject, strings.ReplaceAll(body, "\n", "\r\n"))
	if _, err = w.Write([]byte(msg)); err != nil {
		return err
	}

	if err = w.Close(); err != nil {
		return err
	}

	return c.Quit()
}
