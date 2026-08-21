#!/usr/bin/env bash
# 下载 win32-x64 better-sqlite3 prebuilt（node ABI v127，匹配 Node v22）
# 从 WiseLibs/better-sqlite3 GitHub releases 获取
set -euo pipefail

VERSION="v12.11.1"
ABI="v127"
ARCH="win32-x64"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/build/app/better_sqlite3.win32.node"

if [ -f "$DEST" ]; then
  echo "[fetch-sqlite] better_sqlite3.win32.node 已存在，跳过"
  exit 0
fi

# 离线模式：文件缺失时报错，不尝试联网下载
if [ "${NGJ_OFFLINE:-0}" = "1" ]; then
  echo "[fetch-sqlite] ✗ 离线模式：build/app/better_sqlite3.win32.node 不存在" >&2
  echo "[fetch-sqlite]   请在有网的机器上运行 pnpm app:build:resources 预下载" >&2
  exit 1
fi

mkdir -p "$(dirname "$DEST")"
URL="https://github.com/WiseLibs/better-sqlite3/releases/download/${VERSION}/better-sqlite3-${VERSION}-node-${ABI}-${ARCH}.tar.gz"
TMP="$ROOT/build/.sqlite-download"

echo "[fetch-sqlite] 下载 $URL"
curl -sL "$URL" -o "$TMP.tar.gz"

# 解压 tar.gz
rm -rf "$TMP"
mkdir -p "$TMP"
tar xzf "$TMP.tar.gz" -C "$TMP"

# 找到 .node 文件并复制
NODE_FILE=$(find "$TMP" -name "better_sqlite3.node" -type f | head -1)
if [ -z "$NODE_FILE" ]; then
  echo "[fetch-sqlite] 错误：未找到 better_sqlite3.node"
  exit 1
fi
cp "$NODE_FILE" "$DEST"

# 清理
rm -rf "$TMP" "$TMP.tar.gz"
echo "[fetch-sqlite] 完成: $DEST"
