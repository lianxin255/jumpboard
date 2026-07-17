<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message } from 'antdv-next'
import { PlayCircleOutlined, ReloadOutlined, PauseCircleOutlined } from '@antdv-next/icons'
import { api } from '@renderer/api/ipc'

const status = ref('unknown')
const busy = ref(false)
const error = ref('')

async function refresh() { try { status.value = await api.engineStatus() } catch (e: any) { error.value = e.message } }
async function start() {
  busy.value = true; error.value = ''
  try {
    const r = await api.startEngine()
    status.value = r.status
    if (r.error) { error.value = r.error; message.error('启动失败: ' + r.error) }
  }
  catch (e: any) { error.value = e.message; message.error('启动失败: ' + e.message) }
  finally { busy.value = false }
}
async function stop() {
  busy.value = true; error.value = ''
  try {
    const r = await api.stopEngine()
    status.value = r.status
    if (r.error) { error.value = r.error; message.error('停止失败: ' + r.error) }
  }
  catch (e: any) { error.value = e.message; message.error('停止失败: ' + e.message) }
  finally { busy.value = false }
}
async function reload() {
  busy.value = true; error.value = ''
  try {
    const r = await api.testEngine()
    if (!r.ok) { message.error('配置校验失败: ' + r.output); return }
    const result = await api.reloadEngine()
    if (!result.ok) { message.error('应用失败: ' + result.output); return }
    status.value = await api.engineStatus()
  } catch (e: any) { error.value = e.message; message.error('应用失败: ' + e.message) }
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
    <a-button type="primary" size="small" :disabled="busy || status === 'running'" @click="start">
      <template #icon><PlayCircleOutlined /></template>
      启动
    </a-button>
    <a-button size="small" :disabled="busy || status !== 'running'" @click="reload">
      <template #icon><ReloadOutlined /></template>
      应用
    </a-button>
    <a-button danger size="small" :disabled="busy || status !== 'running'" @click="stop">
      <template #icon><PauseCircleOutlined /></template>
      停止
    </a-button>
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
</style>
