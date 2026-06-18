package buildx

import (
	"go/build"
)

type Option func(*build.Context)

func Tags(tags ...string) Option {
	return func(ctx *build.Context) {
		ctx.BuildTags = tags
	}
}

func Dir(dir string) Option {
	return func(ctx *build.Context) {
		ctx.Dir = dir
	}
}

func Clone(bctx build.Context, options ...Option) build.Context {
	for _, opt := range options {
		opt(&bctx)
	}
	return bctx
}
