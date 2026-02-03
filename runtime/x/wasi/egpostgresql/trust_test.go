package egpostgresql_test

import (
	"net/netip"
	"testing"

	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/x/wasi/egpostgresql"
	"github.com/stretchr/testify/require"
)

type mockOp struct{}

func (mockOp) ID() string { return "test" }

func TestTrustNoPrefixes(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	// Trust with no prefixes should return nil immediately
	opfn := egpostgresql.Trust()
	err := opfn(ctx, mockOp{})
	require.NoError(t, err)
}

func TestTrustOnlyUnroutablePrefix(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	// Trust with only unroutable prefix should return nil (filtered out)
	unroutable := egunsafe.UnroutablePrefix()
	opfn := egpostgresql.Trust(unroutable)
	err := opfn(ctx, mockOp{})
	require.NoError(t, err)
}

func TestTrustMultipleUnroutablePrefixes(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	// Trust with multiple unroutable prefixes should return nil
	unroutable := egunsafe.UnroutablePrefix()
	opfn := egpostgresql.Trust(unroutable, unroutable, unroutable)
	err := opfn(ctx, mockOp{})
	require.NoError(t, err)
}

func TestUnroutablePrefixValue(t *testing.T) {
	// Verify UnroutablePrefix returns the expected sentinel value
	unroutable := egunsafe.UnroutablePrefix()
	expected := netip.PrefixFrom(netip.IPv6Unspecified(), 128)
	require.Equal(t, expected, unroutable)
}
