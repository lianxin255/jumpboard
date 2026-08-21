<script setup lang="ts">
import type { Template } from '@renderer/stores/templates'
import { api } from '@renderer/api/ipc'
import ModalShell from '@renderer/components/ModalShell.vue'
import ToggleSwitch from '@renderer/components/ToggleSwitch.vue'
import { confirmBox, promptBox, toast } from '@renderer/feedback'
import { formToTemplateMapping } from '@renderer/utils/templateMapping'
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'

// 注意：prop 名不能用 template（与 Vue SFC <template> 标签 + 保留字冲突）
const props = defineProps<{ hostId: string, mappingId?: string, templateData?: Template | null }>()
const emit = defineEmits<{ (e: 'saved'): void, (e: 'cancel'): void }>()

// 本机 IP 列表（供"填入本机 IP"下拉）
const localIPs = ref<{ interface: string, address: string, family: string }[]>([])
const ipPopoverOpen = ref(false)
;(async () => {
  try { localIPs.value = await api.getLocalIPs() }
  catch { /* IPC 未就绪 */ }
})()

interface FormState {
  name: string
  protocol: 'tcp' | 'udp' | 'http' | 'https'
  listenPort: number | null
  serverNames: string
  listenPath: string
  upstreamHost: string
  upstreamPort: number | null
  proxyPassUrl: string
  tlsCertPath: string
  tlsKeyPath: string
  redirectHttp: boolean
  extraDirectives: string
  proxyMode: 'standard' | 'minimal' | 'custom'
  proxyHeaders: string
  healthCheckEnabled: boolean
  healthCheckIntervalSec: number
  healthCheckTargetPath: string
  healthCheckExpectedStatus: number | null
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
    proxyPassUrl: m.proxyPassUrl ?? '',
    tlsCertPath: m.tlsCertPath ?? '',
    tlsKeyPath: m.tlsKeyPath ?? '',
    redirectHttp: m.redirectHttp === 1,
    extraDirectives: tpl?.extraDirectives ?? m.extraDirectives ?? '',
    proxyMode: m.proxyMode ?? 'standard',
    proxyHeaders: m.proxyHeaders ?? '',
    healthCheckEnabled: m.healthCheckEnabled === 1,
    healthCheckIntervalSec: m.healthCheckIntervalSec ?? 30,
    healthCheckTargetPath: m.healthCheckTargetPath ?? '/health',
    healthCheckExpectedStatus: m.healthCheckExpectedStatus ?? 200,
  }
}

const form = reactive<FormState>(templateToForm(props.templateData))

// 编辑模式：加载已有 mapping（覆盖模板预填）
async function loadExisting() {
  if (!props.mappingId)
    return
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
      proxyPassUrl: m.proxyPassUrl ?? '',
      tlsCertPath: m.tlsCertPath ?? '',
      tlsKeyPath: m.tlsKeyPath ?? '',
      redirectHttp: m.redirectHttp === 1,
      extraDirectives: m.extraDirectives ?? '',
      proxyMode: m.proxyMode ?? 'standard',
      proxyHeaders: m.proxyHeaders ?? '',
      healthCheckEnabled: m.healthCheckEnabled === 1,
      healthCheckIntervalSec: m.healthCheckIntervalSec ?? 30,
      healthCheckTargetPath: m.healthCheckTargetPath ?? '/health',
      healthCheckExpectedStatus: m.healthCheckExpectedStatus ?? 200,
    })
  }
}
loadExisting()

// 按协议显示字段
const isStream = computed(() => form.protocol === 'tcp' || form.protocol === 'udp')
const isHttp = computed(() => form.protocol === 'http')
const isHttps = computed(() => form.protocol === 'https')

const saving = ref(false)
const errors = ref<Record<string, string>>({})

