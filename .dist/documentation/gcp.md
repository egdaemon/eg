---
created: 2026-10-18T05:00:00.000Z
updated: 2026-10-18T05:00:00.000Z
title: "GCP Access"
index: 5
---

## GCP Access

lets your workloads use resources in your own Google Cloud project, like Secret Manager, without creating or storing service account keys. it uses [Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation): your project trusts an identity that eg controls, and eg issues your workloads short lived credentials for it.

### how it works

1. you create a workload identity pool and provider in your project that only trusts eg's identity, `egd-workloads@egd-workloads.iam.gserviceaccount.com`.
2. you create a service account in your project for workloads to act as, and grant the pool permission to impersonate it. what workloads can reach is whatever roles you give that service account.
3. you register the provider with eg as a connection, using `eg gcp wif create`.

nothing sensitive is shared with eg. the connection only records which provider and service account to use.

### 1. configure your project

the project needs the `iam.googleapis.com`, `iamcredentials.googleapis.com` and `sts.googleapis.com` APIs enabled. this example uses terraform, the same can be done with `gcloud`.

```hcl
resource "google_iam_workload_identity_pool" "egdaemon" {
  workload_identity_pool_id = "egdaemon"
  display_name              = "egdaemon"
}

resource "google_iam_workload_identity_pool_provider" "egdaemon" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.egdaemon.workload_identity_pool_id
  workload_identity_pool_provider_id = "egd-workloads"
  display_name                       = "egd-workloads"

  attribute_mapping = {
    "google.subject"  = "assertion.sub"
    "attribute.email" = "assertion.email"
  }

  # only the eg identity may exchange tokens.
  attribute_condition = "assertion.email == 'egd-workloads@egd-workloads.iam.gserviceaccount.com'"

  oidc {
    issuer_uri = "https://accounts.google.com"
  }
}

# the identity workloads act as.
resource "google_service_account" "egdaemon" {
  account_id   = "egd-compute"
  display_name = "egdaemon workloads"
}

# allow the pool to impersonate it.
resource "google_service_account_iam_member" "egdaemon" {
  service_account_id = google_service_account.egdaemon.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.egdaemon.name}/attribute.email/egd-workloads@egd-workloads.iam.gserviceaccount.com"
}

# what workloads can do, grant only what they need.
resource "google_project_iam_member" "egdaemon" {
  project = google_service_account.egdaemon.project
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.egdaemon.email}"
}

output "audience" {
  value = "//iam.googleapis.com/${google_iam_workload_identity_pool_provider.egdaemon.name}"
}

output "impersonate" {
  value = google_service_account.egdaemon.email
}
```

the provider's allowed audience is not set, it defaults to the provider's own resource name, which is what eg uses as the audience of the identity token. the `audience` output is the value you register next.

### 2. register the connection

managing connections requires the user management permission on your account.

```bash
eg gcp wif create \
  --audience "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/egdaemon/providers/egd-workloads" \
  --impersonate "egd-compute@my-project.iam.gserviceaccount.com" \
  --description "my project"
```

- `--audience` is the provider's full resource name, the `audience` output above.
- `--impersonate` is optional. when set, workloads act as that service account, which is what the permission grants above apply to. without it they act as the federated pool principal directly.
- the label for the connection is an optional argument, it defaults to the id of your account. use labels to connect more than one project.

```bash
# list connections, optionally filtering on text found in the label, audience, or description.
eg gcp wif ls
eg gcp wif ls --query my-project

# change a connection, only the flags provided are changed.
eg gcp wif edit --description "new description"
eg gcp wif edit --clear-impersonate

# remove a connection.
eg gcp wif rm
```

### troubleshooting

- `unable to authorize`: the ssh key `eg` uses is not associated with your account, or the account does not have the user management permission. use `--account-id` if the key is associated with more than one profile.
- `The caller does not have permission` when workloads call Google APIs: the connection works but the service account lacks a role for what the workload is doing. grant it in your project.
- `The audience in ID Token ... does not match`: the registered audience does not match the provider's resource name, including the project number. compare with `gcloud iam workload-identity-pools providers describe egd-workloads --workload-identity-pool=egdaemon --location=global --format='value(name)'`.
