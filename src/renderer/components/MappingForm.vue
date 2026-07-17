<script setup lang="ts">
import { ref, computed } from 'vue'
import { api } from '@renderer/api/ipc'
import type { Template } from '@renderer/stores/templates'

const props = defineProps<{ hostId: string; mappingId?: string; template?: Template | null }>()
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
    extraDirectives: tpl?.extraDirectives ?? m.extraDirectives ?? ''
  }
}

const form = ref<FormState>(templateToForm(props.template))

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
      extraDirectives: m.extraDirectives ?? ''
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
      healthCheckEnabled: 0,
      healthCheckIntervalSec: 30,
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
    <div class="form-card">
      <h3>{{ mappingId ? '编辑映射' : (template ? '从模板新建' : '新建映射') }}</h3>
      <label>名称<input v-model="form.name" placeholder="如：MySQL 内库" /></label>
      <label>协议
        <select v-model="form.protocol">
          <option value="tcp">TCP</option>
          <option value="udp">UDP</option>
          <option value="http">HTTP</option>
          <option value="https">HTTPS</option>
        </select>
      </label>

      <!-- TCP/UDP: 监听端口 -->
      <label v-if="isStream">监听端口<input v-model.number="form.listenPort" type="number" placeholder="9000" /></label>

      <!-- HTTP: 监听端口 + 域名 + 路径 -->
      <label v-if="isHttp">监听端口<input v-model.number="form.listenPort" type="number" placeholder="80" /></label>
      <label v-if="isHttp || isHttps">域名 (server_name)<input v-model="form.serverNames" placeholder="app1.gw.local（可留空）" /></label>
      <label v-if="isHttp || isHttps">路径前缀 (location)<input v-model="form.listenPath" placeholder="/" /></label>

      <!-- HTTPS: 证书 + 跳转 -->
      <template v-if="isHttps">
        <label>证书路径 (ssl_certificate)<input v-model="form.tlsCertPath" placeholder="/data/cert.pem" /></label>
        <label>私钥路径 (ssl_certificate_key)<input v-model="form.tlsKeyPath" placeholder="/data/key.pem" /></label>
        <label class="checkbox"><input type="checkbox" v-model="form.redirectHttp" /> HTTP→HTTPS 自动跳转</label>
      </template>

      <label>目标主机<input v-model="form.upstreamHost" placeholder="192.168.1.10" /></label>
      <label>目标端口<input v-model.number="form.upstreamPort" type="number" placeholder="3306" /></label>
      <label>附加指令（高级）<textarea v-model="form.extraDirectives" rows="2" placeholder="proxy_timeout 1h;"></textarea></label>
      <div v-if="error" class="error">{{ error }}</div>
      <div class="actions">
        <button class="ghost" @click="emit('cancel')">取消</button>
        <button :disabled="saving" @click="save">保存</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.form-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.3); display: flex;
  align-items: center; justify-content: center; z-index: 10; }
.form-card { background: #fff; border-radius: 16px; padding: 24px; width: 480px; max-width: 92vw; max-height: 88vh; overflow-y: auto; }
h3 { margin: 0 0 16px; }
label { display: block; margin-bottom: 12px; font-size: 13px; color: #555; }
label.checkbox { display: flex; align-items: center; gap: 6px; }
input, select, textarea { display: block; width: 100%; margin-top: 4px; padding: 8px;
  border: 1px solid #ddd; border-radius: 8px; font-size: 14px; }
label.checkbox input { width: auto; margin: 0; }
.error { color: #ff3b30; font-size: 13px; margin-bottom: 8px; }
.actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
button { padding: 8px 16px; border: none; border-radius: 8px; background: #0A84FF; color: #fff; cursor: pointer; }
button.ghost { background: #eee; color: #333; }
button:disabled { opacity: 0.5; }
</style>
