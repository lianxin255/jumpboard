# 方案：proxy_pass 自定义字段（proxyPassUrl）

> 状态：✅ 已完成

## 进度
- [x] Step 1 shared/validation.ts 加 isValidProxyPassUrl + 4 条单测
- [x] Step 2 mappingRepo.ts 通读 proxy_pass_url 列（MappingRow + COLUMNS + create/update + rowToMapping）
- [x] Step 3 renderers.ts proxyPassUrl 非空时原样用（抽 renderProxyPass helper，HTTP/HTTPS 共用）
- [x] Step 4 handlers.ts 校验（MAPPING_CREATE + UPDATE）
- [x] Step 5 dataTransfer.ts 导入导出 + rowToMapping
- [x] Step 6 MappingForm.vue UI + FormState + templateToForm/loadExisting/save + templateMapping
- [x] Step 7 验证：typecheck + 164 测试 + build

## 验证
- typecheck ✓ / 164 测试全过（新增 isValidProxyPassUrl 4 + renderers proxyPassUrl 2）✓ / build ✓
- 填 `http://27.196.154.94/` → proxy_pass 原样透传
- 留空 → 自动拼 `http://IP:端口/`（根路径尾斜杠）
- 非法值（含 ;{}# 或不以 http:// 开头）→ handler 拒绝

## 现状

`proxyPassUrl` 字段**半成品**——types.ts:62 有定义、migrations.ts:94 有 ALTER TABLE 加列，但：
- mappingRepo：MappingRow 没声明 `proxy_pass_url`、COLUMNS 没包含、createMapping/updateMapping 不读写
- renderers：proxy_pass 仍硬拼 `http://${upHost}:${port}${trailingSlash}`
- handlers：不校验
- MappingForm：无输入框
- dataTransfer：导入导出不带它

**影响**：DB 有列但永远是 NULL，types 有字段但永远是 null，renderers 永远走 host:port 拼接。

## 方案：打通 proxyPassUrl 全链路

### 数据流设计
- 用户在表单填 proxyPassUrl（可选）
- **填了**：proxy_pass 直接用这个值（原样透传，不拼 host:port）
- **没填**（null/空）：走现有逻辑（host:port + 根路径尾斜杠）

### 改动范围（7 层）

| 层 | 文件 | 改动 |
|---|---|---|
| types.ts | `Mapping.proxyPassUrl` | 已有，无需改 |
| migrations.ts | ALTER TABLE | 已有，无需改 |
| mappingRepo.ts | MappingRow + COLUMNS + create/update | 加 proxy_pass_url 列读写 |
| renderers.ts | renderHttpServer/renderHttpsServer | proxyPassUrl 非空时用它，否则走拼接 |
| handlers.ts | MAPPING_CREATE/UPDATE | 校验 proxyPassUrl 格式（必须 http:// 或 https:// 开头） |
| dataTransfer.ts | 导入导出 | 带 proxyPassUrl |
| MappingForm.vue | FormState + 输入框 + templateToForm/loadExisting/save | 加字段 + UI |

### UI 设计
在"上游服务"区块，upstreamHost/upstreamPort 下方加一个**可选**输入框：
```
proxy_pass 自定义（高级，可选）
[http://27.196.154.94/          ]  留空则自动用 IP:端口拼接
提示：填了此字段后，上面的 IP/端口仅用于显示，proxy_pass 完全用你填的值。
```

### 校验（shared/validation.ts 加一个）
- proxyPassUrl 非空时必须匹配 `^https?://` 开头
- 过滤注入字符（`;{}#` 换行）
- 长度上限 500

### renderers 逻辑
```ts
const proxyPass = m.proxyPassUrl
  ? sanitize(m.proxyPassUrl)                           // 用户自定义，原样用
  : `http://${upHost}:${m.upstreamPort}${trailingSlash}` // 默认拼接
lines.push(`      proxy_pass ${proxyPass};`)
```

## 影响分析

### 向后兼容
- 旧数据 proxyPassUrl = NULL → 走拼接（行为不变）
- 新数据填了 → 原样透传

### 风险
- **注入面**：proxyPassUrl 原样进 conf，必须 sanitize（`;{}#` 换行）。已有 sanitize 函数复用。
- **校验绕过**：renderer 校验可被 DevTools 绕过，handler 层必须也校验（第七批的 validation 模式）。
- **TCP/UDP 不受影响**：stream 块的 proxy_pass 不走 HTTP renderers，不受影响。

### 不改的
- TCP/UDP 映射（stream proxy_pass 语义不同，不含 http://）
- proxyMode（standard/minimal/custom 仍控制 headers，proxyPassUrl 只控制 proxy_pass 那一行）

## 实施步骤
1. shared/validation.ts 加 isValidProxyPassUrl
2. mappingRepo.ts 通读 proxy_pass_url 列
3. renderers.ts proxyPassUrl 非空时用它
4. handlers.ts 校验
5. dataTransfer.ts 导入导出
6. MappingForm.vue UI + FormState
7. 验证 + 文档

## 验证
- 填 `http://27.196.154.94/` → 生成的 conf proxy_pass 完全一致
- 留空 → 走拼接（现有行为）
- 填非法值（含 ; 或换行）→ 被 sanitize / 拒绝
