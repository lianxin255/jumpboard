/**
 * 内置最小 mime.types（nginx `types {}` 格式）。
 *
 * confGenerator 生成的 http 块 include mime.types（nginx 标准做法），该文件按
 * nginx 约定相对 conf 目录解析。打包版从 bundled/nginx 复制；用户自备 nginx
 * （如 Homebrew）或 dev 环境没有 bundled 时，缺失会导致 nginx -t 直接失败，
 * 所有配置永远无法应用（Mac e2e 实测踩中）。此时写入这份兜底文件。
 */
export const MINIMAL_MIME_TYPES = `types {
    text/html html htm shtml;
    text/css css;
    text/xml xml;
    text/plain txt md;
    application/javascript js mjs;
    application/json json map;
    application/wasm wasm;
    application/pdf pdf;
    application/zip zip;
    application/octet-stream bin exe dll iso dmg;
    image/png png;
    image/jpeg jpg jpeg;
    image/gif gif;
    image/svg+xml svg svgz;
    image/webp webp;
    image/x-icon ico;
    video/mp4 mp4 m4v;
    font/woff woff;
    font/woff2 woff2;
    font/ttf ttf;
}
`
