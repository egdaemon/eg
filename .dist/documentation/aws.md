---
created: 2026-10-24T05:00:00.000Z
updated: 2026-10-24T05:00:00.000Z
title: "AWS Access"
index: 5
---

## AWS Access

lets your workloads use resources in your own AWS account, like Secrets Manager or S3, without creating or storing access keys. it uses [web identity federation](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers_oidc.html): your account trusts an identity provider that eg controls, and eg issues your workloads short lived identity tokens for it. the aws cli and sdks exchange the token for your role by themselves, there is nothing to configure in the workload.

### how it works

1. you register eg as an OpenID Connect identity provider in your account.
2. you create a role for workloads to assume. its trust policy only allows identity tokens issued by eg for your eg account. what workloads can reach is whatever permissions you give that role.
3. you register the role with eg as a connection, using `eg aws wif create`.

nothing sensitive is shared with eg. the connection only records which role to use.

### 1. find what to trust

managing connections requires the user management permission on your account.

```bash
eg aws oidc
# issuer: https://sso.example.com
# subject: account:00000000-0000-0000-0000-000000000000
```

the `issuer` is the url of the identity provider, the `subject` identifies your account in the identity tokens. both are needed below.

### 2. configure your account

this example uses terraform, the same can be done with the aws cli.

```hcl
data "eg_aws_oidc" "default" {}

resource "aws_iam_openid_connect_provider" "egdaemon" {
  url            = data.eg_aws_oidc.default.issuer_url
  client_id_list = ["sts.amazonaws.com"]
}

locals {
  issuer = trimprefix(data.eg_aws_oidc.default.issuer_url, "https://")
}

# the role workloads assume. only identity tokens eg issued for your account are accepted.
resource "aws_iam_role" "egdaemon" {
  name = "egdaemon-workloads"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRoleWithWebIdentity"
      Principal = { Federated = aws_iam_openid_connect_provider.egdaemon.arn }
      Condition = {
        StringEquals = {
          "${local.issuer}:aud" = "sts.amazonaws.com"
          "${local.issuer}:sub" = data.eg_aws_oidc.default.subject
        }
      }
    }]
  })
}

# what workloads can do, grant only what they need.
resource "aws_iam_role_policy_attachment" "egdaemon" {
  role       = aws_iam_role.egdaemon.name
  policy_arn = "arn:aws:iam::aws:policy/SecretsManagerReadWrite"
}

resource "eg_aws_identity_federation" "default" {
  role_arn    = aws_iam_role.egdaemon.arn
  region      = "us-east-2"
  description = "my account"
}
```

always pin the `sub` condition. without it any eg account could assume the role.

### 2b. register the connection with the cli

if you do not use terraform.

```bash
eg aws wif create \
  --role-arn "arn:aws:iam::123456789012:role/egdaemon-workloads" \
  --region us-east-2 \
  --description "my account"
```

- `--role-arn` is the role workloads assume.
- `--region` is optional, it is the default region of the aws cli and sdks in the workload.
- `--audience` is optional and defaults to `sts.amazonaws.com`, it must match the client id of the identity provider.
- the label for the connection is an optional argument, it defaults to the id of your account. the workloads of your account use the connection labeled with your account id.

```bash
# list connections, optionally filtering on text found in the label, role, or description.
eg aws wif ls

# change a connection, only the flags provided are changed.
eg aws wif edit --role-arn "arn:aws:iam::123456789012:role/other"
eg aws wif edit --clear-region

# remove a connection.
eg aws wif rm
```

### using it

workloads that read `awssm://` secrets are given the credentials automatically. to enable them for other workloads pass `--aws-auto` to `eg compute upload`. inside the workload the credentials are used by the aws cli and sdks without any setup:

```bash
aws sts get-caller-identity
```

the identity token is refreshed in the background and the sdks pick up the new token on their own, so long running workloads keep their access.

when running locally with `eg compute local --aws-auto` or `eg compute serve --aws-auto` the credentials your machine resolves through the aws default credential chain are provided to the workload instead. they expire when their source does.

the credentials are set in the environment of the workload (`AWS_ROLE_ARN`, `AWS_WEB_IDENTITY_TOKEN_FILE`, ...) unless you set `AWS_ROLE_ARN`, `AWS_ACCESS_KEY_ID` or `AWS_PROFILE` yourself, in which case yours are used. tools that do not inherit the environment of the workload need those variables passed on.

### troubleshooting

- `unable to authorize`: the ssh key `eg` uses is not associated with your account, or the account does not have the user management permission. use `--account-id` if the key is associated with more than one profile.
- `Not authorized to perform sts:AssumeRoleWithWebIdentity`: the trust policy does not match the token. check the `aud` and `sub` conditions against the output of `eg aws oidc` and that the identity provider url is exactly the issuer.
- `Incorrect token audience`: the registered audience is not in the client id list of the identity provider.
- `AccessDenied` when workloads call aws apis: the connection works but the role lacks a permission for what the workload is doing. grant it to the role.
