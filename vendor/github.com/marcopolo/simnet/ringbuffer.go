package simnet

type ringBuffer[T any] struct {
	head int
	tail int
	buf  []T
}

func newRingBuffer[T any](capacity int) ringBuffer[T] {
	return ringBuffer[T]{
		head: 0,
		tail: 0,
		buf:  make([]T, capacity),
	}
}

func (r *ringBuffer[T]) PushBack(value T) {
	r.buf[r.tail] = value
	r.tail = (r.tail + 1) % len(r.buf)
	if r.tail == r.head {
		// reallocate a larger buffer
		newBuf := make([]T, len(r.buf)*2)
		copy(newBuf, r.buf[r.head:])
		copy(newBuf[len(r.buf)-r.head:], r.buf[:r.head])
		oldLen := len(r.buf)
		r.buf = newBuf
		r.head = 0
		r.tail = oldLen
	}
}

func (r *ringBuffer[T]) PopFront() T {
	value := r.buf[r.head]
	r.head = (r.head + 1) % len(r.buf)
	return value
}

func (r *ringBuffer[T]) Peek() T {
	return r.buf[r.head]
}

func (r *ringBuffer[T]) Empty() bool {
	return r.head == r.tail
}
