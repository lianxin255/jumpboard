<script setup lang="ts">
import type { Template } from '@web/stores/templates'
import { api } from '@web/api/http'
import HistoryPanel from '@web/components/HistoryPanel.vue'
import HostManager from '@web/components/HostManager.vue'
import LogViewer from '@web/components/LogViewer.vue'
import MappingForm from '@web/components/MappingForm.vue'
import MappingList from '@web/components/MappingList.vue'
import SettingsPanel from '@web/components/SettingsPanel.vue'
import TemplatePicker from '@web/components/TemplatePicker.vue'
import { confirmBox, FeedbackHost, toast } from '@web/feedback'
import { useHostsStore } from '@web/stores/hosts'
import { useMappingsStore } from '@web/stores/mappings'
import { useTemplatesStore } from '@web/stores/templates'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
// 注意:引擎控制已内联到顶栏,旧 EngineControl 组件已移除

const hosts = useHostsStore()
const mappings = useMappingsStore()
const templates = useTemplatesStore()
const currentView = ref<'mappings' | 'logs' | 'history' | 'hosts' | 'settings'>('mappings')
const showForm = ref(false)
const showPicker = ref(false)
const pickedTemplate = ref<Template | null>(null)
const editingId = ref<string | undefined>()

// 顶栏主机下拉开关
const hostMenuOpen = ref(false)
function toggleHostMenu(e: Event) { e.stopPropagation(); hostMenuOpen.value = !hostMenuOpen.value }
function closeHostMenu() { hostMenuOpen.value = false }

// 当前主机名（用于顶栏显示，永远显示名称而非 id）
const currentHostName = computed(() => {
  const h = hosts.hosts.find(x => x.id === hosts.currentHostId)
  return h?.name ?? '未选择主机'
})

// 导航项（icon 是 SVG 子元素 innerHTML，v-html 渲染到 <svg> 内）
const navItems = [
  { key: 'mappings', label: '映射', icon: '<path d="M3 7h18M3 12h18M3 17h12"/><circle cx="20" cy="17" r="2"/>' },
  { key: 'logs', label: '日志', icon: '<path d="M14 3v4h4"/><path d="M5 21h14a1 1 0 0 0 1-1V7l-4-4H5a1 1 0 0 0-1 1v13"/><line x1="8" y1="13" x2="16" y2="13"/><line x1="8" y1="17" x2="13" y2="17"/>' },
  { key: 'history', label: '历史', icon: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>' },
  { key: 'hosts', label: '主机', icon: '<rect x="3" y="4" width="18" height="6" rx="1"/><rect x="3" y="14" width="18" height="6" rx="1"/>' },
  { key: 'settings', label: '设置', icon: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.6 1.6 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.6 1.6 0 0 0-1.8-.3 1.6 1.6 0 0 0-1 1.5V21a2 2 0 0 1-4 0v-.1a1.6 1.6 0 0 0-1-1.5 1.6 1.6 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.6 1.6 0 0 0 .3-1.8 1.6 1.6 0 0 0-1.5-1H3a2 2 0 0 1 0-4h.1a1.6 1.6 0 0 0 1.5-1 1.6 1.6 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.6 1.6 0 0 0 1.8.3H9a1.6 1.6 0 0 0 1-1.5V3a2 2 0 0 1 4 0v.1a1.6 1.6 0 0 0 1 1.5 1.6 1.6 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.6 1.6 0 0 0-.3 1.8V9a1.6 1.6 0 0 0 1.5 1H21a2 2 0 0 1 0 4h-.1a1.6 1.6 0 0 0-1.5 1z"/>' },
] as const

// 引擎状态（顶栏精简版）+ dirty + pid + 能力，统一在 refreshEng 拉
const engStatus = ref<string>('unknown')
const engDirty = ref(false)
const engPid = ref<number | null>(null)
const engSupportsProcessControl = ref(true) // 默认 true，避免首帧闪烁；切主机后刷新为真实值
const engBusy = ref(false)
async function refreshEng() {
  try {
    engStatus.value = await api.engineStatus()
    engDirty.value = await api.isEngineDirty()
    // 引擎未运行时没必要拉 pid，省一次 IPC
    engPid.value = engStatus.value === 'running' ? await api.getEnginePid() : null
    // 引擎能力：remote 不支持启停，UI 据此隐藏按钮
    const kind = await api.getEngineKind()
    engSupportsProcessControl.value = kind?.supportsProcessControl ?? true
  }
  catch { /* ignore */ }
}
async function engStart() {
  engBusy.value = true
  try {
    const r = await api.startEngine()
    await refreshEng()
    if (r.error)
      toast(`启动失败: ${r.error}`, 'err')
    else toast('引擎已启动', 'ok')
  }
  catch (e: any) { toast(`启动失败: ${e.message}`, 'err') }
  finally { engBusy.value = false }
}
async function engStop() {
  const ok = await confirmBox({
    title: '停止引擎',
    message: '停止后所有转发规则将立即失效，外部无法访问。',
    btnText: '停止',
    kind: 'warn',
  })
  if (!ok)
    return
  engBusy.value = true
  try {
    const r = await api.stopEngine()
    await refreshEng()
    if (r.error)
      toast(`停止失败: ${r.error}`, 'err')
  }
  catch (e: any) { toast(`停止失败: ${e.message}`, 'err') }
  finally { engBusy.value = false }
}

async function reloadAll() {
  await hosts.load()
  if (hosts.currentHostId)
    await mappings.load(hosts.currentHostId)
  await templates.load()
  await refreshEng()
}

// 引擎状态/待应用标记轮询（10s）。dirty 反映后端真实态，无需靠前端假数据猜。
let pollTimer: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  document.addEventListener('click', closeHostMenu)
  reloadAll()
  pollTimer = setInterval(refreshEng, 10000)
})
onUnmounted(() => {
  document.removeEventListener('click', closeHostMenu)
  if (pollTimer)
    clearInterval(pollTimer)
})

