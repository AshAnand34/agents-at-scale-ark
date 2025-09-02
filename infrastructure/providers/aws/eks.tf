module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 21.0"

  name               = var.cluster_name
  kubernetes_version = var.cluster_version

  endpoint_public_access                   = true
  enable_cluster_creator_admin_permissions = true
  deletion_protection                      = true

  compute_config = {
    enabled    = true
    node_pools = ["general-purpose"]
  }

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  tags = local.tags
}

resource "aws_eks_access_entry" "github_access" {
  cluster_name  = module.eks.cluster_name
  principal_arn = data.aws_iam_role.github_oidc_role.arn
}

resource "aws_eks_access_policy_association" "github_access_policy" {
  cluster_name  = module.eks.cluster_name
  policy_arn    = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
  principal_arn = data.aws_iam_role.github_oidc_role.arn

  access_scope {
    type = "cluster"
  }
}