<script setup lang="ts">
import { api } from '@web/api/http'
import ToggleSwitch from '@web/components/ToggleSwitch.vue'
import { confirmBox, toast } from '@web/feedback'
import { useHostsStore } from '@web/stores/hosts'
import { useMappingsStore } from '@web/stores/mappings'
import { relTime } from '@web/utils/time'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import ForwardChain from './ForwardChain.vue'

const props = defineProps<{ engineStatus: string, engineDirty: boolean, enginePid: number | null }>()
const emit = defineEmits<{
  (e: 'new'): void
  (e: 'edit', id: string): void
  (e: 'clone', id: string): void
  (e: 'refreshEng'): void
}>()

const store = useMappingsStore()
const hosts = useHostsStore()

// 搜索关键字
const search = ref('')
// 待应用改动：来自后端真实 dirty 态（增删改 mapping → 后端置脏；reload 成功 → 清脏）

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q)
    return store.list
  return store.list.filter(m =>
    (m.name + m.upstreamHost + m.listenPort + m.upstreamPort + m.protocol).toLowerCase().includes(q),
  )
})

const statText = computed(() => {
  const total = store.list.length
  const on = store.list.filter(m => m.enabled === 1).length
  return `${total} 条规则 · ${on} 启用 · ${total - on} 停用`
})

// 协议 badge 配置
const protoConfig: Record<string, { color: string, icon: string }> = {
  tcp: { color: 'var(--proto-tcp)', icon: 'M5 8h14M5 12h14M5 16h14' },
  udp: { color: 'var(--proto-udp)', icon: 'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6zM5 8h14M5 16h14' },
  http: { color: 'var(--proto-http)', icon: 'M3 12h18M12 3v18' },
  https: { color: 'var(--proto-https)', icon: 'M5 11h14v9H5zM8 11V7a4 4 0 0 1 8 0v4' },
}

// 健康检查显示
const healthConfig: Record<string, { cls: string, label: string }> = {
  healthy: { cls: 'healthy', label: '健康' },
  unhealthy: { cls: 'unhealthy', label: '异常' },
  unknown: { cls: 'unknown', label: '未知' },
  checking: { cls: 'checking', label: '检查中…' },
}
function healthOf(m: any) {
  if (!m.healthCheckEnabled || m.healthCheckEnabled === 0)
    return { cls: 'none', label: '未启用' }
  return healthConfig[m.healthStatus] || { cls: 'unknown', label: '未知' }
}

// relTime 已抽到 @web/utils/time

// 立即健康检查：调后端单条探活，乐观更新本地状态
async function checkNow(m: any, e: Event) {
  e.stopPropagation()
  const prev = m.healthStatus
  m.healthStatus = 'checking'
  try {
    const status = await api.checkMappingNow(m.id)
    m.healthStatus = status
    toast(`「${m.name}」健康检查: ${status === 'healthy' ? '健康' : status === 'unhealthy' ? '异常' : '未知'}`, status === 'healthy' ? 'ok' : 'info')
  }
  catch {
    m.healthStatus = prev
    toast('健康检查失败', 'err')
  }
}

async function toggleMap(m: any, on: boolean) {
  try {
    await store.toggle(m.id, on ? 1 : 0)
    emit('refreshEng') // 后端已置脏，拉一次让顶栏 pending 条立即出现
    toast(on ? `已启用「${m.name}」` : `已停用「${m.name}」`, on ? 'ok' : 'info')
  }
  catch {
    toast('操作失败', 'err')
  }
}

// 批量选择
const selectedIds = ref<Set<string>>(new Set())
// 切换主机时清空选择，避免旧主机的 mapping id 残留导致批量操作作用于错误主机
watch(() => hosts.currentHostId, () => { selectedIds.value = new Set() })
const allSelected = computed(() => store.list.length > 0 && filtered.value.every(m => selectedIds.value.has(m.id)))
const someSelected = computed(() => selectedIds.value.size > 0 && !allSelected.value)
function toggleSelect(id: string) {
  const s = new Set(selectedIds.value)
  s.has(id) ? s.delete(id) : s.add(id)
  selectedIds.value = s
}
function toggleSelectAll(e: Event) {
  const checked = (e.target as HTMLInputElement).checked
  selectedIds.value = checked ? new Set(filtered.value.map(m => m.id)) : new Set()
}
async function batchToggleSelected(enabled: number) {
  const ids = [...selectedIds.value]
  const ok = await store.batchToggle(ids, enabled)
  emit('refreshEng')
  toast(ok === ids.length
    ? `已批量${enabled ? '启用' : '停用'} ${ok} 条`
    : `已批量${enabled ? '启用' : '停用'} ${ok}/${ids.length} 条（部分失败）`, ok > 0 ? 'ok' : 'err')
  selectedIds.value = new Set()
}
async function batchDeleteSelected() {
  const ids = [...selectedIds.value]
  const ok = await confirmBox({
    title: '批量删除',
    message: `确定要删除选中的 ${ids.length} 条映射吗？`,
    btnText: '删除',
    kind: 'danger',
    impact: '此操作不可撤销，请确认后再继续。',
  })
  if (!ok)
    return
  const deleted = await store.batchRemove(ids)
  emit('refreshEng')
  toast(deleted === ids.length
    ? `已删除 ${deleted} 条`
    : `已删除 ${deleted}/${ids.length} 条（部分失败）`, deleted > 0 ? 'ok' : 'err')
  selectedIds.value = new Set()
}

