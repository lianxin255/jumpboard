# 第九批：engine 可测性 + 日志裁剪优化

> 状态：规划中
> 起因：候选池最后两项收尾。

## 目标
1. 日志裁剪优化：trimLogs 的 `NOT IN (SELECT ... LIMIT)` 全表扫描改为基于 timestamp 的范围删除
2. engine 可测性：RemoteNginxEngine 构造时 `new SshClient` 硬编码，改为可注入

## 现状

### 日志裁剪（logger.ts:10 + logRepo.ts:55-61）
- 每次写日志 1% 概率触发 trimLogs(10000)
- trimLogs 用 `DELETE FROM app_logs WHERE id NOT IN (SELECT id ... ORDER BY timestamp DESC LIMIT 10000)`
- 问题：NOT IN + 子查询要扫全表，日志到 10000 条时每次裁剪是 O(N) 全表扫描
- 替代方案：直接用 timestamp 阈值删除——先查出第 N 条的 timestamp，再 `DELETE WHERE timestamp < 阈值`。有索引的话更快

### engine 可测性（remoteEngine.ts:26）
- RemoteNginxEngine 构造函数里 `this.ssh = new SshClient({...})` 硬编码
- 测试只能靠 mock ssh2 模块（现有 remoteEngine.test.ts 就是这么做的）
- 改为可注入：构造函数接受可选的 SshClient 实例，测试时传 mock

## 实施步骤

### Step 1：日志裁剪优化
- logRepo.ts: trimLogs 改用 timestamp 范围删除
  - 先 `SELECT timestamp FROM app_logs ORDER BY timestamp DESC LIMIT 1 OFFSET ?` 拿到第 keepCount 条的时间戳
  - 再 `DELETE FROM app_logs WHERE timestamp < ?` 删除更老的
  - 需确认 app_logs 表有 timestamp 索引（看 migrations.ts）
- logger.ts: 裁剪概率从 1% 调整？保持 1% 即可（频率不是问题，单次开销才是）
- **验证**：单测验证 trimLogs 保留正确条数 + 删除老条目

### Step 2：engine 可测性
- remoteEngine.ts: RemoteEngineConfig 加可选 `sshClient?: SshClient`，构造时优先用注入的
- 保持向后兼容：不传时仍 `new SshClient`（现有 hostEngineSwitch 不用改）
- 现有 remoteEngine.test.ts 的 mock 方式不变（仍是 mock @main/ssh/client 模块），但新增"注入 mock client 直接测试"的能力
- **验证**：现有 remoteEngine 测试全过 + typecheck

### Step 3：验证 + 文档 + review

## 风险
- 低：日志裁剪改 SQL，需确认行为等价（保留最近 N 条不变）
- 低：engine 注入是纯增量（向后兼容）

## 进度
- [x] Step 1 日志裁剪优化（trimLogs 改 rowid NOT IN，比 UUID id 快；timestamp 方案因重复不精确被否决）
- [x] Step 2 engine 可测性（RemoteNginxEngine 构造函数加可选 sshClient 注入 + 新增注入测试）
- [x] Step 3 验证：typecheck + 157 测试 + build

## 决策记录
- **trimLogs 不用 timestamp 范围删除**：Date.now() 毫秒精度，快速连续 addLog 产生相同 timestamp，`WHERE timestamp < 阈值` 无法精确保留 keepCount 条。改用 rowid（隐式自增，有序）NOT IN，精确且比原 UUID id 方案快。注释记录了为何不用 timestamp。
- **engine 注入用可选参数而非工厂**：保持构造函数签名向后兼容（hostEngineSwitch 不传，走原逻辑）。测试传 mock SshClient，不必 mock 整个模块。

## 验证
- typecheck ✓ / 157 测试全过（新增注入测试 1 条）✓ / build ✓
