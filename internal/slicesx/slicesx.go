package slicesx

// Remove elements from the slice where the predicate returns true.
func Remove[T any](remove func(T) bool, items ...T) []T {
	result := make([]T, 0, len(items))
	for _, i := range items {
		if remove(i) {
			continue
		}

		result = append(result, i)
	}

	return result
}

// Filter the element that do not return true
func Filter[T any](match func(T) bool, items ...T) (results []T) {
	results = make([]T, 0, len(items))

	for _, i := range items {
		if !match(i) {
			continue
		}

		results = append(results, i)
	}

	return results
}

// Find the first matching element
func Find[T any](match func(T) bool, items ...T) (zero T, _ bool) {
	for _, i := range items {
		if match(i) {
			return i, true
		}
	}

	return zero, false
}

// Find the first matching element
func FindOrZero[T any](match func(T) bool, items ...T) (zero T) {
	zero, _ = Find(match, items...)
	return zero
}

// First returns first element in the slice if it exists.
func First[T any](items ...T) (zero T, _ bool) {
	if len(items) == 0 {
		return zero, false
	}

	return items[0], true
}

// First returns first element in the slice if it exists, the zero value otherwise.
func FirstOrZero[T any](items ...T) (zero T) {
	l, _ := First(items...)
	return l
}

// Last returns last element in the slice if it exists.
func Last[T any](items ...T) (zero T, _ bool) {
	if len(items) == 0 {
		return zero, false
	}

	return items[len(items)-1], true
}

// Last returns last element in the slice if it exists.
func LastOrZero[T any](items ...T) (zero T) {
	l, _ := Last(items...)
	return l
}

// Last returns last element in the slice if it exists.
func LastOrDefault[T any](fallback T, items ...T) (zero T) {
	if l, ok := Last(items...); ok {
		return l
	}

	return fallback
}

// Map in place applying the transformation.
func Map[T any](m func(T) T, items ...T) (zero []T) {
	for idx, i := range items {
		items[idx] = m(i)
	}

	return items
}

// MapTransform map the type into another type
func MapTransform[T, X any](m func(T) X, items ...T) (zero []X) {
	results := make([]X, 0, len(items))
	for _, i := range items {
		results = append(results, m(i))
	}

	return results
}

// MapTransformErr map the type into another type
func MapTransformErr[T, X any](m func(T) (X, error), items ...T) (zero []X, err error) {
	results := make([]X, 0, len(items))
	for _, i := range items {
		if v, err := m(i); err != nil {
			return results, err
		} else {
			results = append(results, v)
		}
	}

	return results, nil
}

func Reduce[T any](v *T, options ...func(*T)) *T {
	for _, opt := range options {
		opt(v)
	}

	return v
}
