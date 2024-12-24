package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egmetrics"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

type MetricCPU struct {
	Load float32
}

func automcpu() MetricCPU {
	return MetricCPU{
		Load: rand.Float32(),
	}
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

func TCPTransfer(ctx context.Context, op eg.Op) error {
	if err := checkTransfer(ctx, listentcp("tcp", ":0")); err != nil {
		return err
	}

	log.Println("----------------------------- WOOOOT -----------------------------")
	return nil
}

func Debug(ctx context.Context, op eg.Op) error {
	log.Println("debug initiated")
	defer log.Println("debug completed")

	return shell.Run(
		ctx,
		shell.New("env"),
		shell.New("pwd"),
		shell.Newf("truncate --size 0 %s", egenv.RuntimeDirectory("environ.env")).Lenient(true),
		shell.Newf("tree -L 2 %s", egenv.RootDirectory()),
		shell.Newf("apt-get install stress"),
		shell.Newf("stress -t 5 -c %d", 24),
		// shell.Newf("stress -t 5 -m %d", 24), // requires 6GB of ram
	)
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		Debug,
		// TCPTransfer,
	)

	if err != nil {
		log.Fatalln(err)
	}

	if err := egmetrics.Record(ctx, "cpu", automcpu()); err != nil {
		log.Fatalln(err)
	}
}
