<script setup lang="ts">
import { ref } from 'vue'
import { useSnapshotsStore } from '@renderer/stores/snapshots'
import { api } from '@renderer/api/ipc'
import { toast, confirmBox } from '@renderer/feedback'
import { formatTime } from '@renderer/utils/time'

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

async function copyContent() {
  if (!selectedContent.value) return
  try {
    await navigator.clipboard.writeText(selectedContent.value)
    toast('已复制配置原文', 'ok')
  } catch {
    toast('复制失败', 'err')
  }
}

async function rollback() {
  if (!selectedId.value) return
  const ok = await confirmBox({
    title: '回滚配置',
    message: '将回滚到选中版本，当前配置会被覆盖。回滚后仍可再次回滚到其它版本。',
    btnText: '回滚',
    kind: 'warn',
  })
  if (!ok) return
  rolling.value = true
  try {
    const result = await store.rollback(selectedId.value!)
    if (!result.ok) { toast('回滚失败: ' + result.output, 'err'); return }
    toast('已回滚', 'ok')
    emit('rolled-back')
    await refresh()
  } catch (e: any) {
    toast('回滚失败: ' + e.message, 'err')
  } finally {
    rolling.value = false
  }
}

// formatTime 已抽到 @renderer/utils/time

const selectedSnap = () => store.list.find(s => s.id === selectedId.value)
</script>

<template>
  <div class="history-view">
    <div class="page-hd">
      <div>
        <div class="page-tit">配置历史</div>
        <div class="page-sub">每次「应用配置」自动存档 · 可随时回滚</div>
      </div>
    </div>

    <div class="body">
      <!-- 左栏：快照列表 -->
      <div class="snap-list">
        <div class="snap-list-hd">快照 · 共 {{ store.list.length }} 个</div>
        <div v-for="s in store.list" :key="s.id" class="hist-item"
          :class="{ active: s.id === selectedId }" @click="viewContent(s.id)">
          <div class="hist-time">{{ formatTime(s.createdAt) }}</div>
          <div class="hist-reason">{{ s.reason }}</div>
          <span v-if="s.id === store.list[0]?.id" class="tag current">current</span>
          <span v-else-if="s.reason.includes('回滚')" class="tag rollback">rollback</span>
          <span v-if="s.id === selectedId" class="active-bar"></span>
        </div>
        <div v-if="store.list.length === 0" class="empty-state" style="padding:40px">
          暂无历史（应用配置后自动生成）
        </div>
      </div>

      <!-- 右栏：配置内容 -->
      <div class="snap-detail">
        <div class="detail-hd">
          <div class="detail-meta" v-if="selectedSnap()">
            快照 <b>{{ selectedSnap()?.id }}</b> · {{ selectedSnap()?.reason }} · {{ formatTime(selectedSnap()!.createdAt) }}
          </div>
          <div class="detail-meta" v-else>选择左侧某条查看内容</div>
          <div class="btn-row">
            <button class="btn btn-ghost btn-sm" :disabled="!selectedId" @click="copyContent">复制原文</button>
            <button class="btn btn-danger btn-sm" :disabled="!selectedId || rolling" @click="rollback">回滚到此版本</button>
          </div>
        </div>
        <pre v-if="selectedContent" class="code-block">{{ selectedContent }}</pre>
        <div v-else class="empty-state" style="padding:60px">选择左侧某条查看内容</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.history-view { height: 100%; display: flex; flex-direction: column; overflow: hidden; }

.page-hd { padding: 22px var(--page-px) 16px; border-bottom: 1px solid var(--border); }
.page-tit { font-family: var(--font-display); font-size: 26px; font-weight: 500; }
.page-sub { font-size: 12px; color: var(--muted); margin-top: 4px; font-family: var(--font-mono); }

.body { flex: 1; min-height: 0; display: grid; grid-template-columns: 340px 1fr; }

.snap-list { border-right: 1px solid var(--border); overflow: auto; background: var(--surface-code); }
.snap-list-hd { padding: 14px 18px 10px; font-family: var(--font-mono); font-size: 10.5px; color: var(--meta); letter-spacing: 0.08em; text-transform: uppercase; border-bottom: 1px solid var(--border); }

.hist-item { padding: 13px 18px; border-bottom: 1px solid var(--border); cursor: pointer; position: relative; transition: background var(--transition-fast); }
.hist-item:hover { background: var(--surface-3); }
.hist-item.active { background: var(--surface-3); }
.hist-time { font-family: var(--font-mono); font-size: 11px; color: var(--muted); margin-bottom: 3px; }
.hist-reason { font-size: 13px; color: var(--fg); font-weight: 500; }
.tag { display: inline-block; font-family: var(--font-mono); font-size: 9.5px; padding: 1px 5px; border-radius: var(--r-sm); margin-top: 5px; letter-spacing: 0.04em; }
.tag.current { background: var(--surface-2); color: var(--fg-2); }
.tag.rollback { color: var(--color-warning); background: rgba(201,162,58,.1); }
.active-bar { position: absolute; left: 0; top: 0; bottom: 0; width: 2px; background: var(--color-accent); }

.snap-detail { display: flex; flex-direction: column; overflow: hidden; }
.detail-hd { display: flex; align-items: center; justify-content: space-between; padding: 14px 24px; border-bottom: 1px solid var(--border); gap: 12px; }
.detail-meta { font-family: var(--font-mono); font-size: 12px; color: var(--fg-2); }
.detail-meta b { color: var(--fg); font-weight: 600; }

.code-block { flex: 1; overflow: auto; padding: 20px 24px; font-family: var(--font-mono); font-size: 12.5px; line-height: 1.7; color: var(--fg-2); background: var(--surface-code); margin: 0; white-space: pre-wrap; word-break: break-all; }
</style>
