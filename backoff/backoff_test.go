package backoff

import (
	"fmt"
	"math"
	"time"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/onsi/gomega"
)

func testBackoff(attempts int64, s Strategy, expected ...time.Duration) {
	for i := int64(0); i < attempts; i++ {
		Expect(s.Backoff(i)).To(Equal(expected[i]), fmt.Sprintf("attempt %d", i))
	}
}

func expectedDurationTest(attempt int64, s Strategy, expected time.Duration) {
	Expect(s.Backoff(attempt)).To(Equal(expected))
}

var _ = Describe("Backoff", func() {
	DescribeTable("Explicit",
		testBackoff,
		Entry("more attempts than delays", 5, Explicit(1*time.Second, 2*time.Second, 3*time.Second), 1*time.Second, 2*time.Second, 3*time.Second, 1*time.Second, 2*time.Second),
	)
	DescribeTable("Exponential",
		testBackoff,
		Entry("should double each time", 5, Exponential(1*time.Second), 1*time.Second, 2*time.Second, 4*time.Second, 8*time.Second, 16*time.Second),
		Entry(
			"should gracefully handle overflows",
			101,
			Exponential(1*time.Second),
			time.Second<<uint(0),
			time.Second<<uint(1),
			time.Second<<uint(2),
			time.Second<<uint(3),
			time.Second<<uint(4),
			time.Second<<uint(5),
			time.Second<<uint(6),
			time.Second<<uint(7),
			time.Second<<uint(8),
			time.Second<<uint(9),
			time.Second<<uint(10),
			time.Second<<uint(11),
			time.Second<<uint(12),
			time.Second<<uint(13),
			time.Second<<uint(14),
			time.Second<<uint(15),
			time.Second<<uint(16),
			time.Second<<uint(17),
			time.Second<<uint(18),
			time.Second<<uint(19),
			time.Second<<uint(20),
			time.Second<<uint(21),
			time.Second<<uint(22),
			time.Second<<uint(23),
			time.Second<<uint(24),
			time.Second<<uint(25),
			time.Second<<uint(26),
			time.Second<<uint(27),
			time.Second<<uint(28),
			time.Second<<uint(29),
			time.Second<<uint(30),
			time.Second<<uint(31),
			time.Second<<uint(32),
			time.Second<<uint(33),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 40
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 50
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 60
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 70
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 80
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 90
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 100
		),
	)
	DescribeTable("Constant",
		testBackoff,
		Entry("should remain constant", 5, Constant(1*time.Second), 1*time.Second, 1*time.Second, 1*time.Second, 1*time.Second, 1*time.Second),
	)

	DescribeTable("Exponential Backoff",
		expectedDurationTest,
		Entry("attempt 0", 0, Exponential(1*time.Second), time.Duration(1*time.Second)),
		Entry("attempt 1", 1, Exponential(1*time.Second), time.Duration(2*time.Second)),
		Entry("attempt 2", 2, Exponential(1*time.Second), time.Duration(4*time.Second)),
		Entry("attempt 3", 3, Exponential(1*time.Second), time.Duration(8*time.Second)),
		Entry("attempt 36", 36, Exponential(1*time.Second), time.Duration(math.MaxInt64)),
		Entry("attempt 37", 37, Exponential(1*time.Second), time.Duration(math.MaxInt64)),
		Entry("attempt 54 - overflow", 54, Exponential(1*time.Second), time.Duration(math.MaxInt64)),
		Entry("with scaling - attempt 0", 0, Exponential(500*time.Millisecond), time.Duration(500*time.Millisecond)),
		Entry("with scaling - attempt 1", 1, Exponential(500*time.Millisecond), time.Duration(1*time.Second)),
		Entry("with scaling - attempt 2", 2, Exponential(500*time.Millisecond), time.Duration(2*time.Second)),
		Entry("with scaling - attempt 3", 3, Exponential(500*time.Millisecond), time.Duration(4*time.Second)),
		Entry("max attempt value", math.MaxInt64, Exponential(1*time.Second), time.Duration(math.MaxInt64)),
	)

	DescribeTable(
		"Jitter",
		expectedDurationTest,
		Entry(
			"example 1 - with jitter",
			57,
			New(
				Exponential(1*time.Second),
				Jitter(0.25),
			),
			time.Duration(math.MaxInt64),
		),
	)
})
