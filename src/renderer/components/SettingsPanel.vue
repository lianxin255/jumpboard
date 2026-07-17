<script setup lang="ts">
import { ref } from 'vue'
import { h } from 'vue'
import { Modal, message } from 'antdv-next'
import { DownloadOutlined, UploadOutlined, DeleteOutlined } from '@antdv-next/icons'
import { api } from '@renderer/api/ipc'
import { useHostsStore } from '@renderer/stores/hosts'
import { useMappingsStore } from '@renderer/stores/mappings'

const hosts = useHostsStore()
const mappings = useMappingsStore()
const fileInput = ref<HTMLInputElement | null>(null)

function wipeAll() {
  Modal.confirm({
    title: '确定清除所有痕迹？',
    content: '这将删除所有主机、映射、日志、历史配置。程序将需要重启。此操作不可撤销！',
    okType: 'danger',
    okText: '清除所有',
    cancelText: '取消',
    onOk: async () => {
      const r = await api.wipeAllData()
      if (!r.ok) {
        message.error('清除失败：' + (r.output ?? '未知错误'))
        return
      }
      message.info('已清除所有痕迹，请手动关闭并重新打开程序。')
    },
  })
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
  message.info(`已导出 ${data.hosts.length} 个主机、${data.mappings.length} 条映射`)
}

function triggerImport() {
  fileInput.value?.click()
}

async function doImport(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) { input.value = ''; return }
  Modal.confirm({
    title: '确认导入',
    content: '导入会覆盖当前所有配置，确定继续？',
    okType: 'danger',
    okText: '导入',
    cancelText: '取消',
    onOk: async () => {
      try {
        const text = await file.text()
        const payload = JSON.parse(text)
        const r = await api.importData(payload)
        if (!r.ok) {
          message.error('导入失败：' + (r.output ?? '未知错误'))
        } else {
          message.info(`已导入 ${r.hostsImported} 个主机、${r.mappingsImported} 条映射，请重启程序`)
          await hosts.load()
          if (hosts.currentHostId) await mappings.load(hosts.currentHostId)
        }
      } catch (e: any) {
        message.error('导入失败：' + e.message)
      } finally {
        input.value = ''
      }
    },
    onCancel: () => { input.value = '' },
  })
}
</script>

<template>
  <div class="settings-panel">
    <div class="section glass-card">
      <h3>数据管理</h3>
      <p class="desc">备份当前配置到文件，或从备份恢复。</p>
      <div class="actions">
        <a-button type="primary" @click="doExport">
          <template #icon><DownloadOutlined /></template>
          导出配置
        </a-button>
        <a-button @click="triggerImport">
          <template #icon><UploadOutlined /></template>
          导入配置
        </a-button>
        <input ref="fileInput" type="file" accept=".json" hidden @change="doImport" />
      </div>
    </div>

    <div class="section glass-card danger-zone">
      <h3>⚠️ 危险区域</h3>
      <p class="desc">一键清除所有数据（主机、映射、日志、历史）。用于彻底抹除痕迹。</p>
      <a-button danger :icon="h(DeleteOutlined)" @click="wipeAll">清除所有痕迹</a-button>
    </div>

    <div class="section glass-card">
      <h3>关于</h3>
      <p class="desc">Nginx 跳板机管理软件 v0.1.0</p>
      <p class="desc">绿色版 · 免安装 · 退出无痕</p>
    </div>
  </div>
</template>

<style scoped>
.settings-panel { padding: var(--space-4); display: flex; flex-direction: column; gap: var(--space-4); max-width: 600px; }
.section { padding: var(--space-4); }
.section h3 { font-size: var(--font-size-md); margin-bottom: var(--space-2); }
.desc { font-size: var(--font-size-sm); color: var(--text-secondary); margin-bottom: var(--space-3); line-height: 1.5; }
.actions { display: flex; align-items: center; gap: var(--space-2); }
.danger-zone { border-color: var(--color-danger); }
</style>
