# 内置 nginx

这个目录用于存放绿色版打包时捆绑的 Windows nginx 二进制文件。

## 打包前准备（Plan 6）

1. 从 http://nginx.org/en/download.html 下载 Windows 稳定版 zip（如 nginx-1.x.x.zip）
2. 解压后把 `nginx.exe` 和 `conf/mime.types` 放到本目录
3. 运行 `npm run build:portable`，electron-builder 会把本目录内容打包进 resources

## 运行时

程序启动时会在此目录寻找 `nginx.exe`（Windows）或 `nginx`（Unix）。
若 Host 配置了自定义 nginxPath 则优先用那个。

## 开发期

本目录在开发模式下不需要真实 nginx.exe。
`npm run dev` 启动后，引擎的 start/test 会因找不到二进制而报错——这是预期行为。
真实验证需在 Windows 上做（见 docs/plans Plan 1 Task 12）。
