<script setup lang="ts">
import { useLogsStore } from '@renderer/stores/logs'
const store = useLogsStore()
const props = defineProps<{ hostId: string }>()

async function refresh() {
  await store.load(props.hostId, 500)
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
      <button class="btn btn-ghost" @click="refresh">刷新</button>
    </div>
    <div class="log-list">
      <div v-for="log in store.list" :key="log.id" class="log-line">
        <span class="time">{{ formatTime(log.timestamp) }}</span>
        <span class="level" :style="{ color: levelColor(log.level) }">[{{ log.level.toUpperCase() }}]</span>
        <span class="source">({{ log.source }})</span>
        <span class="msg">{{ log.message }}</span>
      </div>
      <div v-if="store.list.length === 0" class="empty">暂无日志</div>
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

.log-list {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-2) var(--space-4);
  font-family: var(--font-mono);
  font-size: var(--font-size-sm);
}

.log-line { padding: 2px 0; display: flex; gap: var(--space-2); }
.time { color: var(--text-tertiary); white-space: nowrap; }
.level { font-weight: 600; white-space: nowrap; }
.source { color: var(--text-tertiary); }
.msg { flex: 1; word-break: break-all; color: var(--text-primary); }

.empty { color: var(--text-tertiary); padding: var(--space-8); text-align: center; }
</style>
