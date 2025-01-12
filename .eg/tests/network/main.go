package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/wasinet/wasinet"
)

func digest(b []byte) string {
	d := md5.Sum(b)
	return hex.EncodeToString(d[:])
}

func DNSDebug(ctx context.Context, _ eg.Op) (err error) {
	return shell.Run(
		ctx,
		shell.Newf("systemctl status systemd-resolved.service"),
		shell.New("ss -t -l -n -p"),
	)
}

func listentcp(network, address string) net.Listener {
	li, err := net.Listen(network, address)
	if err != nil {
		panic(err)
	}

	go func() {
		for conn, err := li.Accept(); err == nil; conn, err = li.Accept() {
			server, client := net.Pipe()
			go func(c net.Conn) {
				if _, err := io.Copy(c, server); err != nil {
					log.Println("server copy failed", err)
				}
			}(conn)
			go func(c net.Conn) {
				defer c.Close()
				if _, err := io.Copy(client, c); err != nil {
					log.Println("client copy failed", err)
				}
			}(conn)
		}
	}()

	return li
}

func checkTransfer(ctx context.Context, li net.Listener, amount int64) error {
	var (
		serr       error
		amountsent int64
	)

	conn, err := net.Dial(li.Addr().Network(), li.Addr().String())
	if err != nil {
		return err
	}

	digestsent := md5.New()
	digestrecv := md5.New()

	go func() {
		amountsent, serr = io.CopyN(conn, io.TeeReader(rand.Reader, digestsent), amount)
	}()

	n, err := io.Copy(digestrecv, io.LimitReader(conn, amount))
	if err != nil {
		return err
	}

	if serr != nil {
		return serr
	}

	if amount != n {
		return fmt.Errorf("didnt receive all data", amount, "!=", n)
	}

	if amount != amountsent {
		return fmt.Errorf("didnt receive all data", amount, "!=", amountsent)
	}

	if !bytes.Equal(digestsent.Sum(nil), digestrecv.Sum(nil)) {
		return fmt.Errorf("digests didnt match")
	}

	return nil
}

func TCPTest(ctx context.Context, op eg.Op) error {
	if err := checkTransfer(ctx, listentcp("tcp", "localhost:0"), 16*1024); err != nil {
		return err
	}
	return nil
}

func DNSTCPResolveTest(ctx context.Context, op eg.Op) error {
	ip, err := net.ResolveTCPAddr("tcp", "www.google.com:443")
	if err != nil {
		return err
	}
	log.Println("IP ADDRESS", ip.IP, ip.Port)
	return nil
}

func HTTPTest(ctx context.Context, op eg.Op) error {
	rsp, err := http.Get("https://egdaemon.com")
	if err != nil {
		return err
	}

	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", rsp.StatusCode)
	}

	return nil
}

func HTTPServerTest(ctx context.Context, op eg.Op) (err error) {
	var (
		l   net.Listener
		buf bytes.Buffer
	)

	_, err = io.CopyN(&buf, rand.Reader, 16*1024)
	if err != nil {
		return err
	}

	m := http.NewServeMux()
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("request received")
		defer log.Println("request completed")

		if _, err := io.Copy(w, bytes.NewBuffer(buf.Bytes())); err != nil {
			log.Println("copy failed", err)
			return
		}
	})

	if l, err = wasinet.Listen(context.Background(), "tcp", "localhost:0"); err != nil {
		return err
	}
	defer l.Close()

	go func() {
		if err = http.Serve(l, m); err != nil {
			log.Println(err)
		}
	}()

	log.Println("server addr", l.Addr().String())
	rsp, err := http.Get(fmt.Sprintf("http://%s/", l.Addr().String()))
	if err != nil {
		return err
	}
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d - %s", rsp.StatusCode, rsp.Status)
	}
	received, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	if e, a := digest(buf.Bytes()), digest(received); e != a {
		return fmt.Errorf("data doesn't match expected %s vs %s", a, e)
	}

	log.Println("successfully ran http server")
	return nil
}

func init() {
	wasinet.Hijack()
	http.DefaultTransport = InsecureHTTP()
}

func main() {
	log.SetFlags(log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eg.Parallel(
			DNSDebug,
			DNSTCPResolveTest,
			TCPTest,
			HTTPTest,
			HTTPServerTest,
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}

func InsecureHTTP() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Proxy:           http.ProxyFromEnvironment,
		DialContext: (&wasinet.Dialer{
			Timeout: 2 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2: true,

		MaxIdleConns:          10,
		ResponseHeaderTimeout: 5 * time.Second,
		IdleConnTimeout:       5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
	}
}
