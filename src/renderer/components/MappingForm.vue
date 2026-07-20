<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import { type FormInstance, type Rule } from 'antdv-next'
import { api } from '@renderer/api/ipc'
import type { Template } from '@renderer/stores/templates'

// 注意：prop 名不能用 template（与 Vue SFC <template> 标签 + 保留字冲突，会导致模板内引用异常）
const props = defineProps<{ hostId: string; mappingId?: string; templateData?: Template | null }>()
const emit = defineEmits<{ (e: 'saved'): void; (e: 'cancel'): void }>()

// 本机 IP 列表（供"填入本机 IP"下拉）
const localIPs = ref<{ interface: string; address: string; family: string }[]>([])
;(async () => {
  try { localIPs.value = await api.getLocalIPs() } catch { /* IPC 未就绪 */ }
})()

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

const formRef = ref<FormInstance>()
const form = reactive<FormState>(templateToForm(props.templateData))

// 编辑模式：加载已有 mapping（覆盖模板预填）
async function loadExisting() {
  if (!props.mappingId) return
  const list = await api.listMappings(props.hostId)
  const m = list.find((x: any) => x.id === props.mappingId)
  if (m) {
    Object.assign(form, {
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
    })
  }
}
loadExisting()

// 按协议显示字段
const isStream = computed(() => form.protocol === 'tcp' || form.protocol === 'udp')
const isHttp = computed(() => form.protocol === 'http')
const isHttps = computed(() => form.protocol === 'https')

// 校验规则：用 computed 让 listenPort/tls 等条件规则随协议变化
const rules = computed<Record<string, Rule[]>>(() => ({
  name: [{ required: true, message: '请填写规则名称' }],
  upstreamHost: [{ required: true, message: '请填写内网服务 IP 地址' }],
  upstreamPort: [{ required: true, message: '请填写内网服务端口', type: 'number' as const }],
  listenPort: isStream.value || isHttp.value
    ? [{ required: true, message: '请填写对外开放端口', type: 'number' as const }]
    : [],
  tlsCertPath: isHttps.value ? [{ required: true, message: 'HTTPS 需要 SSL 证书路径' }] : [],
  tlsKeyPath: isHttps.value ? [{ required: true, message: 'HTTPS 需要 SSL 私钥路径' }] : [],
}))

