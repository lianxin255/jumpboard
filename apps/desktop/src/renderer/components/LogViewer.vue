<script setup lang="ts">
import { api } from '@renderer/api/ipc'
import ToggleSwitch from '@renderer/components/ToggleSwitch.vue'
import { confirmBox, toast } from '@renderer/feedback'
import { useLogsStore } from '@renderer/stores/logs'
import { absTime, relTime } from '@renderer/utils/time'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'

const props = defineProps<{ hostId: string }>()
const store = useLogsStore()
const nginxAccessLogs = ref<string[]>([])
const nginxErrorLogs = ref<string[]>([])
const activeTab = ref<'app' | 'nginx'>('app')
const ngxSub = ref<'err' | 'acc'>('err')

const levelFilter = ref<'all' | 'error' | 'warn' | 'info'>('all')
const keyword = ref('')

// nginx 日志读取开关（关闭后不读 nginx 日志文件，避免大日志 OOM）
const ngxLogReadEnabled = ref(true)
async function loadNgxReadSetting() {
  try { ngxLogReadEnabled.value = await api.isNginxLogReadEnabled() }
  catch { /* 读失败时保守默认为关闭（开关语义就是防 OOM） */ ngxLogReadEnabled.value = false }
}
async function onToggleNgxRead(next: boolean) {
  try {
    await api.setNginxLogReadEnabled(next)
    ngxLogReadEnabled.value = next
    toast(next ? 'nginx 日志读取已开启' : 'nginx 日志读取已关闭', 'info')
    if (!next) { nginxAccessLogs.value = []; nginxErrorLogs.value = [] }
    else {
      await loadNginxLogs()
    }
  }
  catch (e: any) {
    toast(`切换失败: ${e?.message ?? '未知错误'}`, 'err')
  }
}

const filteredLogs = computed(() => {
  let list = store.list
  if (levelFilter.value !== 'all')
    list = list.filter(l => l.level === levelFilter.value)
  if (keyword.value.trim()) {
    const kw = keyword.value.trim().toLowerCase()
    list = list.filter(l => l.message.toLowerCase().includes(kw) || l.source.toLowerCase().includes(kw))
  }
  return list
})

const filteredNginxError = computed(() => {
  if (!keyword.value.trim())
    return nginxErrorLogs.value
  const kw = keyword.value.trim().toLowerCase()
  return nginxErrorLogs.value.filter(l => l.toLowerCase().includes(kw))
})
const filteredNginxAccess = computed(() => {
  if (!keyword.value.trim())
    return nginxAccessLogs.value
  const kw = keyword.value.trim().toLowerCase()
  return nginxAccessLogs.value.filter(l => l.toLowerCase().includes(kw))
})

async function loadNginxLogs() {
  try {
    const r = await api.tailNginxLogs(200)
    nginxAccessLogs.value = r.access
    nginxErrorLogs.value = r.error
  }
  catch { /* engine not running */ }
}

async function refresh() {
  await store.load(props.hostId, 5000)
  if (ngxLogReadEnabled.value)
    await loadNginxLogs()
  toast('日志已刷新', 'ok')
}

// 清空当前 Tab 的日志
async function clearCurrentLogs() {
  if (activeTab.value === 'app') {
    const ok = await confirmBox({
      title: '清空操作日志',
      message: '将删除全部操作日志记录，此操作不可撤销。',
      btnText: '清空',
      kind: 'danger',
    })
    if (!ok)
      return
    try {
      await api.clearLogs()
      store.list = []
      toast('操作日志已清空', 'ok')
    }
    catch (e: any) {
      toast(`清空失败: ${e?.message ?? '未知错误'}`, 'err')
    }
  }
  else {
    const ok = await confirmBox({
      title: '清空 Nginx 日志',
      message: '将清空 nginx 的 access.log 和 error.log 文件内容。',
      btnText: '清空',
      kind: 'danger',
    })
    if (!ok)
      return
    try {
      const success = await api.clearNginxLogs()
      if (!success) { toast('清空失败', 'err'); return }
      nginxAccessLogs.value = []
      nginxErrorLogs.value = []
      toast('Nginx 日志已清空', 'ok')
    }
    catch (e: any) {
      toast(`清空失败: ${e?.message ?? '未知错误'}`, 'err')
    }
  }
}

// 先加载开关设置，再 refresh（避免首次加载绕过开关读 nginx 日志）
onMounted(async () => {
  await loadNgxReadSetting()
  refresh()
  // 轮询：操作日志 10s 刷新（让标题"实时刷新"名副其实）。
  // tab 不可见时暂停，避免后台无意义 IPC；可见时恢复。
  document.addEventListener('visibilitychange', onVisibility)
  startPolling()
})

// 切换主机时立即重新加载，否则 10s 轮询间隔内显示的是旧主机日志
watch(() => props.hostId, () => { refresh() })

