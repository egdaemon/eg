package numericx

type Integer interface {
	int | int8 | int16 | int32 | int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

type Float interface {
	float32 | float64
}
type Numeric interface {
	Integer | Float | ~uintptr
}

func Max[T Numeric](first T, values ...T) T {
	cmp := first
	for _, v := range values {
		if cmp > v {
			continue
		}
		cmp = v
	}

	return cmp
}

func Min[T Numeric](first T, values ...T) T {
	cmp := first
	for _, v := range values {
		if cmp < v {
			continue
		}
		cmp = v
	}

	return cmp
}

func MaxInt[T int | int8 | int16 | int32 | int64](first T, values ...T) T {
	cmp := first
	for _, v := range values {
		if cmp > v {
			continue
		}
		cmp = v
	}

	return cmp
}

func MinInt[T int | int8 | int16 | int32 | int64](first T, values ...T) T {
	cmp := first
	for _, v := range values {
		if cmp < v {
			continue
		}
		cmp = v
	}

	return cmp
}

func MaxUint[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr](first T, values ...T) T {
	cmp := first
	for _, v := range values {
		if cmp > v {
			continue
		}
		cmp = v
	}

	return cmp
}

func MinUint[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr](first T, values ...T) T {
	cmp := first
	for _, v := range values {
		if cmp < v {
			continue
		}
		cmp = v
	}

	return cmp
}
