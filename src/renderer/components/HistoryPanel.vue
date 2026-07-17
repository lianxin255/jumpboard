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
      <button @click="refresh">刷新</button>
      <button :disabled="!selectedId || rolling" @click="rollback">回滚到选中</button>
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
.toolbar { display: flex; align-items: center; gap: 12px; padding: 12px 16px; border-bottom: 1px solid #e5e5e5; }
h3 { margin: 0; font-size: 16px; }
button { padding: 4px 12px; border: 1px solid #ddd; border-radius: 6px; background: #fff; cursor: pointer; }
button:disabled { opacity: 0.4; }
.body { flex: 1; display: flex; overflow: hidden; }
.snap-list { width: 240px; border-right: 1px solid #e5e5e5; overflow-y: auto; }
.snap-item { padding: 10px 14px; border-bottom: 1px solid #f0f0f0; cursor: pointer; }
.snap-item:hover { background: #f5f5f7; }
.snap-item.active { background: #e8f2ff; }
.snap-time { font-size: 12px; color: #555; }
.snap-reason { font-size: 11px; color: #888; margin-top: 2px; }
.snap-content { flex: 1; overflow: auto; padding: 12px; }
pre { margin: 0; font-family: 'SF Mono', Menlo, monospace; font-size: 12px; white-space: pre-wrap; word-break: break-all; }
.empty { color: #888; padding: 32px; text-align: center; }
</style>
