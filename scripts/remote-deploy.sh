#!/usr/bin/env bash
set -euo pipefail
cd /home/ubuntu

: "${IMAGE_TAG:?}" "${ECR_REGISTRY:?}"
export IMAGE_TAG ECR_REGISTRY

# Render .env from SSM Parameter Store instead of relying on a hand-maintained
# file on the box — the same class of drift that caused the migrations bug.
aws ssm get-parameters-by-path \
  --path /torogan-be/prod/ --with-decryption --region ap-southeast-1 \
  --query 'Parameters[].[Name,Value]' --output text \
  | awk -F'\t' '{ n = split($1, a, "/"); print a[n] "=" $2 }' > .env.new
chmod 600 .env.new
mv .env.new .env

aws ecr get-login-password --region ap-southeast-1 \
  | docker login --username AWS --password-stdin "$ECR_REGISTRY"

docker compose -f docker-compose.prod.yml --profile tools pull postgres backend caddy migrate

# Migrations gate the release: if this fails, the old backend keeps serving.
docker compose -f docker-compose.prod.yml run --rm migrate

docker compose -f docker-compose.prod.yml up -d
docker image prune -f