watch(() => hosts.currentHostId, async (id) => {
  if (id)
    await mappings.load(id)
})

const switching = ref(false)
async function pickHost(id: string) {
  if (switching.value)
    return // 防止连续点击并发切换引擎
  switching.value = true
  closeHostMenu()
  try {
    // 真正切换后端活动引擎（local 重建子进程 / remote 重连 SSH），不再只是改前端 ref
    const r = await hosts.switchTo(id)
    if (!r?.ok) { toast(`切换主机失败: ${r?.error || '未知原因'}`, 'err'); return }
    if (hosts.currentHostId)
      await mappings.load(hosts.currentHostId)
    await refreshEng()
    toast('已切换主机', 'info')
  }
  finally {
    switching.value = false
  }
}

function openNew() {
  pickedTemplate.value = null
  editingId.value = undefined
  showPicker.value = true
}
function onPick(tpl: Template | null) {
  pickedTemplate.value = tpl
  showPicker.value = false
  showForm.value = true
}
function openEdit(id: string) { editingId.value = id; showForm.value = true }

// 克隆：用现有映射字段打开新建表单（不带 mappingId → 走 create），名称加 (副本)
function cloneMap(id: string) {
  const m = mappings.list.find(x => x.id === id)
  if (!m)
    return
  pickedTemplate.value = {
    id: '__clone__',
    name: `${m.name || ''} (副本)`,
    icon: m.protocol,
    category: '克隆',
    description: '',
    builtin: false,
    mapping: { ...m, name: `${m.name || ''} (副本)` },
    extraDirectives: m.extraDirectives ?? undefined,
  } as Template
  editingId.value = undefined // 关键：不带 mappingId → 保存时走 createMapping
  showForm.value = true
}
async function onSaved() {
  showForm.value = false
  pickedTemplate.value = null
  if (hosts.currentHostId)
    await mappings.load(hosts.currentHostId)
}
</script>

<template>
  <div class="layout">
    <header class="topbar">
      <!-- 品牌 -->
      <div class="brand">
        <div class="mark">
          跳
        </div>
        <div class="name">
          跳板 Jumpbox
        </div>
        <span class="ver">v1.0.0</span>
      </div>

      <!-- 主机选择器（自定义下拉，显示名称） -->
      <div class="host-sel" :class="{ open: hostMenuOpen }">
        <button @click="toggleHostMenu">
          <svg class="host-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="4" width="18" height="6" rx="1" /><rect x="3" y="14" width="18" height="6" rx="1" /></svg>
          <span class="host-name">{{ currentHostName }}</span>
          <svg class="caret" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m6 9 6 6 6-6" /></svg>
        </button>
        <div class="host-menu" @click.stop>
          <div
            v-for="h in hosts.hosts" :key="h.id" class="item"
            :class="{ cur: h.id === hosts.currentHostId }" @click="pickHost(h.id)"
          >
            <svg class="h-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <template v-if="h.type === 'local'"><rect x="3" y="4" width="18" height="6" rx="1" /><rect x="3" y="14" width="18" height="6" rx="1" /></template>
              <template v-else><rect x="2" y="3" width="20" height="14" rx="2" /><line x1="8" y1="21" x2="16" y2="21" /><line x1="12" y1="17" x2="12" y2="21" /></template>
            </svg>
            <span class="h-nm">{{ h.name }}</span>
            <span class="h-tag">{{ h.type === 'local' ? 'local' : 'ssh' }}</span>
          </div>
          <div class="sep" />
          <div class="item add" @click="currentView = 'hosts'; closeHostMenu()">
            <svg class="h-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M12 5v14M5 12h14" /></svg>
            <span class="h-nm">管理主机…</span>
          </div>
        </div>
      </div>

      <!-- 导航 -->
      <nav class="nav">
        <button
          v-for="n in navItems" :key="n.key"
          :class="{ active: currentView === n.key }" @click="currentView = n.key"
        >
          <svg class="n-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" v-html="n.icon" />
          <span class="n-label">{{ n.label }}</span>
        </button>
      </nav>

      <!-- 引擎控制（顶栏精简版） -->
      <div class="engine">
        <div class="eng-state">
          <span class="eng-dot" :class="engStatus === 'running' ? 'run' : engStatus === 'stopped' ? 'stop' : 'unk'" />
          <span class="lbl">{{ engStatus === 'running' ? '运行中' : engStatus === 'stopped' ? '已停止' : '未知' }}</span>
        </div>
        <div class="eng-btns">
          <button
            v-if="engSupportsProcessControl && engStatus !== 'running'" class="btn btn-accent btn-sm"
            :disabled="engBusy" @click="engStart"
          >
            启动
          </button>
          <button
            v-if="engSupportsProcessControl && engStatus === 'running'" class="btn btn-danger btn-sm"
            :disabled="engBusy" @click="engStop"
          >
            停止
          </button>
        </div>
      </div>
    </header>

    <main class="content">
      <MappingList
        v-if="currentView === 'mappings'"
        :engine-status="engStatus" :engine-dirty="engDirty" :engine-pid="engPid"
        @refresh-eng="refreshEng"
        @new="openNew" @edit="openEdit" @clone="cloneMap"
      />
      <LogViewer v-else-if="currentView === 'logs'" :host-id="hosts.currentHostId!" />
      <HistoryPanel v-else-if="currentView === 'history'" :host-id="hosts.currentHostId!" @rolled-back="reloadAll" />
      <HostManager v-else-if="currentView === 'hosts'" :engine-status="engStatus" />
      <SettingsPanel v-else-if="currentView === 'settings'" />
    </main>

    <TemplatePicker v-if="showPicker" @pick="onPick" @cancel="showPicker = false" />
    <MappingForm
      v-if="showForm"
      :host-id="hosts.currentHostId!"
      :mapping-id="editingId"
      :template-data="pickedTemplate"
      @saved="onSaved"
      @cancel="showForm = false"
    />

    <FeedbackHost />
  </div>
