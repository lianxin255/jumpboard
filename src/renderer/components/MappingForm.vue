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
  if (!form.value.name) { error.value = '请填写规则名称'; return }
  if (!form.value.upstreamHost) { error.value = '请填写内网服务 IP 地址'; return }
  if (!form.value.upstreamPort) { error.value = '请填写内网服务端口'; return }
  if (isStream.value && !form.value.listenPort) { error.value = '请填写对外开放端口'; return }
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
  <a-modal
    :open="true"
    :title="mappingId ? '编辑映射' : (templateData ? '从模板新建' : '新建映射')"
    :footer="null"
    :width="520"
    :mask-closable="true"
    @cancel="emit('cancel')"
  >
    <div class="form-body">
      <a-alert
        type="info"
        show-icon
        banner
        message="把内网服务通过跳板机暴露给外部访问"
        description="填写内网服务的 IP 和端口，选择对外开发的端口，外部就能通过跳板机访问内网服务。"
        style="margin-bottom: var(--space-3)"
      />
      <div class="field">
        <div class="field-label">名称</div>
        <a-input v-model:value="form.name" placeholder="如：内网 MySQL、研发服务器 SSH" />
      </div>
      <div class="field">
        <div class="field-label">协议</div>
        <a-select v-model:value="form.protocol" style="width: 100%">
          <a-select-option value="tcp">TCP（端口转发）</a-select-option>
          <a-select-option value="udp">UDP（DNS/游戏等）</a-select-option>
          <a-select-option value="http">HTTP（网站代理）</a-select-option>
          <a-select-option value="https">HTTPS（加密网站代理）</a-select-option>
        </a-select>
      </div>

      <!-- TCP/UDP: 监听端口 -->
      <div v-if="isStream" class="field">
        <div class="field-label">对外开放端口</div>
        <a-input-number v-model:value="form.listenPort" style="width: 100%" placeholder="外部访问用端口，如 9000" />
        <div class="field-help">跳板机上供外部访问的端口（如 9000）</div>
      </div>

      <!-- HTTP: 监听端口 + 域名 + 路径 -->
      <div v-if="isHttp" class="field">
        <div class="field-label">对外开放端口</div>
        <a-input-number v-model:value="form.listenPort" style="width: 100%" placeholder="外部访问用端口，如 9000" />
        <div class="field-help">跳板机上供外部访问的端口（如 9000）</div>
      </div>
      <div v-if="isHttp || isHttps" class="field">
        <div class="field-label">访问域名（可选）</div>
        <a-input v-model:value="form.serverNames" placeholder="如 app.example.com（可留空）" />
        <div class="field-help">通过此域名访问时生效，留空则匹配所有</div>
      </div>
      <div v-if="isHttp || isHttps" class="field">
        <div class="field-label">URL 路径（可选）</div>
        <a-input v-model:value="form.listenPath" placeholder="如 /api（默认 / 全部转发）" />
        <div class="field-help">只有访问此路径前缀时才转发</div>
      </div>

      <!-- HTTPS: 证书 + 跳转 -->
      <template v-if="isHttps">
        <div class="field">
          <div class="field-label">SSL 证书文件路径</div>
          <a-input v-model:value="form.tlsCertPath" placeholder="如 C:/certs/cert.pem" />
        </div>
        <div class="field">
          <div class="field-label">SSL 私钥文件路径</div>
          <a-input v-model:value="form.tlsKeyPath" placeholder="如 C:/certs/key.pem" />
        </div>
        <div class="switch-row">
          <span>HTTP→HTTPS 自动跳转</span>
          <a-switch v-model:checked="form.redirectHttp" />
        </div>
      </template>

      <div class="field">
        <div class="field-label">内网服务 IP 地址</div>
        <a-input v-model:value="form.upstreamHost" placeholder="如 192.168.1.10" />
        <div class="field-help">要转发的内网服务器的 IP</div>
      </div>
      <div class="field">
        <div class="field-label">内网服务端口</div>
        <a-input-number v-model:value="form.upstreamPort" style="width: 100%" placeholder="如 3306（MySQL）、22（SSH）、80（Web）" />
        <div class="field-help">内网服务实际运行的端口</div>
      </div>
      <div class="field">
        <div class="field-label">高级配置（可选）</div>
        <a-textarea
          v-model:value="form.extraDirectives"
          :rows="2"
          placeholder="# 一般不需要填
# 示例：proxy_timeout 1h;"
        />
        <div class="field-help">懂 nginx 的用户可以手写额外配置指令</div>
      </div>
      <div class="switch-row">
        <span>定期检测内网服务是否在线</span>
        <a-switch v-model:checked="form.healthCheckEnabled" />
      </div>
      <div v-if="form.healthCheckEnabled" class="field">
        <div class="field-label">检测间隔（秒）</div>
        <a-input-number v-model:value="form.healthCheckIntervalSec" :min="5" style="width: 100%" />
      </div>
      <a-alert
        v-if="error"
        type="error"
        :message="error"
        show-icon
        style="margin-bottom: var(--space-3)"
      />
      <div class="actions">
        <a-button @click="emit('cancel')">取消</a-button>
        <a-button type="primary" :loading="saving" @click="save">保存</a-button>
      </div>
    </div>
  </a-modal>
</template>

<style scoped>
.form-body {
  display: flex;
  flex-direction: column;
}

.field {
  display: block;
  margin-bottom: var(--space-3);
}

.field-label {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin-bottom: var(--space-1);
}

.field-help {
  font-size: var(--font-size-xs, 12px);
  color: var(--text-tertiary);
  margin-top: var(--space-1);
  line-height: 1.4;
}

.switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-3);
  font-size: var(--font-size-base);
  color: var(--text-primary);
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  margin-top: var(--space-2);
}
</style>
