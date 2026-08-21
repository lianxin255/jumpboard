<script setup lang="ts">
import { isFeedbackModalOpen } from '@renderer/feedback'
/**
 * 通用弹窗骨架。消除 3 个文件里重复的 overlay/modal-hd/close-x/modal-bd/modal-ft 结构 + CSS。
 *
 * 用法：
 *   <ModalShell title="标题" sub="副标题" size="lg" @cancel="close">
 *     <内容（放进 modal-bd）>
 *     <template #footer><button>...</button></template>
 *   </ModalShell>
 *
 * size: 不传=默认(560px) / sm=420px / lg=860px / xl=1000px
 * 点遮罩、关闭按钮、按 Esc 都触发 cancel。
 */
import { onMounted, onUnmounted } from 'vue'

defineProps<{
  title: string
  sub?: string
  size?: 'sm' | 'lg' | 'xl'
}>()
const emit = defineEmits<{ (e: 'cancel'): void }>()

// Esc 关闭（与遮罩点击同义）。一处覆盖三个弹窗。
// 守卫：若 confirmBox/promptBox 正打开，让 FeedbackHost 处理，不误关弹窗。
// 用 isFeedbackModalOpen（响应式状态）而非 DOM 查询，避免渲染时序竞态。
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    if (isFeedbackModalOpen())
      return
    e.preventDefault()
    emit('cancel')
  }
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="overlay" @click.self="emit('cancel')">
    <div class="modal" :class="size">
      <div class="modal-hd">
        <div>
          <h3>{{ title }}</h3>
          <div v-if="sub" class="sub">
            {{ sub }}
          </div>
        </div>
        <button class="close-x" @click="emit('cancel')">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12" /></svg>
        </button>
      </div>

      <div class="modal-bd">
        <slot />
      </div>

      <div v-if="$slots.footer" class="modal-ft">
        <slot name="footer" />
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 严格匹配 TemplatePicker/HostManager 现有样式（它们一致），统一后 MappingForm 也对齐此版 */
.overlay {
  position: fixed; inset: 0;
  background: rgba(0,0,0,.6);
  backdrop-filter: blur(2px);
  z-index: 50;
  display: flex; align-items: center; justify-content: center;
  animation: fadeIn 0.15s var(--ease);
}
@keyframes fadeIn { from { opacity: 0 } to { opacity: 1 } }

.modal {
  background: var(--surface);
  border: 1px solid var(--border-soft);
  border-radius: var(--r-lg);
  box-shadow: var(--shadow-md);
  width: 92%; max-width: 720px; max-height: 88vh;
  display: flex; flex-direction: column;
  animation: slideUp 0.2s var(--ease);
}
.modal.sm { max-width: 420px; }
.modal.lg { max-width: 860px; }
.modal.xl { max-width: 1000px; }
@keyframes slideUp { from { transform: translateY(12px); opacity: 0 } to { transform: translateY(0); opacity: 1 } }

.modal-hd { display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; border-bottom: 1px solid var(--border); }
.modal-hd h3 { font-family: var(--font-display); font-size: 18px; font-weight: 500; }
.modal-hd .sub { font-family: var(--font-mono); font-size: 11px; color: var(--muted); margin-top: 2px; }
.close-x { width: 28px; height: 28px; display: grid; place-items: center; border-radius: var(--r-sm); color: var(--muted); background: none; border: none; cursor: pointer; }
.close-x:hover { background: var(--surface-2); color: var(--fg); }
.close-x svg { width: 14px; height: 14px; }

.modal-bd { padding: 20px; overflow: auto; flex: 1; }

.modal-ft { display: flex; align-items: center; justify-content: flex-end; gap: 8px; padding: 14px 20px; border-top: 1px solid var(--border); }
</style>
