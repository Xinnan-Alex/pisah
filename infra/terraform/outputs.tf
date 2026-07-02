output "aws_region" {
  value = var.aws_region
}

output "ecr_repository_url" {
  description = "Push the backend image here"
  value       = aws_ecr_repository.app.repository_url
}

output "alb_dns_name" {
  description = "ALB hostname (use over http:// until a domain/TLS is configured)"
  value       = aws_lb.app.dns_name
}

output "app_url" {
  description = "Public base URL of the backend"
  value       = local.tls ? "https://${var.domain_name}" : "http://${aws_lb.app.dns_name}"
}

output "cluster_name" {
  value = aws_ecs_cluster.app.name
}

output "service_name" {
  value = aws_ecs_service.app.name
}

output "github_actions_role_arn" {
  description = "Set as AWS_ROLE_ARN in .github/workflows/deploy-backend.yml (or a GitHub repo variable)."
  value       = aws_iam_role.github_actions.arn
}

output "acm_validation_records" {
  description = "CNAME records to add in Cloudflare (DNS only) before HTTPS can finish provisioning."
  value = local.tls && !local.use_route53 ? [
    for dvo in aws_acm_certificate.app[0].domain_validation_options : {
      name  = dvo.resource_record_name
      type  = dvo.resource_record_type
      value = dvo.resource_record_value
    }
  ] : []
}
