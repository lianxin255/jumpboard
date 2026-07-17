<script setup lang="ts">
import { ref } from 'vue'
import { api } from '@renderer/api/ipc'
import { useHostsStore } from '@renderer/stores/hosts'
import { useMappingsStore } from '@renderer/stores/mappings'

const hosts = useHostsStore()
const mappings = useMappingsStore()
const message = ref('')
const messageType = ref<'info' | 'error'>('info')

async function wipeAll() {
  if (!confirm('⚠️ 确定清除所有痕迹？\n\n这将删除所有主机、映射、日志、历史配置。\n程序将需要重启。\n\n此操作不可撤销！')) return
  if (!confirm('再次确认：真的要删除所有数据吗？')) return
  const r = await api.wipeAllData()
  if (!r.ok) {
    message.value = '清除失败：' + (r.output ?? '未知错误')
    messageType.value = 'error'
    return
  }
  message.value = '已清除所有痕迹，请手动关闭并重新打开程序。'
  messageType.value = 'info'
}

async function doExport() {
  const data = await api.exportData()
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `nginx-jump-backup-${new Date().toISOString().slice(0, 10)}.json`
  a.click()
  URL.revokeObjectURL(url)
  message.value = `已导出 ${data.hosts.length} 个主机、${data.mappings.length} 条映射`
  messageType.value = 'info'
}

async function doImport(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!confirm('导入会覆盖当前所有配置，确定继续？')) { input.value = ''; return }
  try {
    const text = await file.text()
    const payload = JSON.parse(text)
    const r = await api.importData(payload)
    if (!r.ok) {
      message.value = '导入失败：' + (r.output ?? '未知错误')
      messageType.value = 'error'
    } else {
      message.value = `已导入 ${r.hostsImported} 个主机、${r.mappingsImported} 条映射，请重启程序`
      messageType.value = 'info'
      await hosts.load()
      if (hosts.currentHostId) await mappings.load(hosts.currentHostId)
    }
  } catch (e: any) {
    message.value = '导入失败：' + e.message
    messageType.value = 'error'
  }
  input.value = ''
}
</script>

<template>
  <div class="settings-panel">
    <div class="section glass-card">
      <h3>数据管理</h3>
      <p class="desc">备份当前配置到文件，或从备份恢复。</p>
      <div class="actions">
        <button class="btn btn-primary" @click="doExport">导出配置</button>
        <label class="btn btn-ghost import-label">
          导入配置
          <input type="file" accept=".json" @change="doImport" hidden />
        </label>
      </div>
    </div>

    <div class="section glass-card danger-zone">
      <h3>⚠️ 危险区域</h3>
      <p class="desc">一键清除所有数据（主机、映射、日志、历史）。用于彻底抹除痕迹。</p>
      <button class="btn btn-danger" @click="wipeAll">清除所有痕迹</button>
    </div>

    <div class="section glass-card">
      <h3>关于</h3>
      <p class="desc">Nginx 跳板机管理软件 v0.1.0</p>
      <p class="desc">绿色版 · 免安装 · 退出无痕</p>
    </div>

    <div v-if="message" class="message" :class="messageType">{{ message }}</div>
  </div>
</template>

<style scoped>
.settings-panel { padding: var(--space-4); display: flex; flex-direction: column; gap: var(--space-4); max-width: 600px; }
.section { padding: var(--space-4); }
.section h3 { font-size: var(--font-size-md); margin-bottom: var(--space-2); }
.desc { font-size: var(--font-size-sm); color: var(--text-secondary); margin-bottom: var(--space-3); line-height: 1.5; }
.actions { display: flex; gap: var(--space-2); }
.import-label { cursor: pointer; display: inline-block; }
.btn-danger { background: var(--color-danger); color: #fff; padding: var(--space-2) var(--space-4); border: none; border-radius: var(--radius-sm); cursor: pointer; font-size: var(--font-size-base); }
.btn-danger:hover { opacity: 0.85; }
.danger-zone { border-color: var(--color-danger); }
.message { padding: var(--space-3); border-radius: var(--radius-md); font-size: var(--font-size-sm); }
.message.info { background: var(--color-accent-soft); color: var(--text-primary); }
.message.error { background: rgba(255, 59, 48, 0.12); color: var(--color-danger); }
</style>