// 轮询 + 可见性控制（组件销毁时清理，避免泄漏）
let pollTimer: ReturnType<typeof setInterval> | null = null
function startPolling() {
  if (pollTimer)
    return
  pollTimer = setInterval(refresh, 10000)
}
function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}
function onVisibility() {
  if (document.visibilityState === 'visible')
    startPolling()
  else stopPolling()
}
onUnmounted(() => {
  stopPolling()
  document.removeEventListener('visibilitychange', onVisibility)
})

// relTime + absTime 已抽到 @renderer/utils/time

function levelClass(level: string): string {
  if (level === 'error')
    return 'lvl-error'
  if (level === 'warn')
    return 'lvl-warn'
  return 'lvl-info'
}

// 解析 access 日志状态码
function parseStatusCode(line: string): string | null {
  const m = line.match(/"\w+\s+\S+\s[^"]*"\s+(\d{3})/)
  return m ? m[1] : null
}
// 解析 nginx error log 行首时间（格式：2026/07/30 14:32:08）
function parseNgxTime(line: string): string {
  const m = line.match(/^(\d{4}\/\d{2}\/\d{2}\s+\d{2}:\d{2}:\d{2})/)
  return m ? m[1].replace(/^\d{4}\/\d{2}\/\d{2}\s+/, '') : ''
}
function statusClass(code: string): string {
  const n = parseInt(code)
  if (n >= 500)
    return 'lvl-error'
  if (n >= 400)
    return 'lvl-warn'
  return 'lvl-info'
}

const levelOptions = [
  { key: 'all', label: '全部' },
  { key: 'info', label: 'INFO' },
  { key: 'warn', label: 'WARN' },
  { key: 'error', label: 'ERROR' },
] as const
</script>

<template>
  <div class="log-viewer">
    <!-- 页头 -->
    <div class="page-hd">
      <div>
        <div class="page-tit">
          运行日志
        </div>
        <div class="page-sub">
          实时刷新 · 最多保留 5000 条
        </div>
      </div>
    </div>

    <!-- Tab -->
    <div class="log-tabs">
      <button :class="{ active: activeTab === 'app' }" @click="activeTab = 'app'">
        操作日志 <span class="cnt">{{ store.list.length }}</span>
      </button>
      <button :class="{ active: activeTab === 'nginx' }" @click="activeTab = 'nginx'">
        Nginx 日志 <span class="cnt">{{ nginxErrorLogs.length + nginxAccessLogs.length }}</span>
      </button>
    </div>

    <!-- 工具栏 -->
    <div class="log-toolbar">
      <div class="search-box">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="m21 21-4.3-4.3" /></svg>
        <input v-model="keyword" placeholder="过滤关键字…">
      </div>
      <div v-if="activeTab === 'app'" class="lvl-filter">
        <button
          v-for="o in levelOptions" :key="o.key"
          :class="{ active: levelFilter === o.key }" @click="levelFilter = o.key"
        >
          {{ o.label }}
        </button>
      </div>
      <div style="flex:1" />
      <!-- nginx 日志读取开关（仅 nginx tab 显示） -->
      <div v-if="activeTab === 'nginx'" class="ngx-read-toggle" title="关闭后不读取 nginx 日志文件，避免大日志占用内存">
        <ToggleSwitch :model-value="ngxLogReadEnabled" size="sm" @update:model-value="onToggleNgxRead" />
        <span>读取日志文件</span>
      </div>
      <button class="btn btn-ghost btn-sm" title="清空当前标签页的日志" @click="clearCurrentLogs">
        <svg class="b-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6" /></svg>
        清空
      </button>
      <button class="btn btn-ghost btn-sm" @click="refresh">
        <svg class="b-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-3-6.7L21 8" /><path d="M21 3v5h-5" /></svg>
        刷新
      </button>
    </div>

    <!-- 操作日志 -->
    <div v-if="activeTab === 'app'" class="log-table">
      <div v-for="log in filteredLogs" :key="log.id" class="log-row">
        <span class="log-time">{{ relTime(log.timestamp) }}<span class="abs">{{ absTime(log.timestamp) }}</span></span>
        <span class="lvl" :class="levelClass(log.level)">{{ log.level.toUpperCase() }}</span>
        <span class="msg">{{ log.message }}</span>
      </div>
      <div v-if="filteredLogs.length === 0" class="empty-state" style="padding:40px">
        {{ store.list.length === 0 ? '暂无日志' : '无匹配结果' }}
      </div>
    </div>

    <!-- Nginx 日志 -->
    <div v-else class="log-table">
      <div class="ngx-sub-tabs">
        <button :class="{ active: ngxSub === 'err' }" @click="ngxSub = 'err'">
          Error 日志
        </button>
        <button :class="{ active: ngxSub === 'acc' }" @click="ngxSub = 'acc'">
          Access 日志
        </button>
      </div>

      <template v-if="ngxSub === 'err'">
        <div v-for="(line, i) in filteredNginxError" :key="`e${i}`" class="log-row ngx-row ngx-err-row">
          <span class="log-time-sm">{{ parseNgxTime(line) }}</span>
          <span class="lvl" :class="line.toLowerCase().includes('error') ? 'lvl-error' : 'lvl-warn'">ERR</span>
          <span class="msg">{{ line }}</span>
        </div>
      </template>
      <template v-else>
        <div v-for="(line, i) in filteredNginxAccess" :key="`a${i}`" class="log-row ngx-row">
          <span class="lvl" :class="parseStatusCode(line) ? statusClass(parseStatusCode(line)!) : 'lvl-info'">{{ parseStatusCode(line) || '—' }}</span>
          <span class="msg">{{ line }}</span>
        </div>
      </template>

      <div v-if="(ngxSub === 'err' ? filteredNginxError : filteredNginxAccess).length === 0" class="empty-state" style="padding:40px">
        暂无 nginx 运行日志（启动 nginx 并产生流量后会出现）
      </div>
    </div>
  </div>
