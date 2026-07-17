<script setup lang="ts">
import { ref } from 'vue'
import { useHostsStore } from '@renderer/stores/hosts'

const store = useHostsStore()
const showForm = ref(false)
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
  testCmd: 'nginx -t'
})
const testing = ref(false)
const testResult = ref<{ ok: boolean; output: string } | null>(null)
const error = ref('')

function openNew() {
  editingId.value = undefined
  form.value = { name: '', type: 'remote', host: '', port: 22, username: 'root', password: '', nginxPath: '/usr/sbin/nginx', confPath: '/etc/nginx/nginx.conf', reloadCmd: 'nginx -s reload', testCmd: 'nginx -t' }
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
    testCmd: h.remoteConfig?.testCmd ?? 'nginx -t'
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
      passwordEncrypted: form.value.password ? 'PLAIN:' + form.value.password : null
    })
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
    localConfig: form.value.type === 'local' ? { nginxPath: '', confDir: '', logDir: '' } : null,
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
      testCmd: form.value.testCmd
    } : null
  }
  try {
    if (editingId.value) await store.update(editingId.value, input)
    else await store.create(input)
    showForm.value = false
  } catch (e: any) {
    error.value = e.message
  }
}

async function removeHost(id: string) {
  if (!confirm('确定删除这个主机？')) return
  await store.remove(id)
}
</script>

<template>
  <div class="host-manager">
    <div class="toolbar">
      <h3>主机管理</h3>
      <button class="btn btn-primary" @click="openNew">+ 添加主机</button>
    </div>
    <div class="host-list">
      <div v-for="h in store.hosts" :key="h.id" class="host-item glass-card">
        <div class="info">
          <div class="name">{{ h.name }} <span class="badge">{{ h.type }}</span></div>
          <div v-if="h.type === 'remote'" class="meta">{{ h.remoteConfig?.host }}:{{ h.remoteConfig?.port }} · {{ h.remoteConfig?.username }}</div>
          <div v-else class="meta">本机 nginx</div>
        </div>
        <div class="actions">
          <button class="btn btn-ghost" @click="store.setCurrent(h.id)">切换</button>
          <button class="btn btn-ghost" @click="openEdit(h)">编辑</button>
          <button class="btn btn-ghost" @click="removeHost(h.id)">删除</button>
        </div>
      </div>
    </div>

    <div v-if="showForm" class="form-overlay" @click.self="showForm = false">
      <div class="form-card glass-card">
        <h3>{{ editingId ? '编辑主机' : '添加主机' }}</h3>
        <label>名称<input v-model="form.name" class="glass-input" placeholder="如：生产网关" /></label>
        <label>类型
          <select v-model="form.type" class="glass-input">
            <option value="local">本机</option>
            <option value="remote">远程（SSH）</option>
          </select>
        </label>
        <template v-if="form.type === 'remote'">
          <label>主机地址<input v-model="form.host" class="glass-input" placeholder="192.168.1.100" /></label>
          <label>SSH 端口<input v-model.number="form.port" type="number" class="glass-input" /></label>
          <label>用户名<input v-model="form.username" class="glass-input" /></label>
          <label>密码（留空则不修改）<input v-model="form.password" type="password" class="glass-input" placeholder="******" /></label>
          <label>nginx 路径<input v-model="form.nginxPath" class="glass-input" /></label>
          <label>conf 路径<input v-model="form.confPath" class="glass-input" /></label>
          <label>reload 命令<input v-model="form.reloadCmd" class="glass-input" /></label>
          <label>test 命令<input v-model="form.testCmd" class="glass-input" /></label>
          <button class="btn btn-ghost" :disabled="testing" @click="testConn">{{ testing ? '测试中...' : '测试连接' }}</button>
          <div v-if="testResult" class="test-result" :class="{ ok: testResult.ok }">
            {{ testResult.ok ? '✓ 连接成功' : '✗ ' + testResult.output }}
          </div>
        </template>
        <div v-if="error" class="error">{{ error }}</div>
        <div class="form-actions">
          <button class="btn btn-ghost" @click="showForm = false">取消</button>
          <button class="btn btn-primary" @click="save">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.host-manager { display: flex; flex-direction: column; height: 100%; }
.toolbar { display: flex; align-items: center; justify-content: space-between; padding: var(--space-3) var(--space-4); border-bottom: 1px solid var(--border-subtle); }
h3 { font-size: var(--font-size-lg); }
.host-list { padding: var(--space-4); display: flex; flex-direction: column; gap: var(--space-2); }
.host-item { display: flex; align-items: center; gap: var(--space-3); padding: var(--space-3) var(--space-4); }
.info { flex: 1; }
.name { font-weight: 600; font-size: var(--font-size-md); }
.badge { font-size: var(--font-size-xs); background: var(--bg-hover); padding: 1px var(--space-1); border-radius: var(--radius-sm); color: var(--text-secondary); }
.meta { font-size: var(--font-size-sm); color: var(--text-secondary); margin-top: 2px; }
.actions { display: flex; gap: var(--space-1); }
.actions .btn { padding: var(--space-1) var(--space-2); font-size: var(--font-size-sm); }
.form-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.25); backdrop-filter: blur(var(--blur-sm)); display: flex; align-items: center; justify-content: center; z-index: 10; }
.form-card { padding: var(--space-6); width: 480px; max-width: 92vw; max-height: 88vh; overflow-y: auto; }
.form-card h3 { margin-bottom: var(--space-4); }
label { display: block; margin-bottom: var(--space-3); font-size: var(--font-size-sm); color: var(--text-secondary); }
label input, label select { margin-top: var(--space-1); }
.test-result { margin: var(--space-2) 0; padding: var(--space-2); border-radius: var(--radius-sm); font-size: var(--font-size-sm); background: var(--bg-hover); color: var(--color-danger); }
.test-result.ok { color: var(--color-success); }
.error { color: var(--color-danger); font-size: var(--font-size-sm); margin-bottom: var(--space-2); }
.form-actions { display: flex; justify-content: flex-end; gap: var(--space-2); margin-top: var(--space-4); }
</style>
