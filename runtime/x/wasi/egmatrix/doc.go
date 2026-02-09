/*
Package egmatrix generates every permutation of provided options by assigning
values to their respective fields on a struct type T.

The package provides a fluent builder API for defining mutation groups, where each
group represents different possible values for a field. The Perm() method generates
the cartesian product of all mutation groups, yielding every possible combination.

# Basic Usage

Define a struct and use the builder to specify options for each field:

	type Config struct {
		Debug   bool
		Timeout string
	}

	m := egmatrix.New[Config]().
		Boolean(func(c *Config, v bool) { c.Debug = v }).
		String(func(c *Config, v string) { c.Timeout = v }, "30s", "60s")

	for config := range m.Perm() {
		// Yields 4 permutations: all combinations of Debug (true/false) and Timeout (30s/60s)
		fmt.Printf("Debug=%t Timeout=%s\n", config.Debug, config.Timeout)
	}

# Type-Specific Methods

The builder provides convenience methods for common types:
  - Boolean(fn): Generates permutations for true and false
  - String(fn, options...): Generates permutations for each string option
  - Int64(fn, options...): Generates permutations for each int64 option
  - Float64(fn, options...): Generates permutations for each float64 option

# Custom Types

For custom types, use the Assign function directly:

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
	egmatrix.Assign(m, func(c *Config, v LogLevel) { c.Level = v }, Info, Warning, Error)

# Cartesian Product

The Perm() method generates the cartesian product of all mutation groups:

	m := egmatrix.New[BuildConfig]().
		Boolean(func(c *BuildConfig, v bool) { c.Optimize = v }).      // 2 options
		String(func(c *BuildConfig, v string) { c.Target = v }, "linux", "darwin"). // 2 options
		Int64(func(c *BuildConfig, v int64) { c.Workers = v }, 2, 4)   // 2 options

	// Yields 2 × 2 × 2 = 8 permutations
	for config := range m.Perm() {
		// Process each unique combination
	}
*/
package egmatrix
