package egbug_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
	"github.com/stretchr/testify/assert"
)

func TestExpectedFailure(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()
	opok := func(ctx context.Context, o eg.Op) error {
		return nil
	}
	opEOF := func(ctx context.Context, o eg.Op) error {
		return io.EOF
	}

	assert.NoError(t, egbug.ExpectedFailure(opEOF, io.EOF)(ctx, egtest.Op()), "should consider the provided errors as successful")
	assert.ErrorIs(t, egbug.ExpectedFailure(opok, io.EOF)(ctx, egtest.Op()), egbug.ErrUnexpectedSuccessfulOperation, "successful operations are unexpected and should return the unsuccessful operation error")
	assert.ErrorIs(t, egbug.ExpectedFailure(opEOF, errors.ErrUnsupported)(ctx, egtest.Op()), io.EOF, "errors we dont intentionally ignore should be returned")
}
