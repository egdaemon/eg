package egmatrix_test

import (
	"fmt"

	"github.com/egdaemon/eg/runtime/x/wasi/egmatrix"
)

func ExampleNew() {
	type Config struct {
		Debug   bool
		Timeout string
	}

	m := egmatrix.New[Config]().
		Boolean(func(c *Config, v bool) { c.Debug = v }).
		String(func(c *Config, v string) { c.Timeout = v }, "30s", "60s")

	for config := range m.Perm() {
		fmt.Printf("Debug=%t Timeout=%s\n", config.Debug, config.Timeout)
	}

	// Output:
	// Debug=true Timeout=30s
	// Debug=true Timeout=60s
	// Debug=false Timeout=30s
	// Debug=false Timeout=60s
}

func ExampleAssign() {
	type Server struct {
		Port int64
	}

	m := egmatrix.New[Server]()
	egmatrix.Assign(m, func(s *Server, v int64) { s.Port = v }, 8080, 8443, 9000)

	for server := range m.Perm() {
		fmt.Printf("Port=%d\n", server.Port)
	}

	// Output:
	// Port=8080
	// Port=8443
	// Port=9000
}

func ExampleAssign_customType() {
	type LogLevel int

	const (
		Info LogLevel = iota
		Warning
		Error
	)

	type Config struct {
		Level LogLevel
	}

	m := egmatrix.New[Config]()
	egmatrix.Assign(
		m,
		func(c *Config, v LogLevel) { c.Level = v },
		Info, Warning, Error,
	)

	for config := range m.Perm() {
		fmt.Printf("LogLevel=%d\n", config.Level)
	}

	// Output:
	// LogLevel=0
	// LogLevel=1
	// LogLevel=2
}

func ExampleM_Perm() {
	type BuildConfig struct {
		Optimize bool
		Target   string
		Workers  int64
	}

	m := egmatrix.New[BuildConfig]().
		Boolean(func(c *BuildConfig, v bool) { c.Optimize = v }).
		String(func(c *BuildConfig, v string) { c.Target = v }, "linux", "darwin").
		Int64(func(c *BuildConfig, v int64) { c.Workers = v }, 2, 4)

	count := 0
	for config := range m.Perm() {
		count++
		fmt.Printf("%d: Optimize=%t Target=%s Workers=%d\n",
			count, config.Optimize, config.Target, config.Workers)
	}

	// Output:
	// 1: Optimize=true Target=linux Workers=2
	// 2: Optimize=true Target=linux Workers=4
	// 3: Optimize=true Target=darwin Workers=2
	// 4: Optimize=true Target=darwin Workers=4
	// 5: Optimize=false Target=linux Workers=2
	// 6: Optimize=false Target=linux Workers=4
	// 7: Optimize=false Target=darwin Workers=2
	// 8: Optimize=false Target=darwin Workers=4
}

func ExampleM_String() {
	type Request struct {
		Method string
	}

	m := egmatrix.New[Request]().
		String(func(r *Request, v string) { r.Method = v }, "GET", "POST", "PUT")

	for req := range m.Perm() {
		fmt.Printf("Method=%s\n", req.Method)
	}

	// Output:
	// Method=GET
	// Method=POST
	// Method=PUT
}

func ExampleM_Boolean() {
	type Feature struct {
		Enabled bool
	}

	m := egmatrix.New[Feature]().
		Boolean(func(f *Feature, v bool) { f.Enabled = v })

	for feature := range m.Perm() {
		fmt.Printf("Enabled=%t\n", feature.Enabled)
	}

	// Output:
	// Enabled=true
	// Enabled=false
}