async function delMap(m: any) {
  const ok = await confirmBox({
    title: '删除映射',
    message: `确定要删除映射「${m.name}」吗？`,
    btnText: '删除',
    kind: 'danger',
    impact: '此操作不可撤销，请确认后再继续。',
  })
  if (!ok)
    return
  try {
    await store.remove(m.id)
    emit('refreshEng')
    toast(`已删除「${m.name}」`, 'ok')
  }
  catch {
    toast('删除失败', 'err')
  }
}

// 应用配置（引擎 bar 的应用按钮）
const applying = ref(false)
async function applyConfig() {
  if (!hosts.currentHostId)
    return
  const ok = await confirmBox({
    title: '应用配置',
    message: '将重新加载 nginx 配置，所有启用规则生效。',
    btnText: '应用',
    kind: 'ok',
  })
  if (!ok)
    return
  applying.value = true
  try {
    // 先 test
    const test = await api.testEngine()
    if (!test.ok) {
      toast(`配置校验失败: ${test.output}`, 'err')
      return
    }
    const r = await api.reloadEngine()
    if (!r.ok) {
      toast(`应用失败: ${r.output}`, 'err')
      return
    }
    toast('配置已应用 · reload 完成', 'ok')
    emit('refreshEng') // reload 成功后端已清脏，拉一次让 pending 条消失
  }
  catch (e: any) {
    toast(`应用失败: ${e.message}`, 'err')
  }
  finally {
    applying.value = false
  }
}

// 健康状态轮询：后端每 30s 探活并写库，前端每 30s 静默重载列表以反映最新健康状态。
// 静默重载（无 toast）避免打断用户。仅在当前页时轮询。
let healthTimer: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  healthTimer = setInterval(async () => {
    if (hosts.currentHostId && document.visibilityState === 'visible') {
      await store.load(hosts.currentHostId)
    }
  }, 30000)
})
onUnmounted(() => {
  if (healthTimer)
    clearInterval(healthTimer)
})
</script>

