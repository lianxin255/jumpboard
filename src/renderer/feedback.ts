/**
 * 统一反馈层：Toast 轻提示 + Confirm 确认弹窗。
 * 挂载一次 <FeedbackHost /> 即全局可用。
 * 设计稿风格：Toast 底部居中圆点消息；Confirm 圆图标 + Serif 标题 + impact 提示。
 */
import { ref, h, defineComponent, onMounted, onUnmounted } from 'vue'

// ── Toast ──
type ToastKind = 'ok' | 'err' | 'info'
interface ToastItem { id: number; msg: string; kind: ToastKind }

const toastList = ref<ToastItem[]>([])
let toastSeq = 0

export function toast(msg: string, kind: ToastKind = 'info') {
  const id = ++toastSeq
  toastList.value.push({ id, msg, kind })
  setTimeout(() => {
    toastList.value = toastList.value.filter(t => t.id !== id)
  }, 2600)
}

// ── Confirm ──
interface ConfirmOptions {
  title: string
  message: string
  btnText?: string
  kind?: 'danger' | 'warn' | 'ok'
  impact?: string  // 危险后果说明
}
interface ConfirmState extends ConfirmOptions {
  visible: boolean
  resolve?: (v: boolean) => void
  // prompt 模式：带输入框。promptValue 为当前输入，promptDefault 为默认值
  prompt?: boolean
  promptDefault?: string
  promptValue?: string
}

const confirmState = ref<ConfirmState>({ title: '', message: '', visible: false })

export function confirmBox(opts: ConfirmOptions): Promise<boolean> {
  return new Promise(resolve => {
    // 若已有确认弹窗在显示，先把旧的 resolve(false) 掉，避免悬挂（旧 await 后续逻辑视为取消）
    if (confirmState.value.visible && confirmState.value.resolve) {
      confirmState.value.resolve(false)
    }
    confirmState.value = { ...opts, visible: true, resolve }
  })
}

/** 带输入框的确认。返回输入的字符串，取消返回 null。Electron 里 window.prompt 不可靠，用此替代。 */
export function promptBox(opts: ConfirmOptions & { defaultValue?: string }): Promise<string | null> {
  return new Promise(resolve => {
    if (confirmState.value.visible && confirmState.value.resolve) {
      confirmState.value.resolve(false)
    }
    confirmState.value = {
      ...opts,
      visible: true,
      prompt: true,
      promptDefault: opts.defaultValue ?? '',
      promptValue: opts.defaultValue ?? '',
      resolve: (v: boolean) => resolve(v ? (confirmState.value.promptValue ?? '') : null),
    }
  })
}

function closeConfirm(ok: boolean) {
  confirmState.value.resolve?.(ok)
  confirmState.value.visible = false
}

/** confirmBox/promptBox 是否正打开（供 ModalShell 的 Esc 守卫用，避免 DOM 查询时序竞态） */
export function isFeedbackModalOpen(): boolean {
  return confirmState.value.visible
}

// ── 宿主组件 ──
export const FeedbackHost = defineComponent({
  name: 'FeedbackHost',
  setup() {
    // Esc 关闭 confirm/prompt 弹窗（与遮罩点击同义：取消）
    function onKeydown(e: KeyboardEvent) {
      if (e.key === 'Escape' && confirmState.value.visible) {
        e.preventDefault()
        closeConfirm(false)
      }
    }
    onMounted(() => window.addEventListener('keydown', onKeydown))
    onUnmounted(() => window.removeEventListener('keydown', onKeydown))

    return () => [
      // Toast 容器
      h('div', { class: 'fb-toast-wrap' },
        toastList.value.map(t =>
          h('div', { class: 'fb-toast ' + t.kind, key: t.id }, [
            h('span', { class: 'fb-dot' }),
            t.msg,
          ])
        )
      ),
      // Confirm 遮罩
      confirmState.value.visible
        ? h('div', { class: 'fb-overlay', onClick: (e: Event) => { if (e.target === e.currentTarget) closeConfirm(false) } }, [
            h('div', { class: 'fb-modal' }, [
              h('div', { class: 'fb-modal-bd' }, [
                h('div', { class: 'fb-c-ico ' + (confirmState.value.kind || 'warn') }, [
                  h('svg', { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': 2 }, [
                    h('path', { d: confirmState.value.kind === 'danger' ? 'M12 9v4M12 17h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z' : 'M12 9v4M12 17h.01M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z' }),
                  ]),
                ]),
                h('h3', confirmState.value.title),
                h('p', confirmState.value.message),
                confirmState.value.impact
                  ? h('div', { class: 'fb-impact' }, confirmState.value.impact)
                  : null,
                // prompt 模式：渲染输入框（回车确认）
                confirmState.value.prompt
                  ? h('input', {
                      class: 'fb-input',
                      value: confirmState.value.promptValue,
                      placeholder: confirmState.value.promptDefault || '',
                      onInput: (e: Event) => { confirmState.value.promptValue = (e.target as HTMLInputElement).value },
                      onKeydown: (e: KeyboardEvent) => { if (e.key === 'Enter') { e.preventDefault(); closeConfirm(true) } },
                    })
                  : null,
              ]),
              h('div', { class: 'fb-modal-ft' }, [
                h('button', { class: 'btn btn-ghost', onClick: () => closeConfirm(false) }, '取消'),
                h('button', {
                  class: 'btn ' + (confirmState.value.kind === 'danger' ? 'btn-danger' : 'btn-accent'),
                  onClick: () => closeConfirm(true),
                }, confirmState.value.btnText || '确认'),
              ]),
            ]),
          ])
        : null,
    ]
  },
})

// ── 样式（全局注入一次） ──
const STYLE_ID = 'feedback-host-style'
if (typeof document !== 'undefined' && !document.getElementById(STYLE_ID)) {
  const style = document.createElement('style')
  style.id = STYLE_ID
  style.textContent = `
.fb-input{width:100%;padding:8px 11px;background:var(--surface-3);border:1px solid var(--border-soft);border-radius:var(--r);font-size:13px;color:var(--fg);outline:none;margin-top:14px;font-family:inherit}
.fb-input:focus{border-color:var(--color-accent);box-shadow:0 0 0 2px rgba(201,100,66,.18)}
.fb-input::placeholder{color:var(--meta)}
.fb-toast-wrap{position:fixed;bottom:24px;left:50%;transform:translateX(-50%);z-index:80;display:flex;flex-direction:column;gap:8px;align-items:center;pointer-events:none}
.fb-toast{display:flex;align-items:center;gap:9px;padding:10px 16px;background:#0e0e0d;border:1px solid var(--border-soft);border-radius:var(--r);box-shadow:var(--shadow-md);font-size:12.5px;animation:fadeUp .2s cubic-bezier(.2,0,0,1)}
.fb-dot{width:8px;height:8px;border-radius:50%;flex:none}
.fb-toast.ok .fb-dot{background:var(--color-success)}
.fb-toast.err .fb-dot{background:var(--color-danger)}
.fb-toast.info .fb-dot{background:var(--color-accent)}

.fb-overlay{position:fixed;inset:0;background:rgba(0,0,0,.6);backdrop-filter:blur(2px);z-index:50;display:flex;align-items:center;justify-content:center;animation:fade .15s}
.fb-modal{background:var(--surface);border:1px solid var(--border-soft);border-radius:var(--r-lg);box-shadow:var(--shadow-md);width:92%;max-width:420px;display:flex;flex-direction:column;animation:fadeUp .2s cubic-bezier(.2,0,0,1)}
.fb-modal-bd{padding:24px 24px 8px}
.fb-modal-ft{display:flex;align-items:center;justify-content:flex-end;gap:8px;padding:14px 20px}
.fb-c-ico{width:44px;height:44px;border-radius:50%;display:grid;place-items:center;margin-bottom:14px}
.fb-c-ico.danger{background:rgba(181,51,51,.12);color:var(--color-danger)}
.fb-c-ico.warn{background:rgba(201,162,58,.12);color:var(--color-warning)}
.fb-c-ico.ok{background:rgba(91,154,74,.12);color:var(--color-success)}
.fb-c-ico svg{width:22px;height:22px}
.fb-modal h3{font-family:var(--font-display);font-size:18px;font-weight:500;margin-bottom:8px}
.fb-modal p{font-size:13px;color:var(--fg-2);line-height:1.6}
.fb-impact{font-family:var(--font-mono);font-size:11.5px;color:var(--color-danger);background:rgba(181,51,51,.08);border:1px solid rgba(181,51,51,.25);border-radius:var(--r);padding:8px 10px;margin-top:12px}
`
  document.head.appendChild(style)
}
