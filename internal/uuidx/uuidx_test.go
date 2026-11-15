package uuidx_test

import (
	"testing"
	"time"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestUUIDX(t *testing.T) {
	t.Run("Advance16", func(t *testing.T) {
		runAdvance16Test := func(t *testing.T, u UUID, expected UUID, by uint16) {
			actual := Advance16(u, by)
			require.Equal(t, expected, actual, "Advance16(%v, %d) mismatch", u, by)
		}

		t.Run("advance by 1", func(t *testing.T) {
			runAdvance16Test(
				t,
				Nil(),
				MustParse("00010000-0000-0000-0000-000000000000"),
				uint16(1),
			)
		})
	})

	t.Run("WithTimestampPrefix", func(t *testing.T) {
		runWithTimestampPrefixTest := func(t *testing.T, u UUID, ts time.Time, expected UUID) {
			actual := WithTimestampPrefix(u, ts)
			require.Equal(t, expected.String(), actual.String(), "WithTimestampPrefix(%v, %v) mismatch", u, ts)
		}

		t.Run("example - 1 millisecond after epoch", func(t *testing.T) {
			runWithTimestampPrefixTest(
				t,
				Nil(),
				time.Unix(0, 1000),
				FromStringOrNil("00000000-0000-0001-0000-000000000000"),
			)
		})
	})
}

func TestFirstNonZero(t *testing.T) {
	t.Run("should return the first non-nil UUID when it's at the beginning", func(t *testing.T) {
		input := []UUID{
			MustParse("00000000-0000-0000-0000-000000000001"),
			MustParse("00000000-0000-0000-0000-000000000002"),
			MustParse("00000000-0000-0000-0000-000000000003"),
		}
		expected := MustParse("00000000-0000-0000-0000-000000000001")
		actual := FirstNonNil(input...)
		require.Equal(t, expected, actual, "Expected FirstNonZero to return %v, got %v", expected, actual)
	})

	t.Run("should return the first non-nil UUID when it's in the middle", func(t *testing.T) {
		input := []UUID{
			Nil(),
			Nil(),
			MustParse("00000000-0000-0000-0000-000000000002"),
			MustParse("00000000-0000-0000-0000-000000000003"),
		}
		expected := MustParse("00000000-0000-0000-0000-000000000002")
		actual := FirstNonNil(input...)
		require.Equal(t, expected, actual, "Expected FirstNonZero to return %v, got %v", expected, actual)
	})

	t.Run("should return the first non-nil UUID when it's at the end", func(t *testing.T) {
		input := []UUID{
			Nil(),
			Nil(),
			Nil(),
			MustParse("00000000-0000-0000-0000-000000000003"),
		}
		expected := MustParse("00000000-0000-0000-0000-000000000003")
		actual := FirstNonNil(input...)
		require.Equal(t, expected, actual, "Expected FirstNonZero to return %v, got %v", expected, actual)
	})

	t.Run("should return uuid.Nil if all UUIDs are nil", func(t *testing.T) {
		input := []UUID{Nil(), Nil(), Nil()}
		expected := Nil()
		actual := FirstNonNil(input...)
		require.Equal(t, expected, actual, "Expected FirstNonZero to return uuid.Nil, got %v", actual)
	})

	t.Run("should return uuid.Nil for an empty slice", func(t *testing.T) {
		input := []UUID{}
		expected := Nil()
		actual := FirstNonNil(input...)
		require.Equal(t, expected, actual, "Expected FirstNonZero to return uuid.Nil for empty slice, got %v", actual)
	})

	t.Run("should handle a single non-nil UUID correctly", func(t *testing.T) {
		input := []UUID{
			MustParse("00000000-0000-0000-0000-000000000001"),
		}
		expected := MustParse("00000000-0000-0000-0000-000000000001")
		actual := FirstNonNil(input...)
		require.Equal(t, expected, actual, "Expected FirstNonZero to return %v for single non-nil, got %v", expected, actual)
	})

	t.Run("should handle a single nil UUID correctly", func(t *testing.T) {
		input := []UUID{Nil()}
		expected := Nil()
		actual := FirstNonNil(input...)
		require.Equal(t, expected, actual, "Expected FirstNonZero to return uuid.Nil for single nil, got %v", actual)
	})
}

func TestHighN(t *testing.T) {
	runHighNTest := func(t *testing.T, id UUID, n int, expected []byte) {
		actual := HighN(id, n)
		require.Equal(t, expected, actual, "HighN(%v, %d) mismatch", id, n)
	}

	t.Run("n equals 0", func(t *testing.T) {
		runHighNTest(t, MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00"), 0, []byte(nil))
	})

	t.Run("n less than 0", func(t *testing.T) {
		runHighNTest(t, MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00"), -5, []byte(nil))
	})

	t.Run("n equals 1", func(t *testing.T) {
		runHighNTest(t, MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00"), 1, []byte{0x11})
	})

	t.Run("n equals 8 (half UUID)", func(t *testing.T) {
		runHighNTest(t, MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00"), 8, []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88})
	})

	t.Run("n equals 16 (full UUID length)", func(t *testing.T) {
		runHighNTest(t, MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00"), 16, []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00})
	})

	t.Run("n greater than 16", func(t *testing.T) {
		runHighNTest(t, MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00"), 20, []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00})
	})

	t.Run("with uuid.Nil, n equals 0", func(t *testing.T) {
		runHighNTest(t, Nil(), 0, []byte(nil))
	})

	t.Run("with uuid.Nil, n equals 8", func(t *testing.T) {
		runHighNTest(t, Nil(), 8, []byte{0, 0, 0, 0, 0, 0, 0, 0})
	})

	t.Run("with uuid.Nil, n equals 16", func(t *testing.T) {
		runHighNTest(t, Nil(), 16, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	})
}

func TestLowN(t *testing.T) {
	runLowNTest := func(t *testing.T, id UUID, n int, expected []byte) {
		actual := LowN(id, n)
		require.Equal(t, expected, actual, "LowN(%v, %d) mismatch", id, n)
	}

	t.Run("n equals 0", func(t *testing.T) {
		runLowNTest(t, MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00"), 0, []byte(nil))
	})

	t.Run("n less than 0", func(t *testing.T) {
		runLowNTest(t, MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00"), -5, []byte(nil))
	})

	t.Run("n equals 1", func(t *testing.T) {
		runLowNTest(t, MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00"), 1, []byte{0x11})
	})

	t.Run("n equals 8 (half UUID)", func(t *testing.T) {
		runLowNTest(t, MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00"), 8, []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88})
	})

	t.Run("n equals 16 (full UUID length)", func(t *testing.T) {
		runLowNTest(t, MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00"), 16, []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00})
	})

	t.Run("n greater than 16", func(t *testing.T) {
		runLowNTest(t, MustParse("11223344-5566-7788-99AA-BBCCDDEEFF00"), 20, []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00})
	})

	t.Run("with uuid.Nil, n equals 0", func(t *testing.T) {
		runLowNTest(t, Nil(), 0, []byte(nil))
	})

	t.Run("with uuid.Nil, n equals 8", func(t *testing.T) {
		runLowNTest(t, Nil(), 8, []byte{0, 0, 0, 0, 0, 0, 0, 0})
	})

	t.Run("with uuid.Nil, n equals 16", func(t *testing.T) {
		runLowNTest(t, Nil(), 16, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	})
}

func TestIsMinMax(t *testing.T) {
	t.Run("uuid.Nil is min/max", func(t *testing.T) {
		require.True(t, IsMinMax(Nil()))
	})

	t.Run("uuid.Max is min/max", func(t *testing.T) {
		require.True(t, IsMinMax(Max()))
	})

	t.Run("a random uuid is not min/max", func(t *testing.T) {
		require.False(t, IsMinMax(NewV4()))
	})
}