<template>
  <div class="mapping-view">
    <!-- 引擎状态条 -->
    <div class="eng-bar">
      <span class="pill" :class="props.engineStatus === 'running' ? 'ok' : 'stop'">
        <span class="eng-dot" :class="props.engineStatus === 'running' ? 'run' : 'stop'" style="width:6px;height:6px" />
        nginx 引擎 {{ props.engineStatus === 'running' ? '运行中' : '已停止' }}
      </span>
      <span v-if="props.engineStatus === 'running' && props.enginePid" class="pill">PID {{ props.enginePid }}</span>
      <span v-if="props.engineDirty" class="pending">
        <span class="dot" /> 有改动待应用 · 改动不会立即生效
      </span>
      <div class="spacer" />
      <button class="btn btn-accent" :disabled="applying || props.engineStatus !== 'running'" @click="applyConfig">
        {{ applying ? '应用中…' : '应用配置' }}
      </button>
      <button class="btn btn-ghost" @click="emit('new')">
        + 新建映射
      </button>
    </div>

    <!-- 页头 -->
    <div class="page-hd">
      <div>
        <div class="page-tit">
          端口映射
        </div>
        <div class="page-sub">
          {{ statText }}
        </div>
      </div>
      <div class="search-box">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="m21 21-4.3-4.3" /></svg>
        <input v-model="search" placeholder="搜索规则名称 / 端口 / IP…">
      </div>
    </div>

    <!-- 列表 -->
    <div class="map-list">
      <!-- 加载态 -->
      <div v-if="store.loading" class="empty-state">
        <div class="e-ico" style="animation: pulse 1.2s ease-in-out infinite">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M21 12a9 9 0 1 1-6.2-8.5" /></svg>
        </div>
        <h3>加载中…</h3>
      </div>

      <!-- 空状态 -->
      <div v-else-if="store.list.length === 0" class="empty-state">
        <div class="e-ico">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 7h18M3 12h18M3 17h12" /></svg>
        </div>
        <h3>还没有任何映射规则</h3>
        <p>从模板创建你的第一条端口转发规则，几秒就能搞定。</p>
        <button class="btn btn-accent" @click="emit('new')">
          + 从模板创建
        </button>
      </div>

      <!-- 表格头 -->
      <div v-else class="map-head">
        <span><input type="checkbox" class="cb" :checked="allSelected" :indeterminate.prop="someSelected" @change="toggleSelectAll"></span>
        <span>规则 / 协议</span>
        <span>转发关系</span>
        <span>健康检查</span>
        <span>启用</span>
        <span>最后修改</span>
        <span />
      </div>

      <!-- 批量操作工具栏 -->
      <div v-if="selectedIds.size > 0" class="batch-bar">
        <span class="batch-count">已选 {{ selectedIds.size }} 条</span>
        <button class="btn btn-ghost btn-sm" @click="batchToggleSelected(1)">
          批量启用
        </button>
        <button class="btn btn-ghost btn-sm" @click="batchToggleSelected(0)">
          批量停用
        </button>
        <button class="btn btn-danger btn-sm" @click="batchDeleteSelected">
          批量删除
        </button>
        <button class="btn btn-ghost btn-sm" @click="selectedIds = new Set()">
          取消选择
        </button>
      </div>

      <!-- 表格行 -->
      <div v-for="m in filtered" :key="m.id" class="map-row" :class="{ disabled: m.enabled !== 1 }">
        <!-- 多选 checkbox -->
        <div><input type="checkbox" class="cb" :checked="selectedIds.has(m.id)" @change="toggleSelect(m.id)"></div>
        <!-- 协议 badge（色彩由 protoConfig 驱动，淡色背景+边框由 css 按协议上色） -->
        <div>
          <span class="proto-badge" :class="m.protocol" :style="{ color: protoConfig[m.protocol]?.color }">
            <svg class="p-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <path :d="protoConfig[m.protocol]?.icon || ''" />
            </svg>
            {{ m.protocol.toUpperCase() }}
          </span>
        </div>
        <!-- 规则名 -->
        <div class="map-name" @click="emit('edit', m.id)">
          {{ m.name }}
          <span class="sub">{{ m.protocol.toUpperCase() }} · {{ m.serverNames || m.upstreamHost }}</span>
        </div>
        <!-- 转发关系 -->
        <div style="cursor:pointer" @click="emit('edit', m.id)">
          <ForwardChain
            :protocol="m.protocol" :listen-port="m.listenPort"
            :upstream-host="m.upstreamHost" :upstream-port="m.upstreamPort"
          />
        </div>
        <!-- 健康检查（开启时可点立即检查） -->
        <div>
          <span
            class="health" :class="[healthOf(m).cls, { clickable: m.healthCheckEnabled === 1 }]"
            :title="m.healthCheckEnabled === 1 ? '点击立即检查' : ''"
            @click="m.healthCheckEnabled === 1 && checkNow(m, $event)"
          >
            <span class="h-dot" />{{ healthOf(m).label }}
          </span>
        </div>
        <!-- 启用开关 -->
        <div>
          <ToggleSwitch :model-value="m.enabled === 1" @update:model-value="(v: boolean) => toggleMap(m, v)" />
        </div>
        <!-- 最后修改 -->
        <div class="mod-time">
          {{ relTime(m.updatedAt) }}
        </div>
        <!-- 操作 -->
        <div class="row-acts">
          <button class="icon-btn" title="编辑" @click="emit('edit', m.id)">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M11 4H4v16h16v-7" /><path d="M18.5 2.5a2.1 2.1 0 0 1 3 3L12 15l-4 1 1-4z" /></svg>
          </button>
          <button class="icon-btn" title="克隆" @click="emit('clone', m.id)">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="9" y="9" width="11" height="11" rx="1" /><path d="M5 15V5a1 1 0 0 1 1-1h10" /></svg>
          </button>
          <button class="icon-btn danger" title="删除" @click="delMap(m)">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6" /></svg>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.mapping-view { height: 100%; overflow: auto; }

