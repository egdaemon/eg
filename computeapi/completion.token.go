package computeapi

import "context"

type completionTokenKey struct{}

// ContextWithCompletionToken attaches the access token the control plane issued when the
// workload was handed to the runner, it is presented when uploading the completion.
func ContextWithCompletionToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, completionTokenKey{}, token)
}

// CompletionTokenFromContext returns the completion token attached to the context, blank when there is none.
func CompletionTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(completionTokenKey{}).(string)
	return token
}
