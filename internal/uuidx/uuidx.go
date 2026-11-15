package uuidx

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// T constrains to any type with the underlying layout of a uuid,
// e.g. UUID and the stdlib uuid.UUID.
type T interface {
	~[16]byte
}

// WithSuffix makes an almost all balls UUID
func WithSuffix(end int) string {
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", end)
}

// prepend timestamp to a uuid; this is useful for ensuring timebased ordering.
func WithTimestampPrefix[V T](uid V, ts time.Time) V {
	binary.BigEndian.PutUint64(uid[:8], uint64(ts.UnixMicro()))
	return uid
}

func Advance16[V T](u V, by uint16) V {
	dup := u

	if v := binary.BigEndian.Uint16(dup[:]); math.MaxUint16-v < by {
		binary.BigEndian.PutUint16(dup[:], math.MaxUint16)
	} else {
		binary.BigEndian.PutUint16(dup[:], v+by)
	}

	return dup
}

// IsMinMax reports whether id is one of the two sentinel values used
// throughout this codebase to mean "no real id": uuid.Nil (the all-zero
// minimum) or uuid.Max (the all-ones maximum).
func IsMinMax[V T](uid V) bool {
	return UUID(uid).IsZero() || UUID(uid).IsMax()
}

// returns the first non-zero uuid or zero if all are zero.
func FirstNonNil[V T](uids ...V) V {
	for _, uid := range uids {
		if !UUID(uid).IsZero() {
			return uid
		}
	}

	return V{}
}

func HighN[V T](id V, n int) []byte {
	ubytes := id[:]

	if n <= 0 {
		return []byte(nil)
	}

	if n >= len(ubytes) {
		return ubytes
	}

	return ubytes[:n]
}

func LowN[V T](id V, n int) []byte {
	ubytes := id[:]

	if n <= 0 {
		return []byte(nil)
	}
	if n >= len(ubytes) {
		return append([]byte(nil), ubytes...)
	}

	return ubytes[:n]
}
