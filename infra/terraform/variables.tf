variable "aws_region" {
  type    = string
  default = "ap-southeast-1" # co-located with Supabase + Textract
}

variable "app_name" {
  type    = string
  default = "pisah"
}

variable "image_tag" {
  type    = string
  default = "latest"
}

# Supabase session-pooler connection string WITH the db password. Sensitive —
# stored in SSM Parameter Store (SecureString), injected as a container secret.
variable "database_url" {
  type      = string
  sensitive = true
}

variable "supabase_url" {
  type    = string
  default = "https://zqvkqcphgncaelrlvcbs.supabase.co"
}

# Publishable key (sb_publishable_...) — GoTrue sign-in proxy via apikey header.
# Dashboard → Settings → API Keys → Publishable and secret API keys.
variable "supabase_publishable_key" {
  type      = string
  sensitive = true
}

# Secret key (sb_secret_...) — optional; for server-side Supabase API calls.
# Not used by the Go backend today (Postgres is direct). Leave empty to omit.
variable "supabase_secret_key" {
  type      = string
  sensitive = true
  default   = ""
}

# Public URL of THIS backend (for friend share links). Leave empty to default to
# https://<domain_name> when a domain is set; otherwise set it to the ALB DNS
# (http://...) after the first apply and re-apply.
variable "public_base_url" {
  type    = string
  default = ""
}

# Custom domain → enables HTTPS (ACM) on the ALB. DNS can live in Cloudflare or
# Route53; set route53_zone_id only if AWS should manage validation + alias records.
variable "domain_name" {
  type    = string
  default = ""
}

variable "route53_zone_id" {
  type        = string
  default     = ""
  description = "Optional Route53 hosted zone. Leave empty when DNS is in Cloudflare."
}

variable "desired_count" {
  type    = number
  default = 1
}

variable "cpu" {
  type    = number
  default = 256
}

variable "memory" {
  type    = number
  default = 512
}

# GitHub Actions OIDC (see github_oidc.tf). Set create_github_oidc_provider = false
# if this account already has token.actions.githubusercontent.com registered.
variable "github_repository" {
  type        = string
  description = "GitHub org/repo allowed to assume the deploy role."
  default     = "Xinnan-Alex/pisah"
}

variable "github_deploy_branch" {
  type        = string
  description = "Branch ref allowed to assume the deploy role."
  default     = "main"
}

variable "create_github_oidc_provider" {
  type        = bool
  description = "Create the GitHub OIDC provider in IAM. Set false if it already exists in this account."
  default     = true
}