</template>

<style scoped>
.layout {
  display: grid;
  grid-template-rows: 48px 1fr;
  height: 100vh;
}

/* ── 顶栏 ── */
.topbar {
  display: flex;
  align-items: stretch;
  background: var(--bg-topbar);
  border-bottom: 1px solid var(--border-soft);
  position: relative;
  z-index: 30;
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 14px;
  border-right: 1px solid var(--border);
}
.brand .mark {
  width: 22px; height: 22px;
  background: var(--color-accent);
  border-radius: var(--r-sm);
  display: grid; place-items: center;
  font-family: var(--font-mono);
  font-weight: 700;
  font-size: 13px;
  color: var(--fg);
}
.brand .name {
  font-family: var(--font-display);
  font-size: 15px;
  font-weight: 500;
}
.brand .ver {
  font-family: var(--font-mono);
  font-size: 10px;
  color: var(--meta);
  letter-spacing: 0.04em;
}

/* 主机下拉 */
.host-sel { position: relative; border-right: 1px solid var(--border); }
.host-sel > button {
  display: flex; align-items: center; gap: 8px;
  padding: 0 12px; height: 48px;
  font-size: 13px; color: var(--fg);
  background: none; border: none; cursor: pointer;
  transition: background var(--transition-fast);
}
.host-sel > button:hover { background: var(--surface-3); }
.host-ico { width: 14px; height: 14px; color: var(--fg-2); }
.host-name { font-weight: 500; }
.caret { width: 11px; height: 11px; color: var(--muted); transition: transform var(--transition-fast); }
.host-sel.open .caret { transform: rotate(180deg); }
.host-menu {
  position: absolute; top: 48px; left: 0;
  min-width: 260px;
  background: var(--surface);
  border: 1px solid var(--border-soft);
  border-radius: 0 0 var(--r) var(--r);
  box-shadow: var(--shadow-md);
  padding: 4px;
  z-index: 40;
  display: none;
}
.host-sel.open .host-menu { display: block; }
.host-menu .item {
  display: flex; align-items: center; gap: 10px;
  padding: 8px 10px;
  border-radius: var(--r-sm);
  cursor: pointer;
  font-size: 13px;
  color: var(--fg-2);
}
.host-menu .item:hover { background: var(--surface-2); color: var(--fg); }
.host-menu .item.cur { color: var(--fg); background: var(--surface-3); }
.h-ico { width: 14px; height: 14px; flex: none; }
.h-nm { flex: 1; }
.h-tag { font-family: var(--font-mono); font-size: 10px; color: var(--meta); }
.sep { height: 1px; background: var(--border-soft); margin: 4px 0; }
.item.add { color: var(--color-accent); }

/* 导航 */
.nav {
  display: flex; align-items: stretch;
  margin-left: auto;
}
.nav button {
  padding: 0 16px; height: 48px;
  font-size: 13px; color: var(--muted);
  background: none; border: none; cursor: pointer;
  position: relative;
  display: flex; align-items: center; gap: 7px;
  transition: color var(--transition-fast);
}
.n-ico { width: 15px; height: 15px; flex: none; }
.nav button:hover { color: var(--fg-2); }
.nav button.active { color: var(--fg); }
.nav button.active::after {
  content: ""; position: absolute;
  left: 0; right: 0; bottom: -1px;
  height: 2px; background: var(--color-accent);
}

/* 引擎控制 */
.engine {
  display: flex; align-items: center; gap: 10px;
  padding: 0 14px;
  border-left: 1px solid var(--border);
  background: var(--bg-topbar);
}
.eng-state {
  display: flex; align-items: center; gap: 6px;
  font-size: 12px; font-family: var(--font-mono);
}
.eng-state .lbl { color: var(--fg-2); }
.eng-btns { display: flex; gap: 4px; }

.content { overflow: auto; position: relative; }
</style>
