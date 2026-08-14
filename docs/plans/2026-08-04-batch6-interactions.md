# 第六批：前端交互增强（快捷键 + 批量操作 + 克隆映射）

> 状态：规划中
> 起因：剩余候选全部做完。本批做纯前端交互增强，第七批做 IPC 安全加固（后端）。

## 目标
1. 快捷键：Esc 关弹窗（confirmBox/promptBox + ModalShell）、Ctrl+S 保存（MappingForm/HostManager）
2. 批量操作：映射列表多选 + 批量启用/停用/删除
3. 克隆映射：行内克隆按钮，复用现有 templateData 预填机制

## 非目标
- 排序/拖拽（sortOrder 字段存在但拖拽 UI 复杂，单独一批）
- 后端校验（第七批）

## 实施步骤

### Step 1：Esc 关闭弹窗
- feedback.ts：FeedbackHost 的 onMounted 注册 window keydown → Escape 调 closeConfirm(false)。单例挂载，覆盖 confirmBox + promptBox
- ModalShell.vue：setup 里 onMounted/onUnmounted 注册 Escape → emit('cancel')。一处覆盖三个弹窗
- **验证**：typecheck

### Step 2：Ctrl+S 保存
- MappingForm.vue：onMounted/onUnmounted 注册 (ctrl/meta)+s → preventDefault + save()
- HostManager.vue：watch(showForm)，true 时注册 Ctrl+S → save()，false 时注销
- **验证**：typecheck

### Step 3：克隆映射
- MappingList.vue：row-acts 加克隆按钮，emit('clone', m)
- App.vue：onClone(id) → 从 mappings.list 找到，构造 pickedTemplate = { mapping: {...m, name: m.name+' (副本)'} }，editingId=undefined，showForm=true
- MappingForm 零改动（mappingId 空 → 走 create，templateData 预填）
- **验证**：typecheck

### Step 4：批量操作
- MappingList.vue：加 selected = ref<Set<string>>，表头加全选 checkbox，行首加 checkbox，grid 加一列 36px
- 批量工具栏：v-if="selected.size > 0"，显示"已选 N + 批量启用/停用/删除/取消"
- store/mappings.ts：batchToggle(ids, enabled) / batchRemove(ids) 用 Promise.all 复用现有 toggle/remove（乐观更新）
- **验证**：typecheck + 手动核对乐观更新逻辑

### Step 5：验证 + 文档

## 风险
- Ctrl+S 必须 preventDefault（否则触发 Electron 保存网页对话框）
- 批量操作乐观更新：Promise.all 并发，若部分失败状态会不一致——可接受（失败 toast 提示）
- 克隆名称加"(副本)"避免重名

## 进度
- [x] Step 1 Esc 关弹窗（feedback.ts FeedbackHost + ModalShell.vue，两处 window keydown）
- [x] Step 2 Ctrl+S 保存（MappingForm onMounted/onUnmounted + HostManager watch showForm）
- [x] Step 3 克隆映射（MappingList 加克隆按钮 + App.vue cloneMap 复用 templateData 机制）
- [x] Step 4 批量操作（store batchToggle/batchRemove + MappingList 多选 UI + 批量工具栏 + grid 加 32px 列）
- [x] Step 5 验证：typecheck + 130 测试 + build

## 验证
- typecheck ✓ / 130 测试 ✓ / build ✓（CSS +0.4kB batch-bar/checkbox，JS +6kB 交互逻辑）

## review 订正（已完成）
- **Esc 冲突**：confirmBox 在 ModalShell 弹窗内打开时按 Esc，FeedbackHost 和 ModalShell 两个监听都触发，导致 confirmBox 和表单同时关闭。
  - 修复：ModalShell 的 Esc 监听加守卫 `document.querySelector('.fb-overlay')` 检测，confirmBox 打开时让 FeedbackHost 优先处理。
- **Set mutate 不响应**：`selectedIds.value.clear()` 直接 mutate Set，Vue ref 检测不到，批量操作后 checkbox 视觉不更新。
  - 修复：3 处 clear() 改为 `selectedIds.value = new Set()`（替换触发响应式）。
