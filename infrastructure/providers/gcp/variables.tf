variable "gcp_region" {
  description = "Target GCP region"
  type        = string
  default     = "europe-west3"
}

variable "gcp_project_id" {
  description = "Target GCP project ID"
  type    = string
  default = "proj-01k2hqw21jzb2"
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

variable "github_oidc_service_account" {
  description = "The name of the GitHub OIDC service account to associate with the GKE cluster"
  type        = string
  default     = "github-oidc-sa"    
}