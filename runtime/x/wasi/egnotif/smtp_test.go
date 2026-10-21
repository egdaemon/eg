package egnotif_test

import (
	"bufio"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/runtime/x/wasi/egnotif"
	"github.com/stretchr/testify/require"
)

// minimal in memory smtp server, returns the DATA payload once the session completes.
func fakesmtp(t *testing.T) (addr string, data <-chan string) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })

	out := make(chan string, 1)
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(10 * time.Second))

		r := bufio.NewReader(conn)
		write := func(s string) { conn.Write([]byte(s + "\r\n")) }
		write("220 localhost ready")
		var body strings.Builder
		indata := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if indata {
				if line == "." {
					indata = false
					write("250 ok")
					continue
				}
				body.WriteString(line + "\n")
				continue
			}
			switch cmd := strings.ToUpper(line); {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				write("250 localhost")
			case strings.HasPrefix(cmd, "MAIL"), strings.HasPrefix(cmd, "RCPT"):
				write("250 ok")
			case cmd == "DATA":
				indata = true
				write("354 go ahead")
			case cmd == "QUIT":
				write("221 bye")
				out <- body.String()
				return
			default:
				write("250 ok")
			}
		}
	}()

	return l.Addr().String(), out
}

func TestSMTP(t *testing.T) {
	t.Run("delivers the rendered message", func(t *testing.T) {
		addr, data := fakesmtp(t)
		d := egnotif.SMTP(addr, egnotif.SMTPOptions().From("eg@example.com").To("a@example.com", "b@example.com")...)
		err := egnotif.New(d, egnotif.Options().Failure("it broke: {{.Cause}}")...).Fatalln(t.Context(), errors.New("boom"))
		require.ErrorContains(t, err, "boom")
		require.NotContains(t, err.Error(), "smtp")

		select {
		case got := <-data:
			require.Contains(t, got, "From: eg@example.com")
			require.Contains(t, got, "To: a@example.com, b@example.com")
			require.Contains(t, got, "Subject: eg workload failed:")
			require.Contains(t, got, "it broke: boom")
		case <-time.After(5 * time.Second):
			t.Fatal("no message received")
		}
	})

	t.Run("requires from and recipients", func(t *testing.T) {
		require.Error(t, egnotif.New(egnotif.SMTP("127.0.0.1:1")).Fatalln(t.Context(), nil))
	})

	t.Run("subject cannot inject headers", func(t *testing.T) {
		addr, data := fakesmtp(t)
		d := egnotif.SMTP(addr, egnotif.SMTPOptions().From("eg@example.com").To("a@example.com").Subject("hi\r\nBcc: evil@example.com")...)
		require.NoError(t, egnotif.New(d).Fatalln(t.Context(), nil))
		got := <-data
		require.NotContains(t, got, "\nBcc:")
	})
}
