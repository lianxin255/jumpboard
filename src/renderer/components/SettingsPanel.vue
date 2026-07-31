<script setup lang="ts">
import { ref } from 'vue'
import { api } from '@renderer/api/ipc'
import { useHostsStore } from '@renderer/stores/hosts'
import { useMappingsStore } from '@renderer/stores/mappings'
import { toast, confirmBox } from '@renderer/feedback'

const hosts = useHostsStore()
const mappings = useMappingsStore()
const fileInput = ref<HTMLInputElement | null>(null)

async function wipeAll() {
  const ok = await confirmBox({
    title: '清空所有数据',
    message: '将删除全部主机、映射规则、配置历史和日志。',
    btnText: '清空',
    kind: 'danger',
    impact: '此操作不可恢复，建议先导出备份。',
  })
  if (!ok) return
  const r = await api.wipeAllData()
  if (!r.ok) { toast('清除失败：' + (r.output ?? '未知错误'), 'err'); return }
  toast('所有数据已清空，请重启程序', 'ok')
}

async function doExport() {
  try {
    const data = await api.exportData()
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `nginx-jump-backup-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    toast(`已导出 ${data.hosts.length} 主机 / ${data.mappings.length} 映射 / ${data.snapshots?.length ?? 0} 快照`, 'ok')
  } catch (e: any) {
    toast('导出失败: ' + e.message, 'err')
  }
}

function triggerImport() { fileInput.value?.click() }

async function doImport(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) { input.value = ''; return }
  const ok = await confirmBox({
    title: '确认导入',
    message: '从导出的 JSON 文件恢复数据。同名主机的映射将被合并，不会重复创建。',
    btnText: '导入',
    kind: 'ok',
  })
  if (!ok) { input.value = ''; return }
  try {
    const text = await file.text()
    const payload = JSON.parse(text)
    const r = await api.importData(payload)
    if (!r.ok) {
      toast('导入失败：' + (r.output ?? '未知错误'), 'err')
    } else {
      toast(`已导入 ${r.hostsImported} 主机 / ${r.mappingsImported} 映射 / ${r.snapshotsImported} 快照`, 'ok')
      await hosts.load()
      if (hosts.currentHostId) await mappings.load(hosts.currentHostId)
    }
  } catch (e: any) {
    toast('导入失败: ' + e.message, 'err')
  } finally {
    input.value = ''
  }
}
</script>

<template>
  <div class="settings-view">
    <div class="page-hd">
      <div>
        <div class="page-tit">设置</div>
        <div class="page-sub">数据管理 · 关于</div>
      </div>
    </div>

    <div class="content">
      <!-- 数据管理 -->
      <div class="section">
        <div class="sec-tit sec-head">数据管理</div>
        <div class="row">
          <div class="row-info">
            <h4>导出全部数据</h4>
            <p>将所有主机、映射规则、配置历史导出为一个 JSON 文件，便于迁移或备份。</p>
          </div>
          <button class="btn btn-sand" @click="doExport">导出</button>
        </div>
        <div class="row">
          <div class="row-info">
            <h4>导入数据</h4>
            <p>从导出的 JSON 文件恢复数据。同名主机的映射将被合并，不会重复创建。</p>
          </div>
          <button class="btn btn-sand" @click="triggerImport">导入</button>
          <input ref="fileInput" type="file" accept=".json" hidden @change="doImport">
        </div>
      </div>

      <!-- 关于 -->
      <div class="section">
        <div class="sec-tit sec-head">关于</div>
        <div class="about-grid">
          <div class="about-cell"><span class="k">软件名称</span><span class="v">跳板 Jumpbox</span></div>
          <div class="about-cell"><span class="k">版本</span><span class="v">0.1.0</span></div>
          <div class="about-cell"><span class="k">内置 nginx</span><span class="v">1.26.2</span></div>
          <div class="about-cell"><span class="k">数据存储</span><span class="v">SQLite · 本地</span></div>
          <div class="about-cell"><span class="k">平台</span><span class="v">Windows / macOS</span></div>
          <div class="about-cell"><span class="k">许可</span><span class="v">MIT</span></div>
        </div>
      </div>

      <!-- 危险操作区 -->
      <div class="danger-zone">
        <div class="sec-tit sec-head danger">危险操作区</div>
        <div class="row">
          <div class="row-info">
            <h4 class="danger-h">清空所有数据</h4>
            <p>删除全部主机、映射规则、配置历史和日志，<b class="danger-h">不可恢复</b>。建议先导出备份。</p>
          </div>
          <button class="btn btn-danger" @click="wipeAll">清空所有数据</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-view { height: 100%; overflow: auto; }
.page-hd { padding: 22px var(--page-px) 16px; border-bottom: 1px solid var(--border); }
.page-tit { font-family: var(--font-display); font-size: 26px; font-weight: 500; }
.page-sub { font-size: 12px; color: var(--muted); margin-top: 4px; font-family: var(--font-mono); }

.content { padding: 6px var(--page-px) 40px; max-width: 760px; }
.section { margin-bottom: 28px; }
.sec-head { padding: 16px 0 10px; border-bottom: 1px solid var(--border); }

.row { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 14px 0; border-bottom: 1px solid var(--border); }
.row-info h4 { font-size: 13.5px; font-weight: 500; color: var(--fg); }
.row-info p { font-size: 12px; color: var(--muted); margin-top: 3px; max-width: 440px; line-height: 1.5; }

.about-grid { display: grid; grid-template-columns: 1fr 1fr; padding: 14px 0; }
.about-cell { display: flex; justify-content: space-between; padding: 8px 16px 8px 0; border-bottom: 1px solid var(--border); font-size: 13px; }
.about-cell:nth-child(odd) { padding-right: 16px; }
.about-cell:nth-child(even) { padding-left: 16px; border-left: none; }
.about-cell .k { color: var(--muted); font-family: var(--font-mono); font-size: 11.5px; }
.about-cell .v { color: var(--fg); font-family: var(--font-mono); font-size: 12px; }

.danger-zone { border: 1px solid rgba(181,51,51,.4); border-radius: var(--r); padding: 4px 18px; background: rgba(181,51,51,.03); margin-top: 8px; }
.sec-tit.danger { color: var(--color-danger); }
.danger-h { color: var(--color-danger); }
</style>
