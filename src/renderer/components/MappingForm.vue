<script setup lang="ts">
import { ref, computed } from 'vue'
import { api } from '@renderer/api/ipc'
import type { Template } from '@renderer/stores/templates'

// 注意：prop 名不能用 template（与 Vue SFC <template> 标签 + 保留字冲突，会导致模板内引用异常）
const props = defineProps<{ hostId: string; mappingId?: string; templateData?: Template | null }>()
const emit = defineEmits<{ (e: 'saved'): void; (e: 'cancel'): void }>()

interface FormState {
  name: string
  protocol: 'tcp' | 'udp' | 'http' | 'https'
  listenPort: number | null
  serverNames: string
  listenPath: string
  upstreamHost: string
  upstreamPort: number | null
  tlsCertPath: string
  tlsKeyPath: string
  redirectHttp: boolean
  extraDirectives: string
  healthCheckEnabled: boolean
  healthCheckIntervalSec: number
}

function templateToForm(tpl: Template | null | undefined): FormState {
  const m = tpl?.mapping ?? {}
  return {
    name: m.name ?? '',
    protocol: m.protocol ?? 'tcp',
    listenPort: m.listenPort ?? null,
    serverNames: m.serverNames ?? '',
    listenPath: m.listenPath ?? '/',
    upstreamHost: m.upstreamHost ?? '',
    upstreamPort: m.upstreamPort ?? null,
    tlsCertPath: m.tlsCertPath ?? '',
    tlsKeyPath: m.tlsKeyPath ?? '',
    redirectHttp: m.redirectHttp === 1,
    extraDirectives: tpl?.extraDirectives ?? m.extraDirectives ?? '',
    healthCheckEnabled: m.healthCheckEnabled === 1,
    healthCheckIntervalSec: m.healthCheckIntervalSec ?? 30,
  }
}

const form = ref<FormState>(templateToForm(props.templateData))

// 编辑模式：加载已有 mapping（覆盖模板预填）
async function loadExisting() {
  if (!props.mappingId) return
  const list = await api.listMappings(props.hostId)
  const m = list.find((x: any) => x.id === props.mappingId)
  if (m) {
    form.value = {
      name: m.name,
      protocol: m.protocol,
      listenPort: m.listenPort,
      serverNames: m.serverNames ?? '',
      listenPath: m.listenPath ?? '/',
      upstreamHost: m.upstreamHost,
      upstreamPort: m.upstreamPort,
      tlsCertPath: m.tlsCertPath ?? '',
      tlsKeyPath: m.tlsKeyPath ?? '',
      redirectHttp: m.redirectHttp === 1,
      extraDirectives: m.extraDirectives ?? '',
      healthCheckEnabled: m.healthCheckEnabled === 1,
      healthCheckIntervalSec: m.healthCheckIntervalSec ?? 30,
    }
  }
}
loadExisting()

// 按协议显示字段
const isStream = computed(() => form.value.protocol === 'tcp' || form.value.protocol === 'udp')
const isHttp = computed(() => form.value.protocol === 'http')
const isHttps = computed(() => form.value.protocol === 'https')

const saving = ref(false)
const error = ref('')

