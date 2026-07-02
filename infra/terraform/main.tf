locals {
  tls             = var.domain_name != "" && var.route53_zone_id != ""
  public_base_url = var.public_base_url != "" ? var.public_base_url : (local.tls ? "https://${var.domain_name}" : "")

  container_secrets = concat(
    [
      { name = "DATABASE_URL", valueFrom = aws_ssm_parameter.database_url.arn },
      { name = "SUPABASE_PUBLISHABLE_KEY", valueFrom = aws_ssm_parameter.supabase_publishable_key.arn },
    ],
    var.supabase_secret_key != "" ? [
      { name = "SUPABASE_SECRET_KEY", valueFrom = aws_ssm_parameter.supabase_secret_key[0].arn },
    ] : [],
  )

  ssm_secret_arns = concat(
    [aws_ssm_parameter.database_url.arn, aws_ssm_parameter.supabase_publishable_key.arn],
    [for p in aws_ssm_parameter.supabase_secret_key : p.arn],
  )
}

# ---- networking: reuse the default VPC + public subnets (no NAT cost) ----
data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

# ---- image registry + logs ----
resource "aws_ecr_repository" "app" {
  name         = var.app_name
  force_delete = true
  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_cloudwatch_log_group" "app" {
  name              = "/ecs/${var.app_name}"
  retention_in_days = 14
}

# ---- secrets ----
resource "aws_ssm_parameter" "database_url" {
  name  = "/${var.app_name}/DATABASE_URL"
  type  = "SecureString"
  value = var.database_url
}

resource "aws_ssm_parameter" "supabase_publishable_key" {
  name  = "/${var.app_name}/SUPABASE_PUBLISHABLE_KEY"
  type  = "SecureString"
  value = var.supabase_publishable_key
}

resource "aws_ssm_parameter" "supabase_secret_key" {
  count = var.supabase_secret_key != "" ? 1 : 0

  name  = "/${var.app_name}/SUPABASE_SECRET_KEY"
  type  = "SecureString"
  value = var.supabase_secret_key
}

# ---- IAM ----
data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

# Execution role: pull image, write logs, read the SSM secret.
resource "aws_iam_role" "exec" {
  name               = "${var.app_name}-exec"
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

resource "aws_iam_role_policy_attachment" "exec_managed" {
  role       = aws_iam_role.exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy" "exec_secrets" {
  name = "read-secrets"
  role = aws_iam_role.exec.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["ssm:GetParameters"]
        Resource = local.ssm_secret_arns
      },
      {
        Effect   = "Allow"
        Action   = ["kms:Decrypt"]
        Resource = ["*"] # default aws/ssm key; scope to the key ARN if you use a CMK
      }
    ]
  })
}

# Task role: what the app itself may call — Textract receipt OCR.
resource "aws_iam_role" "task" {
  name               = "${var.app_name}-task"
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

resource "aws_iam_role_policy" "task_textract" {
  name = "textract"
  role = aws_iam_role.task.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["textract:AnalyzeExpense"]
      Resource = ["*"]
    }]
  })
}

# ponytail: self-service access keys only; scope stays on the caller's own user ARN.
resource "aws_iam_user_policy" "pisahdamin_self_access_keys" {
  name = "self-manage-access-keys"
  user = "pisahdamin"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Sid    = "CreateOwnAccessKeys"
      Effect = "Allow"
      Action = [
        "iam:CreateAccessKey",
        "iam:GetUser",
        "iam:ListAccessKeys",
        "iam:TagUser",
      ]
      Resource = "arn:aws:iam::*:user/$${aws:username}"
    }]
  })
}

# ---- security groups ----
resource "aws_security_group" "alb" {
  name   = "${var.app_name}-alb"
  vpc_id = data.aws_vpc.default.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "svc" {
  name   = "${var.app_name}-svc"
  vpc_id = data.aws_vpc.default.id

  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# ---- load balancer ----
resource "aws_lb" "app" {
  name               = "${var.app_name}-alb"
  load_balancer_type = "application"
  subnets            = data.aws_subnets.default.ids
  security_groups    = [aws_security_group.alb.id]
  idle_timeout       = 4000 # long-lived SSE connections (owner live tracking)
}

resource "aws_lb_target_group" "app" {
  name        = "${var.app_name}-tg"
  port        = 8080
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = data.aws_vpc.default.id

  health_check {
    path    = "/healthz"
    matcher = "200"
  }
}

# ---- TLS (only when a domain + Route53 zone are provided) ----
resource "aws_acm_certificate" "app" {
  count             = local.tls ? 1 : 0
  domain_name       = var.domain_name
  validation_method = "DNS"
  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_route53_record" "cert_validation" {
  for_each = local.tls ? {
    for dvo in aws_acm_certificate.app[0].domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      type   = dvo.resource_record_type
      record = dvo.resource_record_value
    }
  } : {}

  zone_id = var.route53_zone_id
  name    = each.value.name
  type    = each.value.type
  records = [each.value.record]
  ttl     = 60
}

resource "aws_acm_certificate_validation" "app" {
  count                   = local.tls ? 1 : 0
  certificate_arn         = aws_acm_certificate.app[0].arn
  validation_record_fqdns = [for r in aws_route53_record.cert_validation : r.fqdn]
}

# HTTPS listener + HTTP→HTTPS redirect (TLS mode)
resource "aws_lb_listener" "https" {
  count             = local.tls ? 1 : 0
  load_balancer_arn = aws_lb.app.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = aws_acm_certificate_validation.app[0].certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app.arn
  }
}

resource "aws_lb_listener" "http_redirect" {
  count             = local.tls ? 1 : 0
  load_balancer_arn = aws_lb.app.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"
    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

# Plain HTTP listener (no-TLS mode)
resource "aws_lb_listener" "http_plain" {
  count             = local.tls ? 0 : 1
  load_balancer_arn = aws_lb.app.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app.arn
  }
}

resource "aws_route53_record" "app" {
  count   = local.tls ? 1 : 0
  zone_id = var.route53_zone_id
  name    = var.domain_name
  type    = "A"
  alias {
    name                   = aws_lb.app.dns_name
    zone_id                = aws_lb.app.zone_id
    evaluate_target_health = true
  }
}

# ---- ECS Fargate ----
resource "aws_ecs_cluster" "app" {
  name = var.app_name
}

resource "aws_ecs_task_definition" "app" {
  family                   = var.app_name
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = aws_iam_role.exec.arn
  task_role_arn            = aws_iam_role.task.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64" # build images with --platform linux/arm64
  }

  container_definitions = jsonencode([{
    name         = var.app_name
    image        = "${aws_ecr_repository.app.repository_url}:${var.image_tag}"
    essential    = true
    portMappings = [{ containerPort = 8080 }]
    environment = [
      { name = "SUPABASE_URL", value = var.supabase_url },
      { name = "PUBLIC_BASE_URL", value = local.public_base_url },
      { name = "AWS_REGION", value = var.aws_region },
    ]
    secrets = local.container_secrets
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.app.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "app"
      }
    }
  }])
}

resource "aws_ecs_service" "app" {
  name            = var.app_name
  cluster         = aws_ecs_cluster.app.id
  task_definition = aws_ecs_task_definition.app.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = data.aws_subnets.default.ids
    security_groups  = [aws_security_group.svc.id]
    assign_public_ip = true # public subnets, so tasks can reach ECR/Supabase/Textract
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.app.arn
    container_name   = var.app_name
    container_port   = 8080
  }

  depends_on = [
    aws_lb_listener.http_plain,
    aws_lb_listener.https,
  ]
}
