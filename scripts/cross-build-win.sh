#!/usr/bin/env bash
# macOS 交叉编译 Windows 绿色版（cargo-xwin + llvm）
#
# 前置：
#   brew install llvm
#   cargo install cargo-xwin
#   rustup target add x86_64-pc-windows-msvc
#   pnpm app:build:resources  （预下载 node.exe + better_sqlite3.node）
#
# 用法：
#   bash scripts/cross-build-win.sh           # 只编译 exe
#   bash scripts/cross-build-win.sh --bundle  # 编译 + 打绿色版文件夹
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LLVM_BIN="/opt/homebrew/opt/llvm/bin"
XWIN="$HOME/Library/Caches/cargo-xwin/xwin"
BUNDLE=false

if [ "${1:-}" = "--bundle" ]; then
  BUNDLE=true
fi

# 检查前置条件
if [ ! -f "$LLVM_BIN/llvm-rc" ]; then
  echo "✗ 未找到 llvm-rc，请先运行：brew install llvm"
  exit 1
fi
if ! command -v cargo-xwin &>/dev/null; then
  echo "✗ 未找到 cargo-xwin，请先运行：cargo install cargo-xwin"
  exit 1
fi
if [ ! -f "$ROOT/build/node/node.exe" ]; then
  echo "✗ 未找到 build/node/node.exe，请先运行：pnpm app:build:resources"
  exit 1
fi
if [ ! -f "$ROOT/build/app/better_sqlite3.win32.node" ]; then
  echo "✗ 未找到 build/app/better_sqlite3.win32.node，请先运行：pnpm app:build:resources"
  exit 1
fi

echo "=== 确保构建素材就绪 ==="
cd "$ROOT"
node scripts/build-app.mjs --offline --resources

echo "=== 交叉编译 Windows x86_64 exe ==="
export PATH="$LLVM_BIN:$PATH"
export CC_x86_64_pc_windows_msvc="clang"
export AR_x86_64_pc_windows_msvc="llvm-lib"
export "CFLAGS_x86_64_pc_windows_msvc=--target=x86_64-pc-windows-msvc -I$XWIN/crt/include -I$XWIN/sdk/include/ucrt -I$XWIN/sdk/include/um -I$XWIN/sdk/include/shared"

cd "$ROOT/apps/desktop"
cargo xwin build --release --target x86_64-pc-windows-msvc

EXE="target/x86_64-pc-windows-msvc/release/nginx-jump-gui.exe"
REL="target/x86_64-pc-windows-msvc/release"

if [ ! -f "$EXE" ]; then
  echo "✗ 编译失败：$EXE 不存在"
  exit 1
fi
echo "✓ 编译成功：$EXE ($(du -h "$EXE" | cut -f1))"

if [ "$BUNDLE" = true ]; then
  echo "=== 打绿色版文件夹 ==="
  GREEN="$ROOT/release/NginxJump"
  rm -rf "$GREEN"
  mkdir -p "$GREEN"
  cp "$EXE" "$GREEN/NginxJump.exe"
  cp -r "$REL/bin" "$GREEN/"
  cp -r "$REL/nginx" "$GREEN/"
  cp -r "$REL/server" "$GREEN/"
  cp -r "$REL/web-dist" "$GREEN/"
  cp -r "$REL/templates" "$GREEN/"
  echo "✓ 绿色版打包完成：$GREEN ($(du -sh "$GREEN" | cut -f1))"
  echo ""
  echo "目录结构："
  ls -lh "$GREEN/"
fi
