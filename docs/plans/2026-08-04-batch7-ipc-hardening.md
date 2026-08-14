# 第七批：IPC 安全加固

> 状态：规划中
> 起因：42 个 handler 中约 30 个裸抛 rejection、24 个带参数的完全不校验。渲染进程校验可被 DevTools/直接调 window.electronAPI 绕过，main 侧必须补齐。

## 目标
1. 抽 `src/shared/validation.ts`（前后端共用的校验工具）
2. 高危 handler 加输入校验（WHITELIST_ADD / MAPPING_CREATE / MAPPING_UPDATE / PORT_CHECK / LOG_LIST / NGINX_LOG_TAIL）
3. ENGINE_RELOAD 补 try/catch（裸抛漏网）

## 非目标（务实边界）
- **不引入 wrapIpc 全量包装**：agent 分析指出全量改 {ok,data} 会牵动所有 store，改动面过大、风险高。本批只补高危校验 + 个别 try/catch，不统一返回形态。
- **不校验全部 42 个 handler**：只校验 6 个高危入口，其余保持现状（读操作裸抛 rejection 影响小）。
- SSH 超时、importData 结构校验：留后续批次。

## 实施步骤

### Step 1：抽 shared/validation.ts
从 SettingsPanel.vue:22（validIpOrCidr）和 renderers.ts:15（safePort）提取，前后端共用：
- `isValidIpOrCidr(s): boolean` — IPv4 或 CIDR（各段 0-255，掩码 0-32）
- `isValidPort(p): boolean` — 1-65535 整数
- `clampInt(n, min, max): number` — 夹紧数值
- `isValidProtocol(v): boolean` — tcp/udp/http/https
- `isValidProxyMode(v): boolean` — standard/minimal/custom

### Step 2：高危 handler 加校验
- WHITELIST_ADD：ip 用 isValidIpOrCidr，非法返回 {ok:false,error}
- MAPPING_CREATE/UPDATE：protocol 用 isValidProtocol，port 用 isValidPort，非法拒绝
- PORT_CHECK：port 用 isValidPort，非法返回 false
- LOG_LIST：limit 用 clampInt(limit,0,5000)
- NGINX_LOG_TAIL：lines 用 clampInt(lines,1,10000)
- SettingsPanel.vue 改用 @shared/validation（删本地 validIpOrCidr）

### Step 3：ENGINE_RELOAD 补 try/catch
handlers.ts:127-147 的 applyMappings await 无 try，补上转 {ok:false,output}

### Step 4：验证 + 文档 + review

## 风险
- 低：校验函数从现有代码提取，行为等价
- WHITELIST_ADD 加校验后返回形态从 throw 变 {ok:false}——需确认前端 SettingsPanel 适配

## 进度
- [x] Step 1 shared/validation.ts + 19 条单测（isValidIpOrCidr/isValidPort/clampInt/isValidProtocol/isValidProxyMode）
- [x] Step 2 高危 handler 校验（WHITELIST_ADD/MAPPING_CREATE/MAPPING_UPDATE/PORT_CHECK/LOG_LIST/NGINX_LOG_TAIL）+ SettingsPanel 改用 @shared/validation
- [x] Step 3 ENGINE_RELOAD 补 try/catch（防 generateFullConf 等逃逸异常裸抛）
- [x] Step 4 验证：typecheck + 149 测试 + build

## 决策记录
- **不引入 wrapIpc 全量包装**：全量改 {ok,data} 牵动所有 store，风险高收益不明确。本批只补高危校验。
- **校验失败用 throw 而非 {ok:false}**：保持与现有 DB 约束失败的契约一致，前端 catch 能感知，无需适配返回类型。
- **isValidPort 接受字符串数字**：IPC 层不保证类型（表单 input 可能传 string），Number("80")=80 合法。测试确认。

## 验证
- typecheck ✓ / 149 测试全过（新增 validation 19 条）✓ / build ✓
