#!/usr/bin/env bash
# Build the server image (ARM64, to match Fargate runtime_platform), push to ECR,
# and roll the ECS service. Run AFTER `terraform apply` has created the ECR repo.
#
#   ./infra/deploy.sh
#
# Requires: aws CLI (authenticated), docker, terraform.
set -euo pipefail

TF_DIR="$(cd "$(dirname "$0")/terraform" && pwd)"
APP_DIR="$(cd "$(dirname "$0")/.." && pwd)"

REGION="$(terraform -chdir="$TF_DIR" output -raw aws_region 2>/dev/null || echo ap-southeast-1)"
REPO="$(terraform -chdir="$TF_DIR" output -raw ecr_repository_url)"
CLUSTER="$(terraform -chdir="$TF_DIR" output -raw cluster_name)"
SERVICE="$(terraform -chdir="$TF_DIR" output -raw service_name)"
REGISTRY="${REPO%/*}"

echo "==> ECR login ($REGISTRY)"
aws ecr get-login-password --region "$REGION" | docker login --username AWS --password-stdin "$REGISTRY"

echo "==> build + push ($REPO:latest)"
docker build --platform linux/arm64 -t "$REPO:latest" "$APP_DIR"
docker push "$REPO:latest"

echo "==> roll service ($CLUSTER/$SERVICE)"
aws ecs update-service --cluster "$CLUSTER" --service "$SERVICE" --force-new-deployment --region "$REGION" >/dev/null

echo "==> done. Watch: aws ecs wait services-stable --cluster $CLUSTER --services $SERVICE --region $REGION"
