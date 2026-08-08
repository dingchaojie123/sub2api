#!/usr/bin/env bash
# 本地构建镜像的快速脚本，避免在命令行反复输入构建参数。
# 可选：设置 `SUB2API_IMAGE` 指定构建产物的镜像地址。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
IMAGE_TAG="${SUB2API_IMAGE:-croge-registry.cn-beijing.cr.aliyuncs.com/croge/croge-token:latest}"

docker build \
    -t "${IMAGE_TAG}" \
    --platform linux/amd64 \
    --build-arg GOPROXY=https://goproxy.cn,direct \
    --build-arg GOSUMDB=sum.golang.google.cn \
    -f "${REPO_ROOT}/Dockerfile" \
    "${REPO_ROOT}"
