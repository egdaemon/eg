package runners

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/awsx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
)

// configures the agent to use the aws credentials of the current machine, as resolved by the default credential chain
// of the aws sdk (environment, shared configuration, sso, ...). the credentials are provided to the workload in its
// environment, the runner installs them for the user, see awsx.InstallCredentials. they expire with their source, a
// long running workload outlives them.
func AgentOptionAWSCredentials(ctx context.Context, envb *envx.Builder) (AgentOption, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return AgentOptionNoop, errorsx.Wrap(err, "unable to load the aws configuration")
	}

	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return AgentOptionNoop, errorsx.Wrap(err, "unable to resolve the aws credentials")
	}

	raw, err := json.Marshal(awsx.StaticCredentials{
		AccessKeyID:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
	})
	if err != nil {
		return AgentOptionNoop, err
	}

	envb.Var(eg.EnvUnsafeAWSCredentialsB64, base64.URLEncoding.EncodeToString(raw))

	return AgentOptionNoop, nil
}
