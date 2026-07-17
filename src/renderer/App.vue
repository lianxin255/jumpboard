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

const hosts = useHostsStore()
const mappings = useMappingsStore()
const templates = useTemplatesStore()
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
      <EngineControl ref="engineRef" />
    </header>
    <main>
      <MappingList @new="openNew" @edit="openEdit" />
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
</style>
