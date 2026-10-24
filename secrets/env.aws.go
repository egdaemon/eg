package secrets

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type awsidentitykey struct{}

// awsidentity the role a workload assumes with its identity token.
type awsidentity struct {
	roleARN   string
	tokenFile string
	region    string
	session   string
}

// AWS reads awssm secrets by assuming the role with the web identity token in the file, instead of with the ambient
// credentials. they apply to the reads of this reader only. a blank role leaves the credentials unchanged.
//
//	secrets.Env().AWS(role, tokenfile, region, session).NewReader(ctx, uris...)
func (t *EnvReader) AWS(roleARN, tokenFile, region, session string) *EnvReader {
	if roleARN != "" && tokenFile != "" {
		t.aws = &awsidentity{roleARN: roleARN, tokenFile: tokenFile, region: region, session: session}
	}

	return t
}

func (t *EnvReader) awscontext(ctx context.Context) context.Context {
	if t.aws != nil {
		ctx = context.WithValue(ctx, awsidentitykey{}, *t.aws)
	}

	return ctx
}

// awsconfig loads the aws configuration for the region. the credentials are those of the workload's identity when ctx
// carries one, otherwise the ambient ones.
func awsconfig(ctx context.Context, region string, options ...func(*config.LoadOptions) error) (aws.Config, error) {
	identity, _ := ctx.Value(awsidentitykey{}).(awsidentity)
	if region == "" {
		region = identity.region
	}

	cfg, err := config.LoadDefaultConfig(ctx, append([]func(*config.LoadOptions) error{config.WithRegion(region)}, options...)...)
	if err != nil || identity.roleARN == "" {
		return cfg, err
	}

	provider := stscreds.NewWebIdentityRoleProvider(
		sts.NewFromConfig(cfg),
		identity.roleARN,
		stscreds.IdentityTokenFile(identity.tokenFile),
		func(o *stscreds.WebIdentityRoleOptions) {
			o.RoleSessionName = identity.session
		},
	)
	cfg.Credentials = aws.NewCredentialsCache(provider)

	return cfg, nil
}
