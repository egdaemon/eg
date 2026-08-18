// Package konggdx is a kong-based CLI for gdx's debug HTTP surface. Every
// command takes a plain context.Context, bound by whichever binary embeds
// Commands (kong.Bind(ctx)) — this package has no dependency on any
// particular application's CLI scaffolding.
package konggdx

// Commands is the top-level kong command group for gdx.
type Commands struct {
	Prof  Prof  `cmd:"" help:"pull profiles from a running gdx debug socket"`
	Trace Trace `cmd:"" help:"capture a runtime/trace execution trace from a running gdx debug socket"`
}
