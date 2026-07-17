<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@renderer/api/ipc'

const status = ref('unknown')
const busy = ref(false)

async function refresh() { status.value = await api.engineStatus() }
async function start() { busy.value = true; try { status.value = await api.startEngine() } finally { busy.value = false } }
async function stop() { busy.value = true; try { status.value = await api.stopEngine() } finally { busy.value = false } }
async function reload() {
  busy.value = true
  try {
    const r = await api.testEngine()
    if (!r.ok) { alert('配置校验失败:\n' + r.output); return }
    await api.reloadEngine()
    status.value = await api.engineStatus()
  } finally {
    busy.value = false
  }
}

onMounted(refresh)
defineExpose({ refresh })
</script>

<template>
  <div class="engine-ctrl">
    <span class="status-dot" :class="status"></span>
    <span class="status-text">{{ status }}</span>
    <button :disabled="busy || status === 'running'" @click="start">启动</button>
    <button :disabled="busy || status !== 'running'" @click="reload">应用</button>
    <button :disabled="busy || status !== 'running'" @click="stop">停止</button>
  </div>
</template>

<style scoped>
.engine-ctrl { display: flex; align-items: center; gap: 8px; padding: 8px 16px;
  background: #f5f5f7; border-radius: 12px; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; background: #ccc; }
.status-dot.running { background: #34c759; }
.status-dot.stopped { background: #ff3b30; }
.status-text { font-size: 13px; color: #555; margin-right: 8px; }
button { padding: 6px 12px; border: none; border-radius: 6px; background: #0A84FF;
  color: #fff; font-size: 13px; cursor: pointer; }
button:disabled { opacity: 0.4; }
</style>
