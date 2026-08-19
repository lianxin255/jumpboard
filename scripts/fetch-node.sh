#!/usr/bin/env bash
# 下载 win32-x64 node 运行时到 build/node/
# Node v22.x（ABI v127，与 better-sqlite3 prebuilt 匹配）
set -euo pipefail

NODE_VERSION="v22.11.0"
ARCH="win-x64"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/build/node"

if [ -f "$DEST/node.exe" ]; then
  echo "[fetch-node] node.exe 已存在，跳过"
  exit 0
fi

mkdir -p "$DEST"
URL="https://nodejs.org/dist/${NODE_VERSION}/node-${NODE_VERSION}-${ARCH}.zip"
TMP="$ROOT/build/.node-download"

echo "[fetch-node] 下载 $URL"
curl -sL "$URL" -o "$TMP.zip"

# 解压 zip（macOS 用 unzip）
rm -rf "$TMP"
mkdir -p "$TMP"
unzip -q "$TMP.zip" -d "$TMP"

# 复制 node.exe 到 build/node/
cp "$TMP/node-${NODE_VERSION}-${ARCH}/node.exe" "$DEST/node.exe"

# 清理
rm -rf "$TMP" "$TMP.zip"
echo "[fetch-node] 完成: $DEST/node.exe"
