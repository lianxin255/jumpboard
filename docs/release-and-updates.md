# 发布与自动更新

## 现在的状态

打一个 `v*` 标签（必须与 `mygo.json` 的 `version` 一致，例如 `1.2.0` 对应 `v1.2.0`）会触发 `.github/workflows/release.yml`：

- 先建一个 **draft** GitHub Release；
- macOS runner 构建 `darwin/universal`，Windows runner 构建 `windows/amd64`；
- 各自附带 `SHA256SUMS`，上传到同一个 draft。

产物**没有签名**。macOS 上是 ad hoc 签名，Gatekeeper 会拦截别的 Mac 打开；Windows 上没有签名，SmartScreen 会警告。所以现在的产物只供维护者自己安装和分发，不面向公开下载。

## 补上签名需要什么

仓库目前没有这些 secret，补齐后按 [mygo 的 github-actions 文档](https://github.com/egoist/mygo/blob/v0.6.4/docs/github-actions.md) 在 workflow 里加导入步骤即可：

| 平台 | 需要的 secret | 来源 |
|---|---|---|
| macOS | `MACOS_CERTIFICATE`（.p12 的 base64）、`MACOS_CERTIFICATE_PASSWORD`、`APPLE_ID`、`APPLE_TEAM_ID`、`APPLE_APP_PASSWORD` | Apple Developer Program（付费账号）里的 Developer ID Application 证书，加 notarytool 的 app-specific password |
| Windows | `WINDOWS_CERTIFICATE`（.pfx 的 base64）、`WINDOWS_CERTIFICATE_PASSWORD` | 代码签名证书（EV 或 OV），用 `osslsigncode` 签 |

`mygo.json` 里对应加上 `macos.signingIdentity` 与 `macos.notarize`。

## 自动更新为什么先只留文档

mygo 自带签名更新器（`mygo.Updater`）：`mygo keygen` 生成密钥对，公钥写进 `mygo.json` 的 `updates.publicKey` 并指向 `updates.github`，构建时用 `MYGO_UPDATER_PRIVATE_KEY` 给每个平台产出 `update-<platform>.json` 清单和增量包，应用内检查清单、验签、替换自己。

现在不做的原因：

1. **私钥一旦发布就不能换。** 已安装的应用只接受这把密钥签的更新，丢了密钥这些安装就永远更新不了，而密钥泄露等于别人能给所有用户发代码。在还没有签名分发渠道、没有保管密钥的地方（密码管理器 + CI secret）之前生成它，风险大于收益。
2. **更新包与安装包是同一套产物。** 安装包还没有正式签名，自动替换进去的也过不了 Gatekeeper / SmartScreen，用户侧体验和手动下载没有差别。
3. **无密钥时 `mygo build` 本来就会跳过更新文件**，所以现在的发布流水线不需要为它做任何特殊处理。

## 启用时的步骤

1. 在有保管条件的机器上跑 `go tool mygo keygen`，私钥进密码管理器，再 `gh secret set MYGO_UPDATER_PRIVATE_KEY`。
2. `mygo.json` 加：

   ```json
   "updates": { "publicKey": "…mygo-update.pub 的内容…", "github": "lianxin255/jumpboard" }
   ```

3. workflow 的构建命令改为 `go tool mygo build -platform … -upload`（`-upload` 需要密钥，没有会在构建前停下）。
4. 应用里接 `mygo.Updater`：启动后检查、设置页放「检查更新」。发布说明取 `CHANGELOG.md` 里该版本的段落，所以每个版本都要有 `## x.y.z` 小节。
