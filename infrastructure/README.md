## Overview & purpose

This folder contains Terraform modules and top-level configurations intended to provision minimal, opinionated infrastructure on the three main cloud providers (AWS, GCP, Azure). The goal is to get a managed Kubernetes cluster up and running quickly so application teams can deploy and iterate. These modules are intentionally minimal — they provide a baseline environment and are not a drop-in secure production topology.

Key points:
- Providers supported: AWS, GCP, Azure
- Each provider module provisions a managed Kubernetes control plane:
  - AWS: EKS
  - GCP: GKE
  - Azure: AKS
- Modules focus on fast bootstrap and essential resources (VPC/network, cluster, node pools, IAM/service accounts, and basic storage).
- Users MUST extend and harden these modules to meet their organization’s security, compliance, and operational requirements.

Recommended usage
- Use these modules as a starting point for development or proof-of-concept environments.
- Do not treat the out-of-the-box configuration as production-ready.
- Apply provider- and region-specific best practices (network isolation, RBAC, encryption, logging, monitoring, secrets management, least privilege).

Folder structure
- providers/
  - aws/
  - gcp/
  - azure/ (placeholder)

Security & compliance disclaimer
These modules intentionally provide the bare minimum infrastructure to accelerate onboarding. It is the responsibility of the user or their security team to:
- Review and harden IAM, network policies, and encryption defaults.
- Configure audit logging, monitoring, and alerting.
- Integrate with organization identity and secret stores.
- Conduct penetration testing and security reviews as required.

Contributing
- Open a PR with clear intent and tests for any changes to shared modules.
- Add provider-specific docs if adding features beyond the baseline.

License
- Refer to the repository root for licensing information.


## Provider-specific details

### AWS prerequisites
#### Local development:
- AWS CLI installed and configured: https://docs.aws.amazon.com/cli/
- Terraform (recommended >= 1.0)
- AWS credentials with permissions to create VPC, EKS, EC2, IAM, ELB, S3 (or an appropriately-scoped IAM role) 
#### Fork development:
- GitHub OIDC configured as following:
```
resource "aws_iam_role" "github_oidc_role" {
  name        = "github-oidc-role"
  description = "GitHub OIDC iam role"

  assume_role_policy    = file("samples/github_oidc_assume_role_policy.json")
  max_session_duration  = 3600
  force_detach_policies = true

  tags = {
    Environment = "dev"
  }
}

resource "aws_iam_role_policy_attachment" "github_oidc_role_policy_attachment" {
  role       = aws_iam_role.github_oidc_role.name
  policy_arn = data.aws_iam_policy.github_oidc_administrator_policy.arn
}

data "aws_iam_policy" "github_oidc_administrator_policy" {
  name = "AdministratorAccess"
}

```

Example of the policy you can find in `infrastructure/samples/github_oidc_assume_role_policy.json` - Replace `ACCOUNT_ID`, `ORG`, `REPO`, `REF` with respective values, to further harden the policy, you can add filter for your github runners IPs or other organization-specific requirements.

- Following variables should be configured (via ENV vars or GitHub repository secrets):
```
AWS_REGION
AWS_ACCOUNT
AWS_GH_OIDC_ROLE_ARN
```
- (Placeholder) Any org-specific policies, SCPs or MFA requirements

### GCP prerequisites
#### Local development:
- Google Cloud SDK (gcloud) installed and authenticated: https://cloud.google.com/sdk
- Terraform (recommended >= 1.0)
- GCP project with billing enabled
- Service account with roles such as Kubernetes Engine Admin, Compute Admin, Service Account User, Storage Admin (adjust per needs)
- Service account key or ADC available to Terraform
#### Fork development:
- GitHub OIDC configured as following:
```
resource "google_service_account" "github_oidc_sa" {
  project    = var.gcp_project_id
  account_id = var.github_oidc_service_account
}

module "gcp_gh_oidc" {
  source  = "terraform-google-modules/github-actions-runners/google//modules/gh-oidc"
  version = "~> 4.0"

  project_id  = var.gcp_project_id
  pool_id     = "example-pool"
  provider_id = "example-gh-provider"

  attribute_condition = "assertion.repository==\"ORG/REPO\""

  sa_mapping = {
    "${google_service_account.github_oidc_sa.account_id}" = {
      sa_name   = google_service_account.github_oidc_sa.name
      attribute = "attribute.repository/ORG/REPO"
    }
  }
}

```

Replace `ORG`, `REPO`,  with respective values, to further harden the policy, you can add filter for your github runners IPs or other organization-specific requirements.

- Following variables should be configured (via ENV vars or GitHub repository secrets):
```
GCP_REGION
GCP_PROJECT_ID
```
- (Placeholder) Organization policies or IAM constraints

### Azure prerequisites
- Azure CLI installed and logged in: https://docs.microsoft.com/cli/azure
- Terraform (recommended >= 1.0)
- Azure subscription and target resource group
- Service principal or managed identity with permissions to create VNet, AKS, RBAC roles, storage, etc.
- Subscription and tenant IDs available to Terraform
- (Placeholder) Any management group / policy requirements