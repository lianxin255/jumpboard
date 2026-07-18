<script setup lang="ts">
import { ref } from 'vue'
import { ReloadOutlined } from '@antdv-next/icons'
import { useLogsStore } from '@renderer/stores/logs'
import { api } from '@renderer/api/ipc'

const store = useLogsStore()
const props = defineProps<{ hostId: string }>()

const nginxAccessLogs = ref<string[]>([])
const nginxErrorLogs = ref<string[]>([])
const activeTab = ref<'app' | 'nginx'>('app')

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
  return new Date(ts).toLocaleString('zh-CN', { hour12: false })
}
</script>

<template>
  <div class="log-viewer">
    <div class="toolbar">
      <h3>日志</h3>
      <a-button @click="refresh">
        <template #icon><ReloadOutlined /></template>
        刷新
      </a-button>
    </div>
    <div class="log-body">
      <a-tabs v-model:activeKey="activeTab" size="small">
        <a-tab-pane key="app" tab="操作日志">
          <div class="log-list">
            <div v-for="log in store.list" :key="log.id" class="log-line">
              <span class="time">{{ formatTime(log.timestamp) }}</span>
              <span class="level" :style="{ color: levelColor(log.level) }">[{{ log.level.toUpperCase() }}]</span>
              <span class="source">({{ log.source }})</span>
              <span class="msg">{{ log.message }}</span>
            </div>
            <div v-if="store.list.length === 0" class="empty">暂无日志</div>
          </div>
        </a-tab-pane>
        <a-tab-pane key="nginx" tab="Nginx 运行">
          <div class="log-list">
            <div v-if="nginxErrorLogs.length > 0">
              <div class="log-section-title">错误日志</div>
              <div v-for="(line, i) in nginxErrorLogs" :key="'e'+i" class="log-line nginx-error">{{ line }}</div>
            </div>
            <div v-if="nginxAccessLogs.length > 0" style="margin-top: 12px">
              <div class="log-section-title">访问日志</div>
              <div v-for="(line, i) in nginxAccessLogs" :key="'a'+i" class="log-line nginx-access">{{ line }}</div>
            </div>
            <div v-if="nginxErrorLogs.length === 0 && nginxAccessLogs.length === 0" class="empty">
              暂无 nginx 运行日志（启动 nginx 并产生流量后会出现）
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

h3 { font-size: var(--font-size-lg); }

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
