variable "aws_region" {
  type    = string
  default = "ap-southeast-1" # co-located with Supabase + Bedrock
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

# Secret key (sb_secret_...) — Supabase Storage (DuitNow QR + receipt scan images).
# Leave empty only if you omit storage features from the ECS task.
variable "supabase_secret_key" {
  type      = string
  sensitive = true
  default   = ""
}

# OpenAI API key — only when OCR_PROVIDER=openai (legacy fallback).
variable "openai_api_key" {
  type      = string
  sensitive = true
  default   = ""
}

# Receipt OCR settings (plain env vars on the ECS task).
variable "ocr_provider" {
  type        = string
  description = "bedrock (default), openai, or textract. Controls ECS OCR_PROVIDER and which task IAM policy is attached."
  default     = "bedrock"

  validation {
    condition     = contains(["bedrock", "openai", "textract"], var.ocr_provider)
    error_message = "ocr_provider must be bedrock, openai, or textract."
  }
}

variable "ocr_model" {
  type    = string
  default = "global.anthropic.claude-haiku-4-5-20251001-v1:0"
}

variable "ocr_timeout" {
  type    = string
  default = "45s"
}

variable "local_dev_iam_user" {
  type        = string
  description = "IAM user for local dev (AWS_PROFILE). Gets Bedrock + Textract policies so make run works without matching ocr_provider."
  default     = "pisahdamin"
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
