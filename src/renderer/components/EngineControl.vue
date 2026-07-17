<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '@renderer/api/ipc'

const status = ref('unknown')
const busy = ref(false)
const error = ref('')

async function refresh() { try { status.value = await api.engineStatus() } catch (e: any) { error.value = e.message } }
async function start() {
  busy.value = true; error.value = ''
  try { status.value = await api.startEngine() }
  catch (e: any) { error.value = e.message; alert('启动失败:\n' + e.message) }
  finally { busy.value = false }
}
async function stop() {
  busy.value = true; error.value = ''
  try { status.value = await api.stopEngine() }
  catch (e: any) { error.value = e.message; alert('停止失败:\n' + e.message) }
  finally { busy.value = false }
}
async function reload() {
  busy.value = true; error.value = ''
  try {
    const r = await api.testEngine()
    if (!r.ok) { alert('配置校验失败:\n' + r.output); return }
    const result = await api.reloadEngine()
    if (!result.ok) { alert('应用失败:\n' + result.output); return }
    status.value = await api.engineStatus()
  } catch (e: any) { error.value = e.message; alert('应用失败:\n' + e.message) }
  finally { busy.value = false }
}

onMounted(refresh)
defineExpose({ refresh })
</script>

<template>
  <div class="engine-ctrl">
    <span class="status-dot" :class="status"></span>
    <span class="status-text">{{ status }}</span>
    <span v-if="error" class="error-text" :title="error">⚠</span>
    <button class="btn btn-primary" :disabled="busy || status === 'running'" @click="start">启动</button>
    <button class="btn btn-primary" :disabled="busy || status !== 'running'" @click="reload">应用</button>
    <button class="btn btn-primary" :disabled="busy || status !== 'running'" @click="stop">停止</button>
  </div>
</template>

<style scoped>
.engine-ctrl {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: var(--bg-glass);
  backdrop-filter: blur(var(--blur-sm));
  -webkit-backdrop-filter: blur(var(--blur-sm));
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-pill);
  margin-left: auto;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: var(--radius-pill);
  background: var(--text-tertiary);
  flex-shrink: 0;
}
.status-dot.running { background: var(--color-success); }
.status-dot.stopped { background: var(--color-danger); }

.status-text {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin-right: var(--space-2);
}

.error-text {
  color: var(--color-danger);
  font-size: var(--font-size-md);
  cursor: help;
}

/* 覆盖 .btn 默认 padding，让引擎控制按钮更紧凑 */
.engine-ctrl .btn {
  padding: var(--space-1) var(--space-3);
  font-size: var(--font-size-sm);
}
</style>
