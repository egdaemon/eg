package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
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

func checkTransfer(ctx context.Context, li net.Listener) error {
	var (
		buf []byte = make([]byte, 128)
	)

	conn, err := net.Dial(li.Addr().Network(), li.Addr().String())
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err = conn.Write([]byte("hello world")); err != nil {
		return err
	}

	if n, err := conn.Read(buf); err != nil {
		return err
	} else if v := string(buf[:n]); v != "hello world" {
		return fmt.Errorf("recieved %s expected %s", v, "hello world")
	} else {
		log.Println("transferred", string(buf[:n]))
	}

	return nil
}

func TCPTest(ctx context.Context, op eg.Op) error {
	if err := checkTransfer(ctx, listentcp("tcp", ":0")); err != nil {
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
		if _, err := io.Copy(w, bytes.NewBuffer(buf.Bytes())); err != nil {
			log.Println("copy failed", err)
			return
		}
	})

	if l, err = net.Listen("tcp", ":0"); err != nil {
		return err
	}
	defer l.Close()

	go func() {
		if err = http.Serve(l, m); err != nil {
			log.Println(err)
		}
	}()

	rsp, err := http.Get(fmt.Sprintf("http://%s", l.Addr().String()))
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
func main() {
	log.SetFlags(log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		DNSDebug,
		DNSTCPResolveTest,
		TCPTest,
		HTTPTest,
		HTTPServerTest,
	)

	if err != nil {
		log.Fatalln(err)
	}
}
