variable "aws_region" {
  description = "Target AWS region"
  type        = string
}

variable "cluster_name" {
  description = "Name of the ARK cluster"
  type        = string
  default     = "ark-cluster"
}

variable "cluster_version" {
  description = "Kubernetes version"
  type        = string
  default     = "1.33"
}

variable "github_oidc_role_name" {
  description = "The name of the GitHub OIDC role to associate with the EKS cluster"
  type        = string
  default     = "github-oidc-role"
}