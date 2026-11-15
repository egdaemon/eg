package uuidx

import (
	"bytes"
)

type Range struct {
	Begin UUID
	End   UUID
}

// RangeFromStrings panics if the uuids are malformed
func RangeFromStrings(begin, end string) Range {
	return Range{
		Begin: MustParse(begin),
		End:   MustParse(end),
	}
}

// RangeEverything covers the entire uuid range.
// from uuid.Nil to uuid.Max
func RangeEverything() Range {
	return Range{
		Begin: Nil(),
		End:   Max(),
	}
}

func NewSegmenter() *Segmenter {
	return &Segmenter{
		current: Nil(),
		next:    Nil(),
	}
}

type Segmenter struct {
	current UUID
	next    UUID
}

func (t *Segmenter) Next() bool {
	const (
		// we want to generate 1024 segments,
		// calculated increment using ceil(65535 / 1024)
		by = 64
	)

	t.current = t.next
	t.next = Advance16(t.next, by)
	future := Advance16(t.next, by)
	max := Max()
	done := !bytes.Equal(t.next[:], max[:])

	if bytes.Equal(t.next[:], future[:]) {
		t.next = max
	}

	return done
}

func (t *Segmenter) Range() Range {
	return Range{
		Begin: t.current,
		End:   t.next,
	}
}