// 危险字符：nginx.conf 注入防护。serverNames/listenPath/upstreamHost 不应含这些。
// ; {} # 是指令块/注释语法；换行可逃逸；? <> " ` 会生成非法 location 或注入
const INJECT_CHARS = /[;{}#\n\r?<>"`]/

// 校验
function validate(): boolean {
  const e: Record<string, string> = {}
  if (!form.name.trim())
    e.name = '请填写规则名称'
  if (!form.upstreamHost.trim())
    e.upstreamHost = '请填写内网服务 IP'
  else if (INJECT_CHARS.test(form.upstreamHost))
    e.upstreamHost = 'IP/域名含非法字符'
  if (form.upstreamPort == null) {
    e.upstreamPort = '请填写内网服务端口'
  }
  else {
    const p = Number(form.upstreamPort)
    if (!Number.isInteger(p) || p < 1 || p > 65535)
      e.upstreamPort = '端口范围 1-65535'
  }
  if ((isStream.value || isHttp.value) && form.listenPort == null) {
    e.listenPort = '请填写监听端口'
  }
  else if (form.listenPort != null) {
    const p = Number(form.listenPort)
    if (!Number.isInteger(p) || p < 1 || p > 65535)
      e.listenPort = '端口范围 1-65535'
  }
  if (form.serverNames && INJECT_CHARS.test(form.serverNames))
    e.serverNames = '域名含非法字符（; { } # ? 等）'
  if (form.listenPath && INJECT_CHARS.test(form.listenPath))
    e.listenPath = '路径含非法字符（; { } # ? 等）'
  if (isHttps.value) {
    if (!form.tlsCertPath.trim())
      e.tlsCertPath = 'HTTPS 需要 SSL 证书路径'
    if (!form.tlsKeyPath.trim())
      e.tlsKeyPath = 'HTTPS 需要 SSL 私钥路径'
  }
  // custom 模式必须填写自定义代理头
  if ((isHttp.value || isHttps.value) && form.proxyMode === 'custom' && !form.proxyHeaders.trim()) {
    e.proxyHeaders = '自定义模式必须填写代理头指令，或改回标准/精简模式'
  }
  errors.value = e
  return Object.keys(e).length === 0
}

// 协议图标
const protoIcons: Record<string, string> = {
  tcp: 'M5 8h14M5 12h14M5 16h14',
  udp: 'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6zM5 8h14M5 16h14',
  http: 'M3 12h18M12 3v18',
  https: 'M5 11h14v9H5zM8 11V7a4 4 0 0 1 8 0v4',
}

// 高级配置折叠
const advOpen = ref(false)

function setIp(ip: string) {
  form.upstreamHost = ip
  ipPopoverOpen.value = false
}

async function save() {
  if (!validate()) {
    toast('请补全必填项', 'err')
    return
  }

  // 端口占用检测
  if ((isStream.value || isHttp.value) && form.listenPort) {
    const available = await api.checkPort(form.listenPort)
    if (!available) {
      const ok = await confirmBox({
        title: '端口已被占用',
        message: `监听端口 ${form.listenPort} 已被占用，仍要保存吗？`,
        btnText: '仍然保存',
        kind: 'warn',
      })
      if (!ok)
        return
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
      proxyPassUrl: (isHttp.value || isHttps.value) && f.proxyPassUrl.trim() ? f.proxyPassUrl.trim() : null,
      tlsCertPath: isHttps.value ? f.tlsCertPath : null,
      tlsKeyPath: isHttps.value ? f.tlsKeyPath : null,
      redirectHttp: isHttps.value && f.redirectHttp ? 1 : 0,
      extraDirectives: f.extraDirectives || null,
      proxyMode: f.proxyMode,
      proxyHeaders: f.proxyMode === 'custom' ? (f.proxyHeaders || null) : null,
      healthCheckEnabled: f.healthCheckEnabled ? 1 : 0,
      healthCheckIntervalSec: f.healthCheckIntervalSec,
      healthCheckTargetPath: f.healthCheckEnabled ? f.healthCheckTargetPath || null : null,
      healthCheckExpectedStatus: f.healthCheckEnabled ? f.healthCheckExpectedStatus : null,
    }
    if (props.mappingId)
      await api.updateMapping(props.mappingId, input)
    else await api.createMapping(input)
    toast('映射已保存 · 记得点「应用」生效', 'ok')
    emit('saved')
  }
  catch (e: any) {
    toast(`保存失败: ${e.message}`, 'err')
  }
  finally {
    saving.value = false
  }
}

// 另存为自定义模板：把当前表单字段存成一个可复用模板
async function saveAsTemplate() {
  // Electron 里 window.prompt 不可靠，用自有 promptBox（带输入框的 modal）
  const name = await promptBox({
    title: '保存为模板',
    message: '为这个模板起个名字，方便以后复用。',
    btnText: '保存模板',
    kind: 'ok',
    defaultValue: form.name || '我的模板',
  })
  if (!name)
    return
  const id = `custom-${Date.now()}`
  try {
    await api.saveCustomTemplate({
      id,
      name,
      icon: form.protocol,
      category: '自定义',
      description: `用户保存 · ${form.protocol.toUpperCase()} ${form.upstreamPort || ''}`,
      builtin: false,
      // 用工具函数统一字段映射，补齐全部 16 字段（修旧版漏 tls/healthCheck + upstreamHost 硬编码 ''）
      mapping: formToTemplateMapping({ ...form, name }),
      extraDirectives: form.extraDirectives || undefined,
    })
    toast('模板已保存', 'ok')
  }
  catch (e: any) {
    toast(`保存模板失败: ${e.message}`, 'err')
  }
}

// Ctrl/Cmd+S 保存（preventDefault 避免 Electron 触发保存网页对话框）
function onKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault()
    save()
  }
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <ModalShell :title="props.mappingId ? '编辑映射' : '新建映射'" sub="带 * 为必填 · 保存后需点「应用」才生效" size="xl" @cancel="emit('cancel')">
    <div>
      <!-- 基础信息 -->
      <div class="form-sec">
        <div class="form-sec-tit">
          基础信息
        </div>

        <div class="field">
          <label>规则名称 <span class="req">*</span></label>
          <input v-model="form.name" class="input" placeholder="如：公司 MySQL 对外">
          <div v-if="errors.name" class="err-msg">
            {{ errors.name }}
          </div>
        </div>

        <div class="field">
          <label>协议 <span class="req">*</span></label>
          <div class="proto-pick">
            <label v-for="p in ['tcp', 'udp', 'http', 'https']" :key="p">
              <input v-model="form.protocol" type="radio" name="proto" :value="p">
              <div class="opt">
                <svg class="o-ico" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path :d="protoIcons[p]" /></svg>
                <span class="o-nm">{{ p.toUpperCase() }}</span>
              </div>
            </label>
          </div>
        </div>

        <div class="field-row c2">
          <div class="field">
            <label>内网服务 IP <span class="req">*</span></label>
            <div class="ip-field">
              <input v-model="form.upstreamHost" class="input mono" placeholder="192.168.1.10">
              <button class="ip-btn" @click.stop="ipPopoverOpen = !ipPopoverOpen">
                本机 IP
              </button>
              <div v-if="ipPopoverOpen" class="ip-pop" @click.stop>
                <div
                  v-for="ip in localIPs" :key="ip.address" class="ip-it"
                  @click="setIp(ip.address)"
                >
                  <span class="if">{{ ip.interface }}</span>
                  <span class="ad">{{ ip.address }}</span>
                </div>
                <div class="ip-it" @click="setIp('127.0.0.1')">
                  <span class="if">lo0 · 本地回环</span>
                  <span class="ad">127.0.0.1</span>
                </div>
                <div v-if="localIPs.length === 0" class="ip-empty">
                  获取本机 IP 失败
                </div>
              </div>
            </div>
            <div v-if="errors.upstreamHost" class="err-msg">
              {{ errors.upstreamHost }}
            </div>
          </div>
          <div class="field">
            <label>内网服务端口 <span class="req">*</span></label>
            <input v-model="form.upstreamPort" class="input mono" type="number" placeholder="3306">
            <div v-if="errors.upstreamPort" class="err-msg">
              {{ errors.upstreamPort }}
            </div>
          </div>
          <!-- proxy_pass 自定义（仅 HTTP/HTTPS）：填了原样透传，留空自动拼 host:port -->
          <div v-if="isHttp || isHttps" class="field">
            <label>proxy_pass 自定义 <span class="hint-inline">可选</span></label>
            <input v-model="form.proxyPassUrl" class="input mono" placeholder="留空自动拼 http://IP:端口/，填写则原样使用">
            <div class="hint">
              填写后 proxy_pass 完全用此值（如 http://27.196.154.94/），上面的 IP/端口仅用于显示。
            </div>
          </div>
        </div>
      </div>

      <!-- 协议相关字段 -->
      <div class="form-sec">
        <div v-if="isStream" class="form-sec-tit">
          {{ form.protocol.toUpperCase() }} 监听
        </div>
        <div v-else-if="isHttp" class="form-sec-tit">
          HTTP 配置
        </div>
        <div v-else-if="isHttps" class="form-sec-tit">
          HTTPS 配置
        </div>

        <!-- TCP/UDP -->
        <div v-if="isStream" class="field">
          <label>监听端口 <span class="req">*</span></label>
          <input v-model="form.listenPort" class="input mono" type="number" placeholder="9000">
          <div class="hint">
            跳板机对外开放的端口，外部连接 跳板机IP:此端口 即转发到内网服务。
          </div>
          <div v-if="errors.listenPort" class="err-msg">
            {{ errors.listenPort }}
          </div>
        </div>

        <!-- HTTP -->
        <template v-if="isHttp">
          <div class="field-row c2">
            <div class="field">
              <label>监听端口 <span class="req">*</span></label>
              <input v-model="form.listenPort" class="input mono" type="number" placeholder="80">
              <div v-if="errors.listenPort" class="err-msg">
                {{ errors.listenPort }}
              </div>
            </div>
            <div class="field">
              <label>域名 serverNames</label>
              <input v-model="form.serverNames" class="input mono" placeholder="api.example.com">
              <div v-if="errors.serverNames" class="err-msg">
                {{ errors.serverNames }}
              </div>
            </div>
          </div>
          <div class="field">
            <label>监听路径 listenPath</label>
            <input v-model="form.listenPath" class="input mono" placeholder="/">
            <div v-if="errors.listenPath" class="err-msg">
              {{ errors.listenPath }}
            </div>
          </div>
        </template>

        <!-- HTTPS -->
        <template v-if="isHttps">
          <div class="field-row c2">
            <div class="field">
              <label>监听端口 <span class="req">*</span></label>
              <input v-model="form.listenPort" class="input mono" type="number" placeholder="443">
              <div v-if="errors.listenPort" class="err-msg">
                {{ errors.listenPort }}
              </div>
            </div>
            <div class="field">
              <label>域名 serverNames</label>
              <input v-model="form.serverNames" class="input mono" placeholder="admin.example.com">
              <div v-if="errors.serverNames" class="err-msg">
                {{ errors.serverNames }}
              </div>
            </div>
          </div>
          <div class="field">
            <label>监听路径 listenPath</label>
            <input v-model="form.listenPath" class="input mono" placeholder="/">
            <div v-if="errors.listenPath" class="err-msg">
              {{ errors.listenPath }}
            </div>
          </div>
          <div class="field-row c2">
            <div class="field">
              <label>SSL 证书路径 <span class="req">*</span></label>
              <input v-model="form.tlsCertPath" class="input mono" placeholder="/etc/ssl/cert.pem">
              <div v-if="errors.tlsCertPath" class="err-msg">
                {{ errors.tlsCertPath }}
              </div>
            </div>
            <div class="field">
              <label>SSL 私钥路径 <span class="req">*</span></label>
              <input v-model="form.tlsKeyPath" class="input mono" placeholder="/etc/ssl/key.pem">
              <div v-if="errors.tlsKeyPath" class="err-msg">
                {{ errors.tlsKeyPath }}
              </div>
            </div>
          </div>
          <div class="switch-row">
            <ToggleSwitch v-model="form.redirectHttp" />
            <span>HTTP → HTTPS 自动跳转</span>
          </div>
        </template>
      </div>

      <!-- 高级配置 -->
      <button class="collapse-hd" :class="{ open: advOpen }" @click="advOpen = !advOpen">
        <span>高级配置 · 自定义指令 / 健康检查</span>
        <svg class="chev" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m9 6 6 6-6 6" /></svg>
      </button>
      <div v-if="advOpen" class="collapse-bd">
        <!-- 代理模式（仅 HTTP/HTTPS） -->
        <div v-if="isHttp || isHttps" class="field">
          <label>代理模式</label>
          <select v-model="form.proxyMode" class="select">
            <option value="standard">
              标准反代（自动生成 Host/WebSocket/大文件等头，适合多数 Web 服务）
            </option>
            <option value="minimal">
              精简代理（不加代理头，保留大文件/超时支持，适合 GitLab 等对 Host 敏感的服务）
            </option>
            <option value="custom">
              完全自定义（下方自定义代理头替代自动生成的）
            </option>
          </select>
          <div v-if="form.proxyMode === 'minimal'" class="hint">
            精简模式不加 Host/WebSocket 等代理头（GitLab 502 问题选这个），但仍保留 HTTP/1.1（git push 必需）、大文件上传和超时支持。
          </div>
        </div>
        <!-- 自定义代理头（仅 custom 模式 + HTTP/HTTPS 显示） -->
        <div v-if="(isHttp || isHttps) && form.proxyMode === 'custom'" class="field">
          <label>自定义代理头指令</label>
          <textarea
            v-model="form.proxyHeaders" class="textarea mono" rows="5"
            placeholder="# 完全替代自动生成的代理头，每行一条 nginx 指令&#10;proxy_http_version 1.1;&#10;proxy_set_header Host $host;&#10;proxy_set_header X-Real-IP $remote_addr;&#10;client_max_body_size 0;&#10;proxy_buffering off;"
          />
          <div class="hint">
            这些指令会替代标准反代自动生成的全部代理头。proxy_pass 不需要写（自动生成）。
          </div>
          <div v-if="errors.proxyHeaders" class="err-msg">
            {{ errors.proxyHeaders }}
          </div>
        </div>
        <div class="field">
          <label>自定义 nginx 指令</label>
          <textarea
            v-model="form.extraDirectives" class="textarea" rows="3"
            placeholder="# 高级用户可直接书写 nginx 指令&#10;proxy_read_timeout 60s;&#10;proxy_buffering off;"
          />
        </div>
        <div class="switch-row">
          <ToggleSwitch v-model="form.healthCheckEnabled" />
          <span>启用健康检查</span>
        </div>
        <div v-if="form.healthCheckEnabled" class="field-row c3">
          <div class="field">
            <label>检查间隔（秒）</label>
            <input v-model="form.healthCheckIntervalSec" class="input mono" type="number" min="5">
          </div>
          <div class="field">
            <label>检查路径</label>
            <input v-model="form.healthCheckTargetPath" class="input mono" placeholder="/health">
          </div>
          <div class="field">
            <label>期望状态码</label>
            <input v-model="form.healthCheckExpectedStatus" class="input mono" type="number" placeholder="200">
          </div>
        </div>
      </div>

      <!-- 端口占用提示 -->
      <div class="alert-warn">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 9v4M12 17h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z" /></svg>
        保存时会检测监听端口是否被占用，并提示冲突。
      </div>
    </div>

    <template #footer>
      <button class="btn btn-ghost" @click="emit('cancel')">
        取消
      </button>
      <div style="flex:1" />
      <button class="btn btn-dark" title="把当前配置存成可复用模板" @click="saveAsTemplate">
        另存为模板
      </button>
      <button class="btn btn-accent" :disabled="saving" @click="save">
        {{ saving ? '保存中…' : '保存映射' }}
      </button>
    </template>
  </ModalShell>
