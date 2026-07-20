<script setup lang="ts">
import { ref, computed } from 'vue'
import dayjs from 'dayjs'
import { ReloadOutlined } from '@antdv-next/icons'
import { useLogsStore } from '@renderer/stores/logs'
import { api } from '@renderer/api/ipc'

const store = useLogsStore()
const props = defineProps<{ hostId: string }>()

const nginxAccessLogs = ref<string[]>([])
const nginxErrorLogs = ref<string[]>([])
const activeTab = ref<'app' | 'nginx'>('app')

// 筛选
const levelFilter = ref<'all' | 'error' | 'warn' | 'info'>('all')
const keyword = ref('')

const filteredLogs = computed(() => {
  let list = store.list
  if (levelFilter.value !== 'all') {
    list = list.filter(l => l.level === levelFilter.value)
  }
  if (keyword.value.trim()) {
    const kw = keyword.value.trim().toLowerCase()
    list = list.filter(l =>
      l.message.toLowerCase().includes(kw) ||
      l.source.toLowerCase().includes(kw)
    )
  }
  return list
})

const filteredNginxError = computed(() => {
  if (!keyword.value.trim()) return nginxErrorLogs.value
  const kw = keyword.value.trim().toLowerCase()
  return nginxErrorLogs.value.filter(l => l.toLowerCase().includes(kw))
})
const filteredNginxAccess = computed(() => {
  if (!keyword.value.trim()) return nginxAccessLogs.value
  const kw = keyword.value.trim().toLowerCase()
  return nginxAccessLogs.value.filter(l => l.toLowerCase().includes(kw))
})

async function loadNginxLogs() {
  const r = await api.tailNginxLogs(200)
  nginxAccessLogs.value = r.access
  nginxErrorLogs.value = r.error
}

async function refresh() {
  await store.load(props.hostId, 500)
  await loadNginxLogs()
}
refresh()

function levelColor(level: string): string {
  if (level === 'error') return 'var(--color-danger)'
  if (level === 'warn') return 'var(--color-warning)'
  return 'var(--text-tertiary)'
}
function formatTime(ts: number): string {
  return dayjs(ts).format('YYYY-MM-DD HH:mm:ss')
}
</script>

<template>
  <div class="log-viewer">
    <div class="toolbar">
      <h3>日志</h3>
      <a-input-search
        v-model:value="keyword"
        placeholder="搜索关键词..."
        size="small"
        style="width: 180px"
        allow-clear
      />
      <a-select v-if="activeTab === 'app'" v-model:value="levelFilter" size="small" style="width: 100px">
        <a-select-option value="all">全部级别</a-select-option>
        <a-select-option value="error">仅错误</a-select-option>
        <a-select-option value="warn">仅警告</a-select-option>
        <a-select-option value="info">仅正常</a-select-option>
      </a-select>
      <a-button size="small" @click="refresh">
        <template #icon><ReloadOutlined /></template>
        刷新
      </a-button>
    </div>
    <div class="log-body">
      <a-tabs v-model:activeKey="activeTab" size="small">
        <a-tab-pane key="app" tab="操作日志">
          <div class="log-list">
            <div v-for="log in filteredLogs" :key="log.id" class="log-line">
              <span class="time">{{ formatTime(log.timestamp) }}</span>
              <span class="level" :style="{ color: levelColor(log.level) }">[{{ log.level.toUpperCase() }}]</span>
              <span class="source">({{ log.source }})</span>
              <span class="msg">{{ log.message }}</span>
            </div>
            <div v-if="filteredLogs.length === 0" class="empty">
              {{ store.list.length === 0 ? '暂无日志' : '无匹配结果' }}
            </div>
          </div>
        </a-tab-pane>
        <a-tab-pane key="nginx" tab="Nginx 运行">
          <div class="log-list">
            <div v-if="filteredNginxError.length > 0">
              <div class="log-section-title">错误日志</div>
              <div v-for="(line, i) in filteredNginxError" :key="'e'+i" class="log-line nginx-error">{{ line }}</div>
            </div>
            <div v-if="filteredNginxAccess.length > 0" style="margin-top: 12px">
              <div class="log-section-title">访问日志</div>
              <div v-for="(line, i) in filteredNginxAccess" :key="'a'+i" class="log-line nginx-access">{{ line }}</div>
            </div>
            <div v-if="filteredNginxError.length === 0 && filteredNginxAccess.length === 0" class="empty">
              {{ nginxErrorLogs.length === 0 && nginxAccessLogs.length === 0
                ? '暂无 nginx 运行日志（启动 nginx 并产生流量后会出现）'
                : '无匹配结果' }}
            </div>
          </div>
        </a-tab-pane>
      </a-tabs>
    </div>
  </div>
</template>

<style scoped>
.log-viewer { display: flex; flex-direction: column; height: 100%; }

.toolbar {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
}

h3 { font-size: var(--font-size-lg); margin-right: auto; }

.log-body {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  padding: 0 var(--space-4);
}

.log-list {
  overflow-y: auto;
  font-family: var(--font-mono);
  font-size: var(--font-size-sm);
}

.log-line { padding: 2px 0; display: flex; gap: var(--space-2); }
.time { color: var(--text-tertiary); white-space: nowrap; }
.level { font-weight: 600; white-space: nowrap; }
.source { color: var(--text-tertiary); }
.msg { flex: 1; word-break: break-all; color: var(--text-primary); }

.empty { color: var(--text-tertiary); padding: var(--space-8); text-align: center; }

.log-section-title { font-size: 11px; color: var(--text-tertiary); text-transform: uppercase; margin: 8px 0 4px; }
.nginx-error { color: var(--color-danger); }
.nginx-access { color: var(--text-secondary); }
</style>
