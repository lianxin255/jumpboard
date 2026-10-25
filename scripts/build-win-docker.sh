#!/usr/bin/env bash
# macOS 上用 Docker Wine 交叉编译 Windows portable exe
# 前置：Docker Desktop 运行中
#
# 用法：bash scripts/build-win-docker.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== Docker Wine 交叉编译 Windows portable ==="
docker run --rm \
  -v "$ROOT:/project" \
  -v "$HOME/.cache/electron:/root/.cache/electron" \
  -v "$HOME/.cache/electron-builder:/root/.cache/electron-builder" \
  -w /project \
  electronuserland/builder:wine \
  bash -c "npm install && npx electron-vite build && npx electron-builder --win portable --config apps/desktop/electron-builder.yml"

echo "=== 完成 ==="
echo "产物：$ROOT/release/NginxJump-*.exe"
