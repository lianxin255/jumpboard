# 第八批：性能优化 + 清理收尾

> 状态：规划中
> 起因：候选池剩余项收尾。NGINX_LOG_TAIL 全量读 OOM 风险 + preload 破坏性 API 面收窄 + 版本号/toFwdSlash 清理。

## 目标
1. NGINX_LOG_TAIL 改尾部倒读 N 行（防大日志 OOM）
2. preload 破坏性 API（wipeAllData）加二次确认
3. 清理：版本号三处统一 + toFwdSlash 三份合并

## 非目标
- 拖拽排序（sortOrder，需要新 UI + reorder API，单独一批）
- wrapIpc 全量包装（已决策不做）

## 实施步骤

### Step 1：NGINX_LOG_TAIL 尾部倒读
当前 handlers.ts 全量 readFileSync + split + slice(-N)。大 access.log（几百 MB）会打爆主进程内存。
改为：用文件 stat 拿大小，从尾部向前读（fs.createReadStream + 起始 offset），按 \n 切分取最后 N 行。
- 新建 `src/main/utils/tailFile.ts`：`tailFile(path, lines): Promise<string[]>`
- handlers.ts 的 NGINX_LOG_TAIL 改用 tailFile
- **验证**：单测（大文件 mock + 小文件正确性）

### Step 2：preload 破坏性 API 二次确认
wipeAllData 是最危险操作（清空全部数据）。当前 handler 无参、靠 UI confirmBox 自律。
改为：handler 要求传 confirmToken（如 'CONFIRM_WIPE'），前端 SettingsPanel 传这个 token 表示已确认。
- 不改 confirmBox 流程（UI 已有确认弹窗），只在 IPC 层加 token 校验
- **验证**：typecheck

### Step 3：清理
- 版本号：App.vue "v0.1.0"、SettingsPanel "0.1.0" 统一读 package.json（或都改成 "1.0.0"）
- toFwdSlash：localEngine.ts:40/157 + confGenerator.ts:17 合并到 shared 或 utils

### Step 4：验证 + 文档 + review

## 进度
- [x] Step 1 NGINX_LOG_TAIL 尾部倒读（tailFile.ts + 7 条单测，含大文件跨 CHUNK）
- [x] Step 2 wipeAllData 二次确认 token（CONFIRM_WIPE，四层链路：handler→preload→api→SettingsPanel）
- [x] Step 3 清理（版本号统一 1.0.0 + toFwdSlash 抽 @shared/paths + 删 confGenerator 死代码 confPath + 删 handlers unused readFileSync import）
- [x] Step 4 验证：typecheck + 156 测试 + build

## review 订正
- **handlers readFileSync 变 unused**：tailFile 替换后 readFileSync 不再调用，import 残留。删除。
- **confGenerator confPath 死代码**：定义了从未调用（第二批删 nginxLogDir 时遗留），删除。
- **localEngine 三处 toFwdSlash 不一致**：:47/:189 有 win32 条件，:164 没有。合并到 @shared/paths 后统一（非 win32 原样返回）。

## 验证
- typecheck ✓ / 156 测试全过（新增 tailFile 7 条）✓ / build ✓