/* 引擎条 */
.eng-bar {
  display: flex; align-items: center; gap: 14px;
  padding: 10px var(--page-px);
  background: var(--surface-code);
  border-bottom: 1px solid var(--border);
  font-size: 12.5px;
}
.eng-bar .pill {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 3px 9px;
  border-radius: var(--r-sm);
  font-family: var(--font-mono);
  font-size: 11px;
  border: 1px solid var(--border-soft);
}
.eng-bar .pill.ok { color: var(--color-success); border-color: rgba(91,154,74,.4); }
.eng-bar .pill.stop { color: var(--color-danger); border-color: rgba(181,51,51,.4); }
.pending { display: flex; align-items: center; gap: 7px; color: var(--color-warning); font-size: 12px; }
.pending .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--color-warning); animation: pulse 1.6s ease-in-out infinite; }
.spacer { flex: 1; }

/* 页头 */
.page-hd {
  display: flex; align-items: flex-end; justify-content: space-between; gap: 16px;
  padding: 22px var(--page-px) 16px;
  border-bottom: 1px solid var(--border);
}
.page-tit { font-family: var(--font-display); font-size: 26px; font-weight: 500; line-height: 1.1; }
.page-sub { font-size: 12px; color: var(--muted); margin-top: 4px; font-family: var(--font-mono); }

.search-box {
  display: flex; align-items: center; gap: 7px;
  padding: 5px 10px;
  background: var(--surface-3);
  border: 1px solid var(--border-soft);
  border-radius: var(--r);
  width: 240px;
}
.search-box svg { width: 13px; height: 13px; color: var(--meta); flex: none; }
.search-box input { background: none; border: none; outline: none; font-size: 12.5px; width: 100%; color: var(--fg); }
.search-box input::placeholder { color: var(--meta); }
.search-box:focus-within { border-color: var(--color-accent); }

/* 表格 */
.map-list { padding: 6px var(--page-px) 28px; }
.map-head, .map-row {
  display: grid;
  grid-template-columns: 32px 70px 1.4fr 2.3fr 1fr 70px 100px 64px;
  gap: 14px;
  align-items: center;
}
.batch-bar {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 14px;
  background: var(--surface-3);
  border-bottom: 1px solid var(--border-soft);
}
.batch-count { font-size: 12.5px; color: var(--fg); font-weight: 500; margin-right: 4px; }
.cb { width: 14px; height: 14px; cursor: pointer; accent-color: var(--color-accent); }
.map-head {
  padding: 10px 14px;
  font-family: var(--font-mono);
  font-size: 10.5px;
  color: var(--meta);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  border-bottom: 1px solid var(--border);
}
.map-row {
  padding: 13px 14px;
  border-bottom: 1px solid var(--border);
  transition: background var(--transition-fast);
  animation: fadeUp 0.25s var(--ease) both;
}
.map-row:hover { background: var(--surface-3); }
.map-row.disabled { opacity: 0.55; }

.proto-badge {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 3px 7px;
  border-radius: var(--r-sm);
  border: 1px solid color-mix(in oklab, currentColor 45%, transparent);
  background: color-mix(in oklab, currentColor 8%, transparent);
  font-family: var(--font-mono);
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
.p-ico { width: 11px; height: 11px; }

.map-name { font-size: 13.5px; font-weight: 500; color: var(--fg); cursor: pointer; }
.map-name .sub { display: block; font-size: 11px; color: var(--muted); font-weight: 400; margin-top: 2px; font-family: var(--font-mono); }

.health { display: flex; align-items: center; gap: 7px; font-size: 11.5px; font-family: var(--font-mono); color: var(--muted); }
.health .h-dot { width: 7px; height: 7px; border-radius: 50%; flex: none; }
.health.healthy { color: var(--color-success); }
.health.healthy .h-dot { background: var(--color-success); }
.health.unhealthy { color: var(--color-danger); }
.health.unhealthy .h-dot { background: var(--color-danger); }
.health.unknown .h-dot { background: var(--meta); }
.health.checking { color: var(--color-warning); }
.health.checking .h-dot { background: var(--color-warning); animation: pulse 1.2s ease-in-out infinite; }
.health.none .h-dot { background: transparent; border: 1px solid var(--meta); }
.health.clickable { cursor: pointer; }
.health.clickable:hover { opacity: 0.8; }

.mod-time { font-family: var(--font-mono); font-size: 11.5px; color: var(--muted); }
.row-acts { display: flex; gap: 4px; justify-content: flex-end; }

/* 自定义开关已抽为 ToggleSwitch 组件 */
</style>
