# 自定义 proxy_pass 输入

> 起因：用户想完全控制 proxy_pass 的值（带不带尾斜杠、端口、路径），而非工具用 host:port 拼。

## 方案
加可选字段 `proxyPassUrl`。填了 → `proxy_pass ${用户输入};`（sanitize 后）；不填 → 默认 `http://host:port/`。

## 改动清单
1. `shared/types.ts`：Mapping/MappingInput 加 `proxyPassUrl?: string | null`
2. `db/migrations.ts`：`ALTER TABLE mappings ADD COLUMN proxy_pass_url TEXT`
3. `db/mappingRepo.ts`：COLUMNS + rowToMapping + create + update
4. `nginx/renderers.ts`：有 proxyPassUrl 时用它（sanitize 过滤 ;{}#），否则 host:port 拼
5. `renderer/MappingForm.vue`：转发关系区加可选输入框 + hint
6. `renderer/utils/templateMapping.ts`：处理新字段
7. `services/dataTransfer.ts`：导入导出
8. tests：confGenerator 测试加用例

## sanitize
proxy_pass 合法值含 `http://host:port/path` 的 `:` `/`，现有 sanitize 过滤 `[;{}#\n\r]` 正好——保留 `:` `/`，挡住指令注入。

## 进度
- [ ] Step 1 类型 + DB 迁移 + repo
- [ ] Step 2 renderers 优先用自定义值
- [ ] Step 3 表单输入框
- [ ] Step 4 templateMapping + dataTransfer
- [ ] Step 5 验证 + 文档
