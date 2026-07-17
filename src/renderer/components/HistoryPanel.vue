<script setup lang="ts">
import { ref } from 'vue'
import { useSnapshotsStore } from '@renderer/stores/snapshots'
import { api } from '@renderer/api/ipc'

const store = useSnapshotsStore()
const props = defineProps<{ hostId: string }>()
const emit = defineEmits<{ (e: 'rolled-back'): void }>()
const selectedId = ref<string | null>(null)
const selectedContent = ref('')
const rolling = ref(false)

async function refresh() {
  await store.load(props.hostId)
  selectedId.value = null
  selectedContent.value = ''
}
refresh()

async function viewContent(id: string) {
  selectedId.value = id
  const snap = await api.getSnapshot(id)
  selectedContent.value = snap?.content ?? ''
}

async function rollback() {
  if (!selectedId.value) return
  if (!confirm('确定回滚到选中的配置？当前配置将被覆盖。')) return
  rolling.value = true
  try {
    const result = await store.rollback(selectedId.value)
    if (!result.ok) { alert('回滚失败:\n' + result.output); return }
    emit('rolled-back')
    await refresh()
  } finally {
    rolling.value = false
  }
}

function formatTime(ts: number): string {
  return new Date(ts).toLocaleString('zh-CN', { hour12: false })
}
</script>

<template>
  <div class="history-panel">
    <div class="toolbar">
      <h3>配置历史</h3>
      <button class="btn btn-ghost" @click="refresh">刷新</button>
      <button class="btn btn-primary" :disabled="!selectedId || rolling" @click="rollback">回滚到选中</button>
    </div>
    <div class="body">
      <div class="snap-list">
        <div v-for="s in store.list" :key="s.id"
          class="snap-item" :class="{ active: s.id === selectedId }"
          @click="viewContent(s.id)">
          <div class="snap-time">{{ formatTime(s.createdAt) }}</div>
          <div class="snap-reason">{{ s.reason }}</div>
        </div>
        <div v-if="store.list.length === 0" class="empty">暂无历史（应用配置后自动生成）</div>
      </div>
      <div class="snap-content">
        <pre v-if="selectedContent">{{ selectedContent }}</pre>
        <div v-else class="empty">选择左侧某条查看内容</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.history-panel { display: flex; flex-direction: column; height: 100%; }

.toolbar {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
}

h3 { font-size: var(--font-size-lg); }

.body { flex: 1; display: flex; overflow: hidden; }

.snap-list {
  width: 240px;
  border-right: 1px solid var(--border-subtle);
  overflow-y: auto;
}

.snap-item {
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
  cursor: pointer;
  transition: background var(--transition-fast);
}

.snap-item:hover { background: var(--bg-hover); }
.snap-item.active { background: var(--color-accent-soft); }

.snap-time { font-size: var(--font-size-sm); color: var(--text-secondary); }
.snap-reason { font-size: var(--font-size-xs); color: var(--text-tertiary); margin-top: 2px; }

.snap-content { flex: 1; overflow: auto; padding: var(--space-3); }

pre {
  margin: 0;
  font-family: var(--font-mono);
  font-size: var(--font-size-sm);
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--text-primary);
}

.empty { color: var(--text-tertiary); padding: var(--space-8); text-align: center; }
</style>