</template>

<style scoped>
.log-viewer { height: 100%; display: flex; flex-direction: column; overflow: hidden; }

.page-hd { padding: 22px var(--page-px) 16px; border-bottom: 1px solid var(--border); }
.page-tit { font-family: var(--font-display); font-size: 26px; font-weight: 500; }
.page-sub { font-size: 12px; color: var(--muted); margin-top: 4px; font-family: var(--font-mono); }

.log-tabs { display: flex; padding: 0 var(--page-px); border-bottom: 1px solid var(--border); }
.log-tabs button { padding: 12px 16px; font-size: 13px; color: var(--muted); background: none; border: none; cursor: pointer; display: flex; align-items: center; gap: 7px; }
.log-tabs button.active { color: var(--fg); position: relative; }
.log-tabs button.active::after { content: ''; position: absolute; left: 0; right: 0; bottom: -1px; height: 2px; background: var(--color-accent); }
.cnt { font-family: var(--font-mono); font-size: 10px; color: var(--meta); }

.log-toolbar { display: flex; align-items: center; gap: 10px; padding: 12px var(--page-px); border-bottom: 1px solid var(--border); background: var(--surface-code); }
.search-box { display: flex; align-items: center; gap: 7px; padding: 5px 10px; background: var(--surface-3); border: 1px solid var(--border-soft); border-radius: var(--r); width: 240px; }
.search-box svg { width: 13px; height: 13px; color: var(--meta); flex: none; }
.search-box input { background: none; border: none; outline: none; font-size: 12.5px; width: 100%; color: var(--fg); }
.search-box input::placeholder { color: var(--meta); }

.lvl-filter { display: flex; gap: 2px; background: var(--surface-3); border: 1px solid var(--border-soft); border-radius: var(--r); padding: 2px; }
.lvl-filter button { padding: 4px 9px; font-size: 11px; border-radius: var(--r-sm); color: var(--muted); background: transparent; border: none; cursor: pointer; font-family: var(--font-mono); }
.lvl-filter button.active { color: var(--fg); background: var(--surface-2); }

.log-table { flex: 1; overflow: auto; padding: 0 var(--page-px) 28px; font-family: var(--font-mono); font-size: 12px; }
.log-row { display: grid; grid-template-columns: 150px 60px 1fr; gap: 14px; padding: 6px 10px; border-bottom: 1px solid var(--border-hair); align-items: baseline; }
.log-row.ngx-row { grid-template-columns: 60px 1fr; }
.log-row.ngx-err-row { grid-template-columns: 70px 60px 1fr; }
.log-time-sm { color: var(--meta); font-size: 11px; }
.log-time { color: var(--meta); font-size: 11px; }
.log-time .abs { display: block; color: var(--meta); font-size: 10px; }
.msg { color: var(--fg-2); line-height: 1.5; word-break: break-all; }
.lvl { font-size: 10px; font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; text-align: center; padding: 1px 0; border-radius: var(--r-sm); }
.lvl-info { color: var(--fg-2); background: rgba(176,174,165,.1); }
.lvl-warn { color: var(--color-warning); background: rgba(201,162,58,.12); }
.lvl-error { color: var(--color-danger); background: rgba(181,51,51,.14); }

.ngx-sub-tabs { display: flex; gap: 0; padding: 8px 0; }
.ngx-sub-tabs button { padding: 6px 12px; font-size: 12px; color: var(--muted); background: none; border: none; cursor: pointer; }
.ngx-sub-tabs button.active { color: var(--fg); border-bottom: 2px solid var(--color-accent); }

.ngx-read-toggle { display: flex; align-items: center; gap: 6px; font-size: 11.5px; color: var(--muted); cursor: pointer; }
</style>
