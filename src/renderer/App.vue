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
import HostManager from '@renderer/components/HostManager.vue'
import SettingsPanel from '@renderer/components/SettingsPanel.vue'

const hosts = useHostsStore()
const mappings = useMappingsStore()
const templates = useTemplatesStore()
const currentView = ref<'mappings' | 'logs' | 'history' | 'hosts' | 'settings'>('mappings')
const showForm = ref(false)
const showPicker = ref(false)
const pickedTemplate = ref<Template | null>(null)
const editingId = ref<string | undefined>()

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
    <header class="topbar glass-panel">
      <h1>跳板机</h1>
      <a-select
        v-model:value="hosts.currentHostId"
        style="width: 160px"
        placeholder="选择主机"
      >
        <a-select-option v-for="h in hosts.hosts" :key="h.id" :value="h.id">{{ h.name }}</a-select-option>
      </a-select>
      <a-tabs v-model:activeKey="currentView" type="card" class="nav-tabs" size="small">
        <a-tab-pane key="mappings" tab="映射" />
        <a-tab-pane key="logs" tab="日志" />
        <a-tab-pane key="history" tab="历史" />
        <a-tab-pane key="hosts" tab="主机" />
        <a-tab-pane key="settings" tab="设置" />
      </a-tabs>
      <EngineControl ref="engineRef" />
    </header>
    <main>
      <MappingList v-if="currentView === 'mappings'" @new="openNew" @edit="openEdit" />
      <LogViewer v-else-if="currentView === 'logs'" :host-id="hosts.currentHostId!" />
      <HistoryPanel v-else-if="currentView === 'history'" :host-id="hosts.currentHostId!" @rolled-back="reloadAll" />
      <HostManager v-else-if="currentView === 'hosts'" />
      <SettingsPanel v-else-if="currentView === 'settings'" />
    </main>
    <TemplatePicker v-if="showPicker" @pick="onPick" @cancel="showPicker = false" />
    <MappingForm v-if="showForm"
      :host-id="hosts.currentHostId!"
      :mapping-id="editingId"
      :template-data="pickedTemplate"
      @saved="onSaved"
      @cancel="showForm = false" />
  </div>
</template>

<style scoped>
.layout { display: flex; flex-direction: column; height: 100vh; }

.topbar {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  padding: var(--space-3) var(--space-6);
}

h1 {
  font-size: var(--font-size-lg);
  font-weight: 600;
  letter-spacing: -0.02em;
  margin: 0;
}

main { flex: 1; overflow-y: auto; }

.nav-tabs {
  flex: 1;
  min-width: 0;
}
.nav-tabs :deep(.ant-tabs-nav) {
  margin: 0;
  min-width: 360px;
}
.nav-tabs :deep(.ant-tabs) {
  width: 100%;
}
</style>
