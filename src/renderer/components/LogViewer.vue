<script setup lang="ts">
import { useLogsStore } from '@renderer/stores/logs'
const store = useLogsStore()
const props = defineProps<{ hostId: string }>()

async function refresh() {
  await store.load(props.hostId, 500)
}
refresh()

function levelColor(level: string): string {
  return level === 'error' ? '#ff3b30' : level === 'warn' ? '#ff9500' : '#8e8e93'
}
function formatTime(ts: number): string {
  return new Date(ts).toLocaleString('zh-CN', { hour12: false })
}
</script>

<template>
  <div class="log-viewer">
    <div class="toolbar">
      <h3>日志</h3>
      <button @click="refresh">刷新</button>
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
.toolbar { display: flex; align-items: center; gap: 12px; padding: 12px 16px; border-bottom: 1px solid #e5e5e5; }
h3 { margin: 0; font-size: 16px; }
button { padding: 4px 12px; border: 1px solid #ddd; border-radius: 6px; background: #fff; cursor: pointer; }
.log-list { flex: 1; overflow-y: auto; padding: 8px 16px; font-family: 'SF Mono', Menlo, monospace; font-size: 12px; }
.log-line { padding: 2px 0; display: flex; gap: 6px; }
.time { color: #8e8e93; white-space: nowrap; }
.level { font-weight: 600; white-space: nowrap; }
.source { color: #8e8e93; }
.msg { flex: 1; word-break: break-all; }
.empty { color: #888; padding: 32px; text-align: center; }
</style>
