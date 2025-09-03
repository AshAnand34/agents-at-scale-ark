locals {
  cluster_type                = "simple-regional"
  default_workload_pool       = "${var.gcp_project_id}.svc.id.goog"
  network_name                = "aas-ark-gke-private-network-${var.gcp_region}"
  subnet_name                 = "aas-ark-gke-private-subnet-${var.gcp_region}"
  master_auth_subnetwork      = "aas-ark-gke-private-master-subnet-${var.gcp_region}"
  pods_range_name             = "ip-range-pods-aas-ark-gke-private"
  services_range_name         = "ip-range-services-aas-ark-gke-private"
  subnet_names                = [for subnet_self_link in module.gcp-network.subnets_self_links : split("/", subnet_self_link)[length(split("/", subnet_self_link)) - 1]]
  github_oidc_service_account = "github-oidc-sa"
}

resource "google_project_iam_member" "member-role" {
  for_each = toset([
    "roles/container.admin",
    "roles/container.clusterAdmin"
  ])
  role    = each.key
  member  = "serviceAccount:${data.google_service_account.github_oidc_sa.email}"
  project = var.gcp_project_id
}

data "google_service_account" "github_oidc_sa" {
  project    = var.gcp_project_id
  account_id = var.github_oidc_service_account
}