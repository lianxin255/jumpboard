<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useHostsStore } from '@renderer/stores/hosts'
import { api } from '@renderer/api/ipc'
import { toast, confirmBox } from '@renderer/feedback'

const props = defineProps<{ engineStatus?: string }>()

const store = useHostsStore()
const showForm = ref(false)

// 本机 IP + 平台信息（本机卡片展示）
const localIP = ref<string>('')
const localOS = ref<string>('')
async function loadLocalInfo() {
  try {
    const ips = await api.getLocalIPs()
    localIP.value = ips[0]?.address ?? ''
  } catch { /* ignore */ }
  localOS.value = navigator.platform
}
onMounted(loadLocalInfo)

// 主机状态：只有默认主机对应活动引擎，用传入的 engineStatus 显示真实状态；其余主机无独立引擎，显示「—」
function hostStatus(h: any): string | null {
  if (h.isDefault !== 1) return null
  return props.engineStatus ?? 'unknown'
}
const editingId = ref<string | undefined>()
const form = ref({
  name: '',
  type: 'remote' as 'local' | 'remote',
  host: '',
  port: 22,
  username: 'root',
  password: '',
  nginxPath: '/usr/sbin/nginx',
  confPath: '/etc/nginx/nginx.conf',
  reloadCmd: 'nginx -s reload',
  testCmd: 'nginx -t',
  // 本地专用
  localNginxPath: '',
  confDir: '',
  logDir: '',
})
const testing = ref(false)
const testResult = ref<{ ok: boolean; output: string } | null>(null)
const error = ref('')

function openNew() {
  editingId.value = undefined
  form.value = { name: '', type: 'remote', host: '', port: 22, username: 'root', password: '', nginxPath: '/usr/sbin/nginx', confPath: '/etc/nginx/nginx.conf', reloadCmd: 'nginx -s reload', testCmd: 'nginx -t', localNginxPath: '', confDir: '', logDir: '' }
  testResult.value = null
  showForm.value = true
}

function openEdit(h: any) {
  editingId.value = h.id
  form.value = {
    name: h.name,
    type: h.type,
    host: h.remoteConfig?.host ?? '',
    port: h.remoteConfig?.port ?? 22,
    username: h.remoteConfig?.username ?? 'root',
    password: '',
    nginxPath: h.remoteConfig?.nginxPath ?? '/usr/sbin/nginx',
    confPath: h.remoteConfig?.confPath ?? '/etc/nginx/nginx.conf',
    reloadCmd: h.remoteConfig?.reloadCmd ?? 'nginx -s reload',
    testCmd: h.remoteConfig?.testCmd ?? 'nginx -t',
    localNginxPath: h.localConfig?.nginxPath ?? '',
    confDir: h.localConfig?.confDir ?? '',
    logDir: h.localConfig?.logDir ?? '',
  }
  testResult.value = null
  showForm.value = true
}

async function testConn() {
  testing.value = true
  testResult.value = null
  try {
    testResult.value = await store.testConnection({
      host: form.value.host,
      port: form.value.port,
      username: form.value.username,
      passwordEncrypted: form.value.password ? 'PLAIN:' + form.value.password : null,
    })
    toast(testResult.value.ok ? 'SSH 连接成功' : '连接失败: ' + testResult.value.output, testResult.value.ok ? 'ok' : 'err')
  } finally {
    testing.value = false
  }
}

async function save() {
  error.value = ''
  if (!form.value.name) { error.value = '请填写名称'; return }
  if (form.value.type === 'remote' && !form.value.host) { error.value = '请填写主机地址'; return }
  const input: any = {
    name: form.value.name,
    type: form.value.type,
    isDefault: store.hosts.length === 0 ? 1 : 0,
    localConfig: form.value.type === 'local' ? { nginxPath: form.value.localNginxPath, confDir: form.value.confDir, logDir: form.value.logDir } : null,
    remoteConfig: form.value.type === 'remote' ? {
      host: form.value.host,
      port: form.value.port,
      username: form.value.username,
      authType: 'password' as const,
      passwordEncrypted: form.value.password ? 'PLAIN:' + form.value.password : null,
      keyPath: null,
      nginxPath: form.value.nginxPath,
      confPath: form.value.confPath,
      reloadCmd: form.value.reloadCmd,
      testCmd: form.value.testCmd,
    } : null,
  }
  try {
    if (editingId.value) await store.update(editingId.value, input)
    else await store.create(input)
    showForm.value = false
    toast('主机已保存', 'ok')
  } catch (e: any) {
    error.value = e.message
    toast('保存失败: ' + e.message, 'err')
  }
}

async function removeHost(id: string, name: string) {
  const ok = await confirmBox({
    title: '删除主机',
    message: `确定要删除主机「${name}」吗？该主机下的所有映射、日志和历史将一并删除。`,
    btnText: '删除',
    kind: 'danger',
    impact: '此操作不可撤销，请确认后再继续。',
  })
  if (!ok) return
  await store.remove(id)
  toast('已删除', 'ok')
}

async function makeDefault(id: string, name: string) {
  await store.setDefault(id)
  toast(`已将「${name}」设为默认主机`, 'ok')
}
</script>

<template>
  <div class="host-view">
    <div class="page-hd">
      <div>
        <div class="page-tit">主机管理</div>
        <div class="page-sub">nginx 运行的机器 · 所有数据按主机隔离</div>
      </div>
      <button class="btn btn-accent" @click="openNew">+ 新增主机</button>
    </div>

    <div class="host-list">
      <div v-for="h in store.hosts" :key="h.id" class="host-item" :class="{ 'is-default': h.isDefault === 1 }">
        <div class="h-ico">
          <svg v-if="h.type === 'local'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="4" width="18" height="6" rx="1"/><rect x="3" y="14" width="18" height="6" rx="1"/></svg>
          <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
        </div>
        <div class="h-info">
          <div class="h-name">
            {{ h.name }}
            <span v-if="h.isDefault === 1" class="tag-default">默认</span>
          </div>
          <div class="h-meta">
            <template v-if="h.type === 'remote'">{{ h.remoteConfig?.host }} · {{ h.remoteConfig?.username }}</template>
            <template v-else>{{ localIP || '本机' }} · {{ localOS }}</template>
          </div>
        </div>
        <div class="h-nginx">
          <div class="h-ver">{{ h.type === 'local' ? '内置 nginx' : (h.remoteConfig?.nginxPath || 'nginx') }}</div>
          <div class="h-conf">{{ h.type === 'remote' ? 'ssh://' + (h.remoteConfig?.username || '') + '@' + (h.remoteConfig?.host || '') : '本地配置目录' }}</div>
        </div>
        <div class="h-status">
          <template v-if="hostStatus(h)">
            <span class="eng-dot" :class="hostStatus(h) === 'running' ? 'run' : hostStatus(h) === 'stopped' ? 'stop' : 'unk'"></span>
            <span>{{ hostStatus(h) === 'running' ? '运行中' : hostStatus(h) === 'stopped' ? '已停止' : '未知' }}</span>
          </template>
          <span v-else class="h-status-none">—</span>
        </div>
        <div class="row-acts">
          <button v-if="h.isDefault !== 1" class="icon-btn" title="设为默认" @click="makeDefault(h.id, h.name)">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><polygon points="12,2 15,9 22,9 17,14 19,21 12,17 5,21 7,14 2,9 9,9"/></svg>
          </button>
          <button v-if="h.type === 'remote'" class="icon-btn" title="测试连接" @click="testConn">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M5 12h14M13 6l6 6-6 6"/></svg>
          </button>
          <button class="icon-btn" title="编辑" @click="openEdit(h)">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M11 4H4v16h16v-7"/><path d="M18.5 2.5a2.1 2.1 0 0 1 3 3L12 15l-4 1 1-4z"/></svg>
          </button>
          <button class="icon-btn danger" title="删除" @click="removeHost(h.id, h.name)">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6"/></svg>
          </button>
        </div>
      </div>

      <div v-if="store.hosts.length === 0" class="empty-state" style="padding:60px">
        <div class="e-ico"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="4" width="18" height="6" rx="1"/><rect x="3" y="14" width="18" height="6" rx="1"/></svg></div>
        <h3>还没有任何主机</h3>
        <p>添加你的第一台 nginx 主机（本机或远程 SSH）。</p>
        <button class="btn btn-accent" @click="openNew">+ 新增主机</button>
      </div>
    </div>

    <!-- 主机表单浮层 -->
    <div v-if="showForm" class="overlay" @click.self="showForm = false">
      <div class="modal lg">
        <div class="modal-hd">
          <div>
            <h3>{{ editingId ? '编辑主机' : '新增主机' }}</h3>
            <div class="sub">本地 / 远程 SSH · 所有数据按主机隔离</div>
          </div>
          <button class="close-x" @click="showForm = false">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12"/></svg>
          </button>
        </div>
        <div class="modal-bd">
          <!-- 主机类型 -->
          <div class="field">
            <label>主机类型</label>
            <div class="proto-pick c2">
              <label>
                <input type="radio" name="htype" value="local" v-model="form.type">
                <div class="opt">
                  <svg class="o-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="4" width="18" height="6" rx="1"/><rect x="3" y="14" width="18" height="6" rx="1"/></svg>
                  <span class="o-nm">本地主机</span>
                </div>
              </label>
              <label>
                <input type="radio" name="htype" value="remote" v-model="form.type">
                <div class="opt">
                  <svg class="o-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21"/></svg>
                  <span class="o-nm">远程主机 (SSH)</span>
                </div>
              </label>
            </div>
          </div>

          <div class="field">
            <label>名称 <span class="req">*</span></label>
            <input class="input" v-model="form.name" placeholder="如：生产跳板 prod-jump-01">
          </div>

          <!-- 本地 -->
          <template v-if="form.type === 'local'">
            <div class="field">
              <label>nginx 路径</label>
              <input class="input mono" v-model="form.localNginxPath" placeholder="留空 = 使用内置 nginx">
            </div>
            <div class="field-row c2">
              <div class="field">
                <label>配置目录</label>
                <input class="input mono" v-model="form.confDir" placeholder="留空 = 默认目录">
              </div>
              <div class="field">
                <label>日志目录</label>
                <input class="input mono" v-model="form.logDir" placeholder="留空 = 默认目录">
              </div>
            </div>
          </template>

          <!-- 远程 -->
          <template v-else>
            <div class="form-sec-tit" style="margin-top:8px">连接信息</div>
            <div class="field-row c2">
              <div class="field">
                <label>SSH 地址 <span class="req">*</span></label>
                <input class="input mono" v-model="form.host" placeholder="10.0.20.15">
              </div>
              <div class="field">
                <label>SSH 端口</label>
                <input class="input mono" type="number" v-model="form.port">
              </div>
            </div>
            <div class="field">
              <label>用户名 <span class="req">*</span></label>
              <input class="input mono" v-model="form.username" placeholder="ops">
            </div>

            <div class="form-sec-tit">认证方式</div>
            <div class="field">
              <label>密码</label>
              <input class="input mono" type="password" v-model="form.password" placeholder="••••••••">
            </div>

            <div class="form-sec-tit" style="margin-top:8px">nginx 路径</div>
            <div class="field">
              <label>远程 nginx 路径</label>
              <input class="input mono" v-model="form.nginxPath" placeholder="/usr/sbin/nginx">
            </div>
            <div class="field">
              <label>远程配置文件路径</label>
              <input class="input mono" v-model="form.confPath" placeholder="/etc/nginx/nginx.conf">
            </div>
            <div class="field-row c2">
              <div class="field">
                <label>reload 命令</label>
                <input class="input mono" v-model="form.reloadCmd">
              </div>
              <div class="field">
                <label>test 命令</label>
                <input class="input mono" v-model="form.testCmd">
              </div>
            </div>

            <button class="btn btn-ghost btn-sm" :disabled="testing" style="margin-top:8px" @click="testConn">
              {{ testing ? '测试中…' : '测试 SSH 连接' }}
            </button>
          </template>

          <div v-if="error" class="alert-warn" style="margin-top:12px">{{ error }}</div>
        </div>

        <div class="modal-ft">
          <button class="btn btn-ghost" @click="showForm = false">取消</button>
          <button class="btn btn-accent" @click="save">保存主机</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.host-view { height: 100%; overflow: auto; }

.page-hd { display: flex; align-items: flex-end; justify-content: space-between; gap: 16px; padding: 22px var(--page-px) 16px; border-bottom: 1px solid var(--border); }
.page-tit { font-family: var(--font-display); font-size: 26px; font-weight: 500; }
.page-sub { font-size: 12px; color: var(--muted); margin-top: 4px; font-family: var(--font-mono); }

.host-list { padding: 6px var(--page-px) 28px; }

.host-item { display: grid; grid-template-columns: 40px 1.4fr 1.2fr 110px auto; gap: 16px; align-items: center; padding: 16px 18px; border: 1px solid var(--border); border-radius: var(--r); margin-bottom: 8px; background: var(--surface-3); transition: all var(--transition-fast); }
.host-item:hover { border-color: var(--muted); }
.host-item.is-default { background: rgba(201,100,66,.04); border-color: rgba(201,100,66,.4); }

.h-ico { width: 36px; height: 36px; border-radius: var(--r-sm); display: grid; place-items: center; background: var(--surface-2); border: 1px solid var(--border-soft); color: var(--proto-http); }
.h-ico svg { width: 18px; height: 18px; }
.h-name { font-size: 14px; font-weight: 500; color: var(--fg); display: flex; align-items: center; gap: 8px; }
.h-meta { font-family: var(--font-mono); font-size: 12px; color: var(--meta); margin-top: 2px; }
.h-nginx { font-family: var(--font-mono); font-size: 12px; color: var(--fg-2); }
.h-conf { color: var(--meta); font-size: 10.5px; margin-top: 2px; }
.h-status { font-family: var(--font-mono); font-size: 11px; display: flex; align-items: center; gap: 6px; }
.h-status .eng-dot { width: 7px; height: 7px; }
.h-status:has(.run) { color: var(--color-success); }
.h-status:has(.stop) { color: var(--color-danger); }
.h-status:has(.unk) { color: var(--meta); }
.h-status-none { color: var(--meta); }
.tag-default { font-family: var(--font-mono); font-size: 9.5px; color: var(--color-accent); border: 1px solid rgba(201,100,66,.4); padding: 1px 5px; border-radius: var(--r-sm); letter-spacing: 0.04em; }

.row-acts { display: flex; gap: 4px; justify-content: flex-end; }

/* 浮层（与 MappingForm 共享样式） */
.overlay { position: fixed; inset: 0; background: rgba(0,0,0,.6); backdrop-filter: blur(2px); z-index: 50; display: flex; align-items: center; justify-content: center; animation: fade .15s; }
.modal { background: var(--surface); border: 1px solid var(--border-soft); border-radius: var(--r-lg); box-shadow: var(--shadow-md); width: 92%; max-width: 720px; max-height: 88vh; display: flex; flex-direction: column; animation: fadeUp .2s var(--ease); }
.modal.lg { max-width: 860px; }
.modal-hd { display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; border-bottom: 1px solid var(--border); }
.modal-hd h3 { font-family: var(--font-display); font-size: 18px; font-weight: 500; }
.modal-hd .sub { font-family: var(--font-mono); font-size: 11px; color: var(--muted); margin-top: 2px; }
.close-x { width: 28px; height: 28px; display: grid; place-items: center; border-radius: var(--r-sm); color: var(--muted); background: none; border: none; cursor: pointer; }
.close-x:hover { background: var(--surface-2); color: var(--fg); }
.modal-bd { padding: 20px; overflow: auto; flex: 1; }
.modal-ft { display: flex; align-items: center; justify-content: flex-end; gap: 8px; padding: 14px 20px; border-top: 1px solid var(--border); }

.field { margin-bottom: 14px; }
.field > label { display: block; font-size: 12px; color: var(--fg-2); margin-bottom: 6px; font-weight: 500; }
.req { color: var(--color-accent); }
.input { width: 100%; padding: 8px 11px; background: var(--surface-3); border: 1px solid var(--border-soft); border-radius: var(--r); font-size: 13px; color: var(--fg); transition: border-color var(--transition-fast); outline: none; font-family: inherit; }
.input.mono { font-family: var(--font-mono); font-size: 12.5px; }
.input:focus { border-color: var(--color-accent); box-shadow: 0 0 0 2px rgba(201,100,66,.18); }
.input::placeholder { color: var(--meta); }
.field-row { display: grid; gap: 12px; }
.field-row.c2 { grid-template-columns: 1fr 1fr; }
.form-sec-tit { font-family: var(--font-mono); font-size: 10.5px; color: var(--meta); letter-spacing: 0.08em; text-transform: uppercase; margin-bottom: 12px; padding-bottom: 8px; border-bottom: 1px solid var(--border); }

/* radio card */
.proto-pick { display: grid; grid-template-columns: repeat(4, 1fr); gap: 6px; }
.proto-pick.c2 { grid-template-columns: 1fr 1fr; }
.proto-pick label { position: relative; cursor: pointer; display: block; }
.proto-pick input { position: absolute; opacity: 0; pointer-events: none; }
.proto-pick .opt { display: flex; flex-direction: column; align-items: center; gap: 5px; padding: 10px; border: 1px solid var(--border-soft); border-radius: var(--r); transition: all var(--transition-fast); background: var(--surface-3); color: var(--fg-2); }
.proto-pick input:checked + .opt { border-color: var(--color-accent); background: rgba(201,100,66,.07); color: var(--fg); }
.proto-pick .o-ico { width: 18px; height: 18px; }
.proto-pick input:checked + .opt .o-ico { color: var(--color-accent); }
.proto-pick .o-nm { font-family: var(--font-mono); font-size: 11px; font-weight: 600; }

.alert-warn { display: flex; align-items: center; gap: 9px; padding: 9px 12px; border: 1px solid rgba(201,162,58,.4); background: rgba(201,162,58,.08); border-radius: var(--r); font-size: 12px; color: var(--color-warning); }
</style>