</template>

<style scoped>
/* overlay/modal 骨架抽为 ModalShell 组件 */

.form-sec { margin-bottom: 20px; }
.form-sec-tit { font-family: var(--font-mono); font-size: 10.5px; color: var(--meta); letter-spacing: 0.08em; text-transform: uppercase; margin-bottom: 12px; padding-bottom: 8px; border-bottom: 1px solid var(--border); }
.field { margin-bottom: 14px; }
.field > label { display: block; font-size: 12px; color: var(--fg-2); margin-bottom: 6px; font-weight: 500; }
.field .req, .req { color: var(--color-accent); }
.field .hint { font-size: 11px; color: var(--meta); margin-top: 5px; }
.err-msg { font-size: 11px; color: var(--color-danger); margin-top: 4px; }

.input, .textarea, .select {
  width: 100%;
  padding: 8px 11px;
  background: var(--surface-3);
  border: 1px solid var(--border-soft);
  border-radius: var(--r);
  font-size: 13px;
  color: var(--fg);
  transition: border-color var(--transition-fast);
  outline: none;
  font-family: inherit;
}
.input.mono, .textarea.mono { font-family: var(--font-mono); font-size: 12.5px; }
.input:focus, .textarea:focus, .select:focus { border-color: var(--color-accent); box-shadow: 0 0 0 2px rgba(201,100,66,.18); }
.input::placeholder, .textarea::placeholder { color: var(--meta); }
.textarea { resize: vertical; min-height: 70px; line-height: 1.6; }
.select { cursor: pointer; appearance: none; -webkit-appearance: none; background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%2387867f' stroke-width='2'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E"); background-repeat: no-repeat; background-position: right 10px center; background-size: 13px; padding-right: 30px; }

.field-row { display: grid; gap: 12px; }
.field-row.c2 { grid-template-columns: 1fr 1fr; }
.field-row.c3 { grid-template-columns: 1fr 1fr 1fr; }

/* 协议 radio card */
.proto-pick { display: grid; grid-template-columns: repeat(4, 1fr); gap: 6px; }
.proto-pick label { position: relative; cursor: pointer; display: block; }
.proto-pick input { position: absolute; opacity: 0; pointer-events: none; }
.proto-pick .opt { display: flex; flex-direction: column; align-items: center; gap: 5px; padding: 10px; border: 1px solid var(--border-soft); border-radius: var(--r); transition: all var(--transition-fast); background: var(--surface-3); color: var(--fg-2); }
.proto-pick input:checked + .opt { border-color: var(--color-accent); background: rgba(201,100,66,.07); color: var(--fg); }
.proto-pick .o-ico { width: 18px; height: 18px; }
.proto-pick input:checked + .opt .o-ico { color: var(--color-accent); }
.proto-pick .o-nm { font-family: var(--font-mono); font-size: 11px; font-weight: 600; letter-spacing: 0.05em; }

/* 本机 IP 选择器 */
.ip-field { position: relative; }
.ip-btn { position: absolute; right: 4px; top: 50%; transform: translateY(-50%); padding: 4px 8px; font-size: 11px; color: var(--color-accent); border-radius: var(--r-sm); background: rgba(201,100,66,.1); border: none; cursor: pointer; }
.ip-pop { position: absolute; top: calc(100% + 4px); left: 0; right: 0; background: var(--surface); border: 1px solid var(--border-soft); border-radius: var(--r); box-shadow: var(--shadow-md); z-index: 20; padding: 4px; }
.ip-it { display: flex; justify-content: space-between; padding: 6px 9px; border-radius: var(--r-sm); cursor: pointer; font-family: var(--font-mono); font-size: 11.5px; }
.ip-it:hover { background: var(--surface-2); }
.ip-it .if { color: var(--meta); }
.ip-it .ad { color: var(--fg); }
.ip-empty { padding: 8px; color: var(--meta); font-size: 11px; text-align: center; }

/* 开关：switch-row 是 toggle 外层排版；toggle 本身抽为 ToggleSwitch 组件 */
.switch-row { display: flex; align-items: center; gap: 10px; font-size: 12px; color: var(--fg-2); margin-bottom: 14px; }

/* 折叠面板 */
.collapse-hd { display: flex; align-items: center; justify-content: space-between; width: 100%; padding: 11px 14px; background: var(--surface-3); border: 1px solid var(--border-soft); border-radius: var(--r); font-size: 12.5px; color: var(--fg-2); font-weight: 500; cursor: pointer; }
.collapse-hd:hover { background: var(--surface-2); }
.collapse-hd .chev { transition: transform var(--transition-fast); color: var(--muted); width: 14px; height: 14px; }
.collapse-hd.open .chev { transform: rotate(90deg); }
.collapse-bd { padding: 14px 4px 0; }

/* 警告条 */
.alert-warn { display: flex; align-items: center; gap: 9px; padding: 9px 12px; border: 1px solid rgba(201,162,58,.4); background: rgba(201,162,58,.08); border-radius: var(--r); font-size: 12px; color: var(--color-warning); margin-top: 8px; }
.alert-warn svg { width: 15px; height: 15px; flex: none; }
</style>