async function save() {
  error.value = ''
  if (!form.value.name) { error.value = '请填写名称'; return }
  if (!form.value.upstreamHost) { error.value = '请填写目标主机'; return }
  if (!form.value.upstreamPort) { error.value = '请填写目标端口'; return }
  if (isStream.value && !form.value.listenPort) { error.value = '请填写监听端口'; return }
  if (isHttps.value && (!form.value.tlsCertPath || !form.value.tlsKeyPath)) { error.value = 'HTTPS 需要证书和私钥路径'; return }

  // 端口占用检测（仅 stream/http 监听端口）
  if ((isStream.value || isHttp.value) && form.value.listenPort) {
    const available = await api.checkPort(form.value.listenPort)
    if (!available) {
      if (!confirm(`端口 ${form.value.listenPort} 已被占用，仍要保存吗？`)) return
    }
  }

  saving.value = true
  try {
    const f = form.value
    const input = {
      hostId: props.hostId,
      name: f.name,
      enabled: 1,
      protocol: f.protocol,
      listenPort: isStream.value ? f.listenPort : (isHttp.value ? f.listenPort : null),
      serverNames: (isHttp.value || isHttps.value) ? (f.serverNames || null) : null,
      listenPath: (isHttp.value || isHttps.value) ? f.listenPath : null,
      upstreamHost: f.upstreamHost,
      upstreamPort: f.upstreamPort,
      tlsCertPath: isHttps.value ? f.tlsCertPath : null,
      tlsKeyPath: isHttps.value ? f.tlsKeyPath : null,
      redirectHttp: isHttps.value && f.redirectHttp ? 1 : 0,
      extraDirectives: f.extraDirectives || null,
      healthCheckEnabled: f.healthCheckEnabled ? 1 : 0,
      healthCheckIntervalSec: f.healthCheckIntervalSec,
      healthCheckTargetPath: null,
      healthCheckExpectedStatus: null,
    }
    if (props.mappingId) await api.updateMapping(props.mappingId, input)
    else await api.createMapping(input)
    emit('saved')
  } catch (e: any) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="form-overlay" @click.self="emit('cancel')">
    <div class="form-card glass-card">
      <h3>{{ mappingId ? '编辑映射' : (templateData ? '从模板新建' : '新建映射') }}</h3>
      <label>名称<input v-model="form.name" class="glass-input" placeholder="如：MySQL 内库" /></label>
      <label>协议
        <select v-model="form.protocol" class="glass-input">
          <option value="tcp">TCP</option>
          <option value="udp">UDP</option>
          <option value="http">HTTP</option>
          <option value="https">HTTPS</option>
        </select>
      </label>

      <!-- TCP/UDP: 监听端口 -->
      <label v-if="isStream">监听端口<input v-model.number="form.listenPort" type="number" class="glass-input" placeholder="9000" /></label>

      <!-- HTTP: 监听端口 + 域名 + 路径 -->
      <label v-if="isHttp">监听端口<input v-model.number="form.listenPort" type="number" class="glass-input" placeholder="80" /></label>
      <label v-if="isHttp || isHttps">域名 (server_name)<input v-model="form.serverNames" class="glass-input" placeholder="app1.gw.local（可留空）" /></label>
      <label v-if="isHttp || isHttps">路径前缀 (location)<input v-model="form.listenPath" class="glass-input" placeholder="/" /></label>

      <!-- HTTPS: 证书 + 跳转 -->
      <template v-if="isHttps">
        <label>证书路径 (ssl_certificate)<input v-model="form.tlsCertPath" class="glass-input" placeholder="/data/cert.pem" /></label>
        <label>私钥路径 (ssl_certificate_key)<input v-model="form.tlsKeyPath" class="glass-input" placeholder="/data/key.pem" /></label>
        <label class="checkbox"><input type="checkbox" v-model="form.redirectHttp" /> HTTP→HTTPS 自动跳转</label>
      </template>

      <label>目标主机<input v-model="form.upstreamHost" class="glass-input" placeholder="192.168.1.10" /></label>
      <label>目标端口<input v-model.number="form.upstreamPort" type="number" class="glass-input" placeholder="3306" /></label>
      <label>附加指令（高级）<textarea v-model="form.extraDirectives" rows="2" class="glass-input" placeholder="proxy_timeout 1h;"></textarea></label>
      <label class="checkbox"><input type="checkbox" v-model="form.healthCheckEnabled" /> 启用健康检查</label>
      <label v-if="form.healthCheckEnabled">检查间隔（秒）<input v-model.number="form.healthCheckIntervalSec" type="number" min="5" class="glass-input" /></label>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="actions">
        <button class="btn btn-ghost" @click="emit('cancel')">取消</button>
        <button class="btn btn-primary" :disabled="saving" @click="save">保存</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.form-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.25);
  backdrop-filter: blur(var(--blur-sm));
  -webkit-backdrop-filter: blur(var(--blur-sm));
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
}

.form-card {
  padding: var(--space-6);
  width: 480px;
  max-width: 92vw;
  max-height: 88vh;
  overflow-y: auto;
}

h3 {
  margin-bottom: var(--space-4);
  font-size: var(--font-size-lg);
}

label {
  display: block;
  margin-bottom: var(--space-3);
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

label.checkbox {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  color: var(--text-primary);
  font-size: var(--font-size-base);
}

label.checkbox input {
  width: auto;
  margin: 0;
  accent-color: var(--color-accent);
}

/* glass-input provides the input styling globally; here only margin-top spacing */
input.glass-input, select.glass-input, textarea.glass-input {
  margin-top: var(--space-1);
}

textarea.glass-input {
  resize: vertical;
  min-height: 48px;
  font-family: var(--font-mono);
  font-size: var(--font-size-sm);
}

.error {
  color: var(--color-danger);
  font-size: var(--font-size-sm);
  margin-bottom: var(--space-2);
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  margin-top: var(--space-4);
}
</style>
