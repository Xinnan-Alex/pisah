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

# Public URL of THIS backend (for friend share links). Leave empty to default to
# https://<domain_name> when a domain is set; otherwise set it to the ALB DNS
# (http://...) after the first apply and re-apply.
variable "public_base_url" {
  type    = string
  default = ""
}

# Optional custom domain + Route53 hosted zone → enables HTTPS (ACM) on the ALB.
# Without these the ALB serves HTTP only (fine for testing; iOS needs HTTPS for prod).
variable "domain_name" {
  type    = string
  default = ""
}

variable "route53_zone_id" {
  type    = string
  default = ""
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
