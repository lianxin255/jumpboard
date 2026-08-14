# 第一批：安全漏洞修复 + Bug 修复 + 死代码清理

> 状态：已完成（含 review 修订）
> 起因：全项目体检报告（基于两个探索 agent + 复核）发现 2 个安全漏洞、1 个真 bug、1 处死代码。

## 目标
修复体检报告里"风险最低、收益最高"的 4 项，作为重构的前置清理。

## 已完成项

### A1 模板路径穿越校验 ✅
- **问题**：`saveCustomTemplate`/`deleteCustomTemplate` 用 `tpl.id` 直接拼文件名，handler 零校验。`tpl.id = '../../evil'` 可写出到任意目录（`templateRepo.ts:101,106`）。
- **修复**：加 `TEMPLATE_ID_RE = /^[a-zA-Z0-9_-]{1,64}$/` 白名单。save 对非法 id 抛错，delete 静默拒绝。
- **测试**：新增 2 条（路径穿越 id save 抛错 + delete 不抛错）。
- **文件**：`src/main/templates/templateRepo.ts`

### A4 白名单 fail-closed 统一 ✅（含 review 修订）
- **问题**：白名单"开启但无有效 IP"两个出口行为相反——ENGINE_RELOAD 拒绝（fail-closed），但 hostEngineSwitch 切主机时静默全放行（fail-open）。**初版只改了 local 分支，remote 被绕过；review 抓到后重做**。
- **最终修复**：
  - 白名单检查提到 local/remote 分支之前，对两种主机都生效（`hostEngineSwitch.ts:25-30`）
  - `switchEngineToHost` 返回类型从 `Promise<boolean>` 改为 `Promise<{ok, error?}>`，三种失败（host 不存在/白名单错/类型无效）各有明确文案
  - handler 透传结果，前端能拿到失败原因
- **级联改动**（体现 review 价值）：hostEngineSwitch 返回类型变更涉及 4 文件——`handlers.ts`（HOST_SWITCH 透传）、`index.ts`（启动调用读 r.ok + 补 logger import）、`stores/hosts.ts`（switchTo 返回完整结果）、`App.vue`（读 r?.error）、`api/ipc.ts`（switchHost 返回类型加 error）
- **文件**：`src/main/nginx/hostEngineSwitch.ts`、`src/main/ipc/handlers.ts`、`src/main/index.ts`、`src/renderer/stores/hosts.ts`、`src/renderer/App.vue`、`src/renderer/api/ipc.ts`

### B1 HostManager 测试连接按钮 ✅（含 review 修订）
- **问题**：行内"测试连接"按钮 `@click="testConn"` 读的是编辑表单的 `form.value.host`（初始空），不是当前行的 host。未开表单点它会拿空 host 去连；编辑过别的机器会用上一台凭据测当前行（张冠李戴）。
- **修复**：`testConn(h?)` 接收可选参数。行内按钮传 `h` → 用 `h.remoteConfig`（密文密码，后端 HOST_TEST_CONNECTION 会 decryptPassword）；表单内按钮不传 → 用表单明文。
- **review 增补**：行内按钮绑 `:disabled="testing"`，避免测 A 时点 B 仍可触发。
- **文件**：`src/renderer/components/HostManager.vue`

### B3 删除死配置 ✅
- **问题**：`ConfGenOptions.nginxLogDir/nginxTempDir` 从未被 generateFullConf 读取（只用硬编码相对路径），但 applyFlow/handlers/hostEngineSwitch 还在传，误导维护者。
- **修复**：ConfGenOptions 只剩 `whitelistIps`；所有传参点和测试调用（17 处）同步清理。
- **文件**：`src/main/nginx/confGenerator.ts`、`src/main/nginx/hostEngineSwitch.ts`、`src/main/ipc/handlers.ts`、`tests/main/nginx/applyFlow.test.ts`、`tests/main/nginx/confGenerator.test.ts`

## review 订正记录
第一轮自我 review 偏肯定，第二轮用挑剔眼光抓到 3 个问题：
1. **A4 白名单检查只对 local 生效，remote 被绕过**（回归）→ 检查提到分支前
2. **switchEngineToHost 返回 false 语义重载**（host 不存在 vs 白名单错共用 false）→ 返回类型改 {ok, error?}
3. **B1 行内按钮没绑 disabled** → 补 `:disabled="testing"`

## 教训
- **review 必须 typecheck + test 在"所有改动闭环"之后重跑一次**，不能只看单点改动。第一轮在 A4 返回值还是 boolean 时跑 typecheck 报"通过"，但那时 App.vue 的 `r?.error` 和 boolean 类型并不匹配——是改成 `{ok,error?}` 后 typecheck 才真正能校验那条链路。
- **"两个出口统一"这类断言要验证全覆盖**：初版只改 local 分支就宣称统一，漏了 remote。

## 验证
- typecheck ✓ / 111 测试全过（新增 2 条路径穿越测试）✓
- 改动面小、风险低
