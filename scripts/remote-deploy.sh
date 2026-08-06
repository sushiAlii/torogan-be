#!/usr/bin/env bash
set -euo pipefail
cd /home/ubuntu

: "${IMAGE_TAG:?}" "${ECR_REGISTRY:?}"
export IMAGE_TAG ECR_REGISTRY

aws ecr get-login-password --region ap-southeast-1 \
  | docker login --username AWS --password-stdin "$ECR_REGISTRY"

docker compose -f docker-compose.prod.yml --profile tools pull postgres backend caddy migrate

# Migrations gate the release: if this fails, the old backend keeps serving.
docker compose -f docker-compose.prod.yml run --rm migrate

docker compose -f docker-compose.prod.yml up -d
docker image prune -f
