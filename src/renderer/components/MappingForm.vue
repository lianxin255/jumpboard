<script setup lang="ts">
import { ref } from 'vue'
import { api } from '@renderer/api/ipc'

const props = defineProps<{ hostId: string; mappingId?: string }>()
const emit = defineEmits<{ (e: 'saved'): void; (e: 'cancel'): void }>()

const form = ref({
  name: '',
  protocol: 'tcp' as 'tcp' | 'udp' | 'http' | 'https',
  listenPort: null as number | null,
  upstreamHost: '',
  upstreamPort: null as number | null,
  extraDirectives: 'proxy_timeout 1h;',
})

const saving = ref(false)
const error = ref('')

async function loadExisting() {
  if (!props.mappingId) return
  const list = await api.listMappings(props.hostId)
  const m = list.find((x: any) => x.id === props.mappingId)
  if (m) {
    form.value = {
      name: m.name,
      protocol: m.protocol,
      listenPort: m.listenPort,
      upstreamHost: m.upstreamHost,
      upstreamPort: m.upstreamPort,
      extraDirectives: m.extraDirectives ?? 'proxy_timeout 1h;',
    }
  }
}
loadExisting()

async function save() {
  error.value = ''
  if (!form.value.name || !form.value.upstreamHost || !form.value.listenPort || !form.value.upstreamPort) {
    error.value = '请填写名称、监听端口、目标地址、目标端口'
    return
  }
  saving.value = true
  try {
    const input = {
      hostId: props.hostId,
      name: form.value.name,
      enabled: 1,
      protocol: form.value.protocol,
      listenPort: form.value.listenPort,
      serverNames: null, listenPath: null,
      upstreamHost: form.value.upstreamHost,
      upstreamPort: form.value.upstreamPort,
      tlsCertPath: null, tlsKeyPath: null, redirectHttp: 0,
      extraDirectives: form.value.extraDirectives,
      healthCheckEnabled: 0, healthCheckIntervalSec: 30,
      healthCheckTargetPath: null, healthCheckExpectedStatus: null,
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
      <h3>{{ mappingId ? '编辑映射' : '新建映射（TCP）' }}</h3>
      <label>名称<input v-model="form.name" placeholder="如：MySQL 内库" /></label>
      <label>协议
        <select v-model="form.protocol">
          <option value="tcp">TCP</option>
        </select>
        <small>UDP/HTTP/HTTPS 将在后续版本支持</small>
      </label>
      <label>监听端口<input v-model.number="form.listenPort" type="number" placeholder="9000" /></label>
      <label>目标主机<input v-model="form.upstreamHost" placeholder="192.168.1.10" /></label>
      <label>目标端口<input v-model.number="form.upstreamPort" type="number" placeholder="3306" /></label>
      <label>附加指令（高级）<textarea v-model="form.extraDirectives" rows="2"></textarea></label>
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
  align-items: center; justify-content: center; }
.form-card { background: #fff; border-radius: 16px; padding: 24px; width: 420px; max-width: 90vw; }
h3 { margin: 0 0 16px; }
label { display: block; margin-bottom: 12px; font-size: 13px; color: #555; }
input, select, textarea { display: block; width: 100%; margin-top: 4px; padding: 8px;
  border: 1px solid #ddd; border-radius: 8px; font-size: 14px; }
small { color: #999; }
.error { color: #ff3b30; font-size: 13px; margin-bottom: 8px; }
.actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
button { padding: 8px 16px; border: none; border-radius: 8px; background: #0A84FF; color: #fff; cursor: pointer; }
button.ghost { background: #eee; color: #333; }
button:disabled { opacity: 0.5; }
</style>