const saving = ref(false)
// 仅用于后端/网络错误（字段校验交给 antd Form）
const backendError = ref('')

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    // 校验失败，antd Form 自动在对应字段下显示错误
    return
  }

  // 端口占用检测（仅 stream/http 监听端口）
  if ((isStream.value || isHttp.value) && form.listenPort) {
    const available = await api.checkPort(form.listenPort)
    if (!available) {
      if (!confirm(`端口 ${form.listenPort} 已被占用，仍要保存吗？`)) return
    }
  }

  saving.value = true
  try {
    const f = form
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
    // 后端错误用 a-alert 显示（字段校验已交给 antd Form）
    backendError.value = e.message
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
      <a-form
        ref="formRef"
        :model="form"
        :rules="rules"
        layout="vertical"
        :validate-messages="{ default: '字段校验失败：${label}' }"
      >
        <a-form-item label="名称" name="name">
          <a-input v-model:value="form.name" placeholder="如：内网 MySQL、研发服务器 SSH" />
        </a-form-item>
        <a-form-item label="协议" name="protocol">
          <a-select v-model:value="form.protocol" style="width: 100%">
            <a-select-option value="tcp">TCP（端口转发）</a-select-option>
            <a-select-option value="udp">UDP（DNS/游戏等）</a-select-option>
            <a-select-option value="http">HTTP（网站代理）</a-select-option>
            <a-select-option value="https">HTTPS（加密网站代理）</a-select-option>
          </a-select>
        </a-form-item>

        <!-- TCP/UDP: 监听端口 -->
        <a-form-item
          v-if="isStream"
          label="对外开放端口"
          name="listenPort"
        >
          <a-input-number v-model:value="form.listenPort" style="width: 100%" placeholder="外部访问用端口，如 9000" />
          <template #extra>跳板机上供外部访问的端口（如 9000）</template>
        </a-form-item>

        <!-- HTTP: 监听端口 + 域名 + 路径 -->
        <a-form-item
          v-if="isHttp"
          label="对外开放端口"
          name="listenPort"
        >
          <a-input-number v-model:value="form.listenPort" style="width: 100%" placeholder="外部访问用端口，如 9000" />
          <template #extra>跳板机上供外部访问的端口（如 9000）</template>
        </a-form-item>
        <a-form-item v-if="isHttp || isHttps" label="访问域名（可选）" name="serverNames">
          <a-input v-model:value="form.serverNames" placeholder="如 app.example.com（可留空）" />
          <template #extra>通过此域名访问时生效，留空则匹配所有</template>
        </a-form-item>
        <a-form-item v-if="isHttp || isHttps" label="URL 路径（可选）" name="listenPath">
          <a-input v-model:value="form.listenPath" placeholder="如 /api（默认 / 全部转发）" />
          <template #extra>只有访问此路径前缀时才转发</template>
        </a-form-item>

        <!-- HTTPS: 证书 + 跳转 -->
        <template v-if="isHttps">
          <a-form-item label="SSL 证书文件路径" name="tlsCertPath">
            <a-input v-model:value="form.tlsCertPath" placeholder="如 C:/certs/cert.pem" />
          </a-form-item>
          <a-form-item label="SSL 私钥文件路径" name="tlsKeyPath">
            <a-input v-model:value="form.tlsKeyPath" placeholder="如 C:/certs/key.pem" />
          </a-form-item>
          <div class="switch-row">
            <span>HTTP→HTTPS 自动跳转</span>
            <a-switch v-model:checked="form.redirectHttp" />
          </div>
        </template>

        <a-form-item label="内网服务 IP 地址" name="upstreamHost">
          <a-input-group compact style="display: flex">
            <a-input v-model:value="form.upstreamHost" placeholder="如 192.168.1.10" style="flex: 1" />
            <a-popover trigger="click" placement="bottomRight">
              <template #content>
                <div class="ip-picker">
                  <div v-for="ip in localIPs" :key="ip.address" class="ip-item"
                    @click="form.upstreamHost = ip.address">
                    {{ ip.address }} <span class="ip-iface">({{ ip.interface }})</span>
                  </div>
                  <div class="ip-item ip-loopback" @click="form.upstreamHost = '127.0.0.1'">
                    127.0.0.1 (本机回环)
                  </div>
                  <div v-if="localIPs.length === 0" class="ip-empty">获取本机 IP 失败</div>
                </div>
              </template>
              <a-button>本机 IP</a-button>
            </a-popover>
          </a-input-group>
          <template #extra>要转发的内网服务器的 IP，可点击"本机 IP"自动填入</template>
        </a-form-item>
        <a-form-item label="内网服务端口" name="upstreamPort">
          <a-input-number v-model:value="form.upstreamPort" style="width: 100%" placeholder="如 3306（MySQL）、22（SSH）、80（Web）" />
          <template #extra>内网服务实际运行的端口</template>
        </a-form-item>
        <a-form-item label="高级配置（可选）" name="extraDirectives">
          <a-textarea
            v-model:value="form.extraDirectives"
            :rows="2"
            placeholder="# 一般不需要填
# 示例：proxy_timeout 1h;"
          />
          <template #extra>懂 nginx 的用户可以手写额外配置指令</template>
        </a-form-item>
        <div class="switch-row">
          <span>定期检测内网服务是否在线</span>
          <a-switch v-model:checked="form.healthCheckEnabled" />
        </div>
        <a-form-item v-if="form.healthCheckEnabled" label="检测间隔（秒）" name="healthCheckIntervalSec">
          <a-input-number v-model:value="form.healthCheckIntervalSec" :min="5" style="width: 100%" />
        </a-form-item>
        <a-alert
          v-if="backendError"
          type="error"
          :message="backendError"
          show-icon
          style="margin-bottom: var(--space-3)"
        />
        <div class="actions">
          <a-button @click="emit('cancel')">取消</a-button>
          <a-button type="primary" :loading="saving" @click="save">保存</a-button>
        </div>
      </a-form>
    </div>
  </a-modal>
</template>

<style scoped>
.form-body {
  display: flex;
  flex-direction: column;
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

.ip-picker { max-width: 280px; }
.ip-item {
  padding: 4px 8px; cursor: pointer; border-radius: 4px;
  transition: background var(--transition-fast);
}
.ip-item:hover { background: rgba(0, 0, 0, 0.06); }
.ip-iface { color: #999; font-size: 12px; }
.ip-loopback { border-top: 1px solid #eee; margin-top: 4px; }
.ip-empty { padding: 8px; color: #999; }
</style>
