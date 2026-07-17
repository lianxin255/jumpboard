<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useHostsStore } from '@renderer/stores/hosts'
import { useMappingsStore } from '@renderer/stores/mappings'
import { useTemplatesStore } from '@renderer/stores/templates'
import type { Template } from '@renderer/stores/templates'
import MappingList from '@renderer/components/MappingList.vue'
import MappingForm from '@renderer/components/MappingForm.vue'
import TemplatePicker from '@renderer/components/TemplatePicker.vue'
import EngineControl from '@renderer/components/EngineControl.vue'
import LogViewer from '@renderer/components/LogViewer.vue'
import HistoryPanel from '@renderer/components/HistoryPanel.vue'

const hosts = useHostsStore()
const mappings = useMappingsStore()
const templates = useTemplatesStore()
const currentView = ref<'mappings' | 'logs' | 'history'>('mappings')
const showForm = ref(false)
const showPicker = ref(false)
const pickedTemplate = ref<Template | null>(null)
const editingId = ref<string | undefined>()
const engineRef = ref()

async function reloadAll() {
  await hosts.load()
  if (hosts.currentHostId) await mappings.load(hosts.currentHostId)
  await templates.load()
}

onMounted(reloadAll)

watch(() => hosts.currentHostId, async (id) => {
  if (id) await mappings.load(id)
})

function openNew() {
  pickedTemplate.value = null
  editingId.value = undefined
  showPicker.value = true
}
function onPick(tpl: Template) {
  pickedTemplate.value = tpl
  showPicker.value = false
  showForm.value = true
}
function openEdit(id: string) { editingId.value = id; showForm.value = true }
async function onSaved() {
  showForm.value = false
  pickedTemplate.value = null
  if (hosts.currentHostId) await mappings.load(hosts.currentHostId)
}
</script>

<template>
  <div class="layout">
    <header class="topbar">
      <h1>跳板机</h1>
      <select v-model="hosts.currentHostId">
        <option v-for="h in hosts.hosts" :key="h.id" :value="h.id">{{ h.name }}</option>
      </select>
      <div class="nav-tabs">
        <button :class="{ active: currentView === 'mappings' }" @click="currentView = 'mappings'">映射</button>
        <button :class="{ active: currentView === 'logs' }" @click="currentView = 'logs'">日志</button>
        <button :class="{ active: currentView === 'history' }" @click="currentView = 'history'">历史</button>
      </div>
      <EngineControl ref="engineRef" />
    </header>
    <main>
      <MappingList v-if="currentView === 'mappings'" @new="openNew" @edit="openEdit" />
      <LogViewer v-else-if="currentView === 'logs'" :host-id="hosts.currentHostId!" />
      <HistoryPanel v-else-if="currentView === 'history'" :host-id="hosts.currentHostId!" @rolled-back="reloadAll" />
    </main>
    <TemplatePicker v-if="showPicker" @pick="onPick" @cancel="showPicker = false" />
    <MappingForm v-if="showForm"
      :host-id="hosts.currentHostId!"
      :mapping-id="editingId"
      :template="pickedTemplate"
      @saved="onSaved"
      @cancel="showForm = false" />
  </div>
</template>

<style scoped>
.layout { display: flex; flex-direction: column; height: 100vh; background: #f5f5f7; }
.topbar { display: flex; align-items: center; gap: 16px; padding: 12px 24px;
  background: rgba(255,255,255,0.7); backdrop-filter: blur(20px); border-bottom: 1px solid rgba(0,0,0,0.06); }
h1 { font-size: 18px; margin: 0; font-weight: 600; }
select { padding: 6px 10px; border-radius: 8px; border: 1px solid #ddd; }
main { flex: 1; overflow-y: auto; }
.nav-tabs { display: flex; gap: 4px; margin-left: 8px; }
.nav-tabs button { padding: 6px 14px; border: none; border-radius: 8px; background: transparent; color: #555; cursor: pointer; font-size: 14px; }
.nav-tabs button.active { background: #0A84FF; color: #fff; }
</style>
