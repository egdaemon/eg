package uuidx

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"time"
	"uuid"
)

// UUID is the uuid type for the codebase. It shares its underlying type with the
// stdlib uuid.UUID (conversions are free) but allows attaching methods.
type UUID uuid.UUID

// Size is the number of bytes in a uuid.
const Size = len(UUID{})

// V7 is the version number reported by UUID.Version for v7 uuids.
const V7 byte = 7

// Nil returns the all-zero uuid.
func Nil() UUID {
	return UUID(uuid.Nil())
}

// Max returns the all-ones uuid.
func Max() UUID {
	return UUID(uuid.Max())
}

func NewV4() UUID {
	return UUID(uuid.NewV4())
}

func NewV7() UUID {
	return UUID(uuid.NewV7())
}

// NewV7AtTime returns a v7 uuid whose timestamp is ts instead of the current time.
func NewV7AtTime(ts time.Time) UUID {
	u := NewV7()

	var ms [8]byte
	binary.BigEndian.PutUint64(ms[:], uint64(ts.UnixMilli()))
	copy(u[:6], ms[2:])

	return u
}

func Parse(text string) (UUID, error) {
	u, err := uuid.Parse(text)
	return UUID(u), err
}

// MustParse panics if text is not a valid uuid.
func MustParse(text string) UUID {
	return UUID(uuid.MustParse(text))
}

// FromString returns a UUID parsed from the input string.
// Input is expected in a form accepted by uuid.Parse.
func FromString(text string) (UUID, error) {
	return Parse(text)
}

// FromStringOrNil is FromString but returns Nil() instead of an error.
func FromStringOrNil(input string) UUID {
	u, err := Parse(input)
	if err != nil {
		return Nil()
	}
	return u
}

// FromBytes returns a UUID from the input bytes; the input must be exactly Size bytes.
func FromBytes(input []byte) (UUID, error) {
	if len(input) != Size {
		return Nil(), fmt.Errorf("uuid: invalid length %d, expected %d", len(input), Size)
	}

	var u UUID
	copy(u[:], input)

	return u, nil
}

// FromBytesOrNil is FromBytes but returns Nil() instead of an error.
func FromBytesOrNil(input []byte) UUID {
	// avoids the overhead of generating an error which would be checked and discarded.
	if len(input) != Size {
		return Nil()
	}

	var u UUID
	copy(u[:], input)

	return u
}

func (u UUID) String() string {
	return uuid.UUID(u).String()
}

func (u UUID) Bytes() []byte {
	return u[:]
}

func (u UUID) IsZero() bool {
	return u == UUID{}
}

// Version returns the uuid version (the high nibble of byte 6).
func (u UUID) Version() byte {
	return u[6] >> 4
}

func (u UUID) IsMax() bool {
	return u == Max()
}

func (u UUID) Compare(v UUID) int {
	return uuid.UUID(u).Compare(uuid.UUID(v))
}

func (u UUID) MarshalText() ([]byte, error) {
	return uuid.UUID(u).MarshalText()
}

func (u UUID) AppendText(b []byte) ([]byte, error) {
	return uuid.UUID(u).AppendText(b)
}

func (u *UUID) UnmarshalText(b []byte) error {
	return (*uuid.UUID)(u).UnmarshalText(b)
}

// Value implements driver.Valuer, the uuid is stored as its string form.
func (u UUID) Value() (driver.Value, error) {
	return u.String(), nil
}

// Scan implements sql.Scanner; accepts the text form (string or []byte)
// or the raw 16 bytes.
func (u *UUID) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*u = Nil()
		return nil
	case string:
		return u.UnmarshalText([]byte(v))
	case []byte:
		if len(v) == Size {
			copy(u[:], v)
			return nil
		}
		return u.UnmarshalText(v)
	default:
		return fmt.Errorf("uuid: unable to scan %T", src)
	}
}
