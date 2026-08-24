// Package langx provides small utility functions to extend the standard golang language.
package langx

// Autoptr converts a value into a pointer
func Autoptr[T any](a T) *T {
	return &a
}

// Autoderef safely converts a pointer to its value, uses the zero value for nil.
func Autoderef[T any](a *T) (zero T) {
	if a == nil {
		return zero
	}

	return *a
}
