# 发布与自动更新

## 现在的状态

打一个 `v*` 标签（必须与 `mygo.json` 的 `version` 一致，例如 `1.2.0` 对应 `v1.2.0`）会触发 `.github/workflows/release.yml`：

- 先建一个 **draft** GitHub Release；
- macOS runner 构建 `darwin/universal`，Windows runner 构建 `windows/amd64`；
- 各自附带 `SHA256SUMS-macOS` / `SHA256SUMS-Windows` 校验和，上传到同一个 draft；
- 构建带 `MYGO_UPDATER_PRIVATE_KEY`（来自仓库 secret）：**配了 secret 就同时产出签名更新包**（`jumpboard-x.y.z-<platform>.tar.gz` + `update-<platform>.json` 清单 + 增量包；增量包要拿已发布的旧版做底，**首个发布没有**），**没配则跳过更新文件**，流水线照常绿。

**macOS 最低系统 13.0**（`mygo.json` 的 `macos.minimumSystemVersion`）：终端功能随包带 1–2 MB 的 `libghostty-vt`，需要 macOS 13+；Windows 侧要求 10 1809+（ConPTY）。低于下限的系统应用其余功能不受影响，只是终端页不可用。

产物**没有代码签名**。macOS 上是 ad hoc 签名，Gatekeeper 会拦截别的 Mac 打开；Windows 上没有签名，SmartScreen 会警告。所以安装包只供维护者自己安装和分发，不面向公开下载。自动更新用的签名是 mygo 自己的 Ed25519 密钥对（见下），与代码签名无关，不受此影响——但更新替换进来的新版本同样是 ad hoc 签名，未公证，首次打开仍需绕过 Gatekeeper（`xattr -cr /Applications/JumpBoard.app`）。

## 自动更新（已接线）

应用内已接 `mygo.Updater`：设置页「更新」卡片，手动「检查更新」→ 展示版本与 `CHANGELOG.md` 该版本的发布说明 → 「安装并重启」。默认**不做后台自动检查**。开发构建（`go run .`、测试）里卡片显示「开发构建不支持自动更新」。

方案（mygo keygen 密钥体系，不需要 Apple 开发者账号）：

- `mygo keygen` 生成的**公钥**已提交在 `mygo.json` 的 `updates.publicKey`，并指向 `updates.github: "lianxin255/jumpboard"`（应用从本仓库 Releases 的 latest 读 `update-<platform>.json`）。
- **私钥绝不进仓库**：它只存在两处——生成机的 `~/Library/Application Support/mygo/update-keys/mygo-update.key`，和 GitHub 仓库 secret `MYGO_UPDATER_PRIVATE_KEY`。`.gitignore` 与提交历史已核查无 `mygo-update.key`。

### ⚠️ 私钥保管（主人必读）

已安装的应用只认这把公钥签的更新。**私钥丢失 = 所有已安装副本永远无法再自动更新**（只能手动下载覆盖）；私钥泄露 = 别人可以给所有用户推任意代码。请把 `mygo-update.key` 的内容存进密码管理器，再配进 GitHub secret。

### 主人需要做的一次性配置

```sh
# 在生成密钥的机器上（或从密码管理器取出私钥文件）：
gh secret set MYGO_UPDATER_PRIVATE_KEY --repo lianxin255/jumpboard < mygo-update.key
```

配好后，下一个 `v*` tag 的 release 就会带签名更新文件；没配之前的 release 只有安装包，已装应用检查更新会得到「已是最新版本」之外的错误提示（读不到清单），属预期。

### 发布一个新版本的流程

1. `CHANGELOG.md` 加（或补全）`## x.y.z` 小节——构建时它成为更新说明，**没有该小节签名构建会直接失败**；
2. `mygo.json` 的 `version` 改成 `x.y.z`；
3. 提交、打 tag 并推送：`git tag vx.y.z && git push origin vx.y.z`；
4. CI 产出 draft release → 检查产物 → **发布（publish）**。只有发布成 latest 后，已安装的应用才能检查到它（draft 不可见）。

### 首个版本的端到端冒烟（主人手测清单）

配好 secret 后，按上面流程发布 v1.2.0 并安装；再发布一个 v1.2.1（CHANGELOG 随便写一行），然后在装着 v1.2.0 的应用里：设置 → 更新 → 检查更新 → 看到 1.2.1 与说明 → 安装并重启 → 重启后「当前版本」是 1.2.1。

本地已验证的部分（2026-10-27，开发机）：`MYGO_UPDATER_PRIVATE_KEY=… go tool mygo build -platform darwin/universal` 产出 `jumpboard-1.2.0-darwin-universal.tar.gz`、`update-darwin-universal.json`（含版本、取自 CHANGELOG 的说明、URL 与 Ed25519 签名）；文件名无空格，与 release.yml 的改名与校验和步骤不冲突。未做：真实 vA→vB 应用内升级冒烟（需要仓库里存在已发布的签名 release，见上手测清单）。

## 补上代码签名需要什么

仓库目前没有这些 secret，补齐后按 [mygo 的 github-actions 文档](https://github.com/egoist/mygo/blob/v0.6.4/docs/github-actions.md) 在 workflow 里加导入步骤即可：

| 平台 | 需要的 secret | 来源 |
|---|---|---|
| macOS | `MACOS_CERTIFICATE`（.p12 的 base64）、`MACOS_CERTIFICATE_PASSWORD`、`APPLE_ID`、`APPLE_TEAM_ID`、`APPLE_APP_PASSWORD` | Apple Developer Program（付费账号）里的 Developer ID Application 证书，加 notarytool 的 app-specific password |
| Windows | `WINDOWS_CERTIFICATE`（.pfx 的 base64）、`WINDOWS_CERTIFICATE_PASSWORD` | 代码签名证书（EV 或 OV），用 `osslsigncode` 签 |

`mygo.json` 里对应加上 `macos.signingIdentity` 与 `macos.notarize`。
