// Package api is a fixture.
package api

// Equal is documented, the documentation is not listed.
func Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool {
	return expected == actual
}

func Equalf(expected, actual interface{}, msg string, args ...interface{}) bool {
	return expected == actual
}

func Zero[T any](v T) bool {
	var z T
	_ = z
	return false
}

func hidden() {}

type Assertions struct{}

func (t Assertions) Equal(expected, actual interface{}) bool { return true }
