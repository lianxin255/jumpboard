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
      <a-button type="primary" @click="openNew">+ 添加主机</a-button>
    </div>
    <div class="host-list">
      <div v-for="h in store.hosts" :key="h.id" class="host-item glass-card">
        <div class="info">
          <div class="name">{{ h.name }} <span class="badge">{{ h.type }}</span></div>
          <div v-if="h.type === 'remote'" class="meta">{{ h.remoteConfig?.host }}:{{ h.remoteConfig?.port }} · {{ h.remoteConfig?.username }}</div>
          <div v-else class="meta">本机 nginx</div>
        </div>
        <div class="actions">
          <a-button size="small" @click="store.setCurrent(h.id)">切换</a-button>
          <a-button size="small" @click="openEdit(h)">编辑</a-button>
          <a-button size="small" danger @click="removeHost(h.id)">删除</a-button>
        </div>
      </div>
    </div>

    <a-modal
      v-model:open="showForm"
      :title="editingId ? '编辑主机' : '添加主机'"
      :footer="null"
      :width="520"
    >
      <div class="form-body">
        <div class="field">
          <div class="field-label">名称</div>
          <a-input v-model:value="form.name" placeholder="如：生产网关" />
        </div>
        <div class="field">
          <div class="field-label">类型</div>
          <a-select v-model:value="form.type" style="width: 100%">
            <a-select-option value="local">本机</a-select-option>
            <a-select-option value="remote">远程（SSH）</a-select-option>
          </a-select>
        </div>
        <template v-if="form.type === 'remote'">
          <div class="field">
            <div class="field-label">主机地址</div>
            <a-input v-model:value="form.host" placeholder="192.168.1.100" />
          </div>
          <div class="field">
            <div class="field-label">SSH 端口</div>
            <a-input-number v-model:value="form.port" style="width: 100%" />
          </div>
          <div class="field">
            <div class="field-label">用户名</div>
            <a-input v-model:value="form.username" />
          </div>
          <div class="field">
            <div class="field-label">密码（留空则不修改）</div>
            <a-input-password v-model:value="form.password" placeholder="******" />
          </div>
          <div class="field">
            <div class="field-label">nginx 路径</div>
            <a-input v-model:value="form.nginxPath" />
          </div>
          <div class="field">
            <div class="field-label">conf 路径</div>
            <a-input v-model:value="form.confPath" />
          </div>
          <div class="field">
            <div class="field-label">reload 命令</div>
            <a-input v-model:value="form.reloadCmd" />
          </div>
          <div class="field">
            <div class="field-label">test 命令</div>
            <a-input v-model:value="form.testCmd" />
          </div>
          <a-button :loading="testing" @click="testConn">{{ testing ? '测试中...' : '测试连接' }}</a-button>
          <a-alert
            v-if="testResult"
            :type="testResult.ok ? 'success' : 'error'"
            :message="testResult.ok ? '✓ 连接成功' : testResult.output"
            show-icon
            style="margin-top: var(--space-3)"
          />
        </template>
        <a-alert
          v-if="error"
          type="error"
          :message="error"
          show-icon
          style="margin-top: var(--space-3)"
        />
        <div class="form-actions">
          <a-button @click="showForm = false">取消</a-button>
          <a-button type="primary" @click="save">保存</a-button>
        </div>
      </div>
    </a-modal>
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

.form-body { display: flex; flex-direction: column; }
.field { display: block; margin-bottom: var(--space-3); }
.field-label { font-size: var(--font-size-sm); color: var(--text-secondary); margin-bottom: var(--space-1); }
.form-actions { display: flex; justify-content: flex-end; gap: var(--space-2); margin-top: var(--space-4); }
</style>
