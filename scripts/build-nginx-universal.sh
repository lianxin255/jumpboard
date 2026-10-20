#!/usr/bin/env bash
# 为 macOS 构建一份「通用二进制」nginx（arm64 + x86_64），静态链接 OpenSSL 与 PCRE2。
#
# 为什么不用官方/Homebrew 的 nginx：
#   - 它们依赖 @executable_path/lib/*.dylib 或 Homebrew 的绝对路径（/opt/homebrew/...），
#     打包进 .app 后要么找不到，要么写死只在 arm64 机器上有效。
#   - mygo 打包 universal app 时会把 resources/darwin-arm64 与 darwin-amd64 合并，
#     两侧的 dylib 必须成对，容易漏。静态链接后没有 dylib，天然通用。
#
# 产物：resources/darwin/nginx/nginx（单文件，arm64 + x86_64）
#
# 用法：bash scripts/build-nginx-universal.sh
set -euo pipefail

NGINX_VERSION="${NGINX_VERSION:-1.26.3}"
OPENSSL_VERSION="${OPENSSL_VERSION:-3.0.15}"
PCRE2_VERSION="${PCRE2_VERSION:-10.44}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="${WORK:-/tmp/nginx-universal-build}"
OUT="$ROOT/resources/darwin/nginx"

# 与现有 Windows / 旧 macOS 构建保持一致的模块集（stream 21 个）
CONFIGURE_ARGS=(
  --with-stream
  --with-stream_ssl_module
  --with-stream_ssl_preread_module
  --with-stream_realip_module
  --with-http_ssl_module
)

mkdir -p "$WORK" "$OUT"
cd "$WORK"

fetch() { # url file
  [ -f "$2" ] || curl -fsSL -o "$2" "$1"
}

fetch "https://nginx.org/download/nginx-${NGINX_VERSION}.tar.gz" "nginx-${NGINX_VERSION}.tar.gz"
fetch "https://github.com/openssl/openssl/releases/download/openssl-${OPENSSL_VERSION}/openssl-${OPENSSL_VERSION}.tar.gz" "openssl-${OPENSSL_VERSION}.tar.gz"
fetch "https://github.com/PCRE2Project/pcre2/releases/download/pcre2-${PCRE2_VERSION}/pcre2-${PCRE2_VERSION}.tar.gz" "pcre2-${PCRE2_VERSION}.tar.gz"

for f in nginx-*.tar.gz openssl-*.tar.gz pcre2-*.tar.gz; do
  [ -d "${f%.tar.gz}" ] || tar xzf "$f"
done

build_arch() { # arch  -> 按架构编 OpenSSL / PCRE2 / nginx 各一份
  local arch="$1"

  if [ ! -f "$WORK/ssl-$arch/lib/libssl.a" ]; then
    rm -rf "$WORK/openssl-$arch"
    cp -R "$WORK/openssl-${OPENSSL_VERSION}" "$WORK/openssl-$arch"
    ( cd "$WORK/openssl-$arch" \
      && ./Configure "darwin64-${arch}-cc" no-shared no-tests --prefix="$WORK/ssl-$arch" \
      && make -j"$(sysctl -n hw.ncpu)" \
      && make install_sw )
  fi

  if [ ! -f "$WORK/pcre-$arch/lib/libpcre2-8.a" ]; then
    rm -rf "$WORK/pcre-$arch"
    cp -R "$WORK/pcre2-${PCRE2_VERSION}" "$WORK/pcre-$arch"
    ( cd "$WORK/pcre-$arch" \
      && CC="clang -arch $arch" CFLAGS="-arch $arch" LDFLAGS="-arch $arch" \
         ./configure --prefix="$WORK/pcre-$arch" --disable-shared --enable-static --disable-tests \
      && make -j"$(sysctl -n hw.ncpu)" \
      && make install )
  fi

  rm -rf "$WORK/nginx-$arch"
  cp -R "$WORK/nginx-${NGINX_VERSION}" "$WORK/nginx-$arch"
  ( cd "$WORK/nginx-$arch" \
    && ./configure \
        --with-cc-opt="-arch $arch -I$WORK/pcre-$arch/include" \
        --with-ld-opt="-arch $arch -L$WORK/pcre-$arch/lib -lpcre2-8" \
        --with-openssl="$WORK/openssl-$arch" \
        --with-openssl-opt="no-shared no-tests" \
        "${CONFIGURE_ARGS[@]}" \
    && make -j"$(sysctl -n hw.ncpu)" )
}

build_arch arm64
build_arch x86_64

lipo -create "$WORK/nginx-arm64/objs/nginx" "$WORK/nginx-x86_64/objs/nginx" -output "$OUT/nginx"
chmod +x "$OUT/nginx"
cp "$WORK/nginx-${NGINX_VERSION}/conf/mime.types" "$OUT/mime.types"

echo
echo "built: $OUT/nginx"
lipo -info "$OUT/nginx"
otool -L "$OUT/nginx"
