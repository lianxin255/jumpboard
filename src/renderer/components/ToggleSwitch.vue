<script setup lang="ts">
/**
 * 通用开关。消除 4 个文件里逐字重复的 toggle CSS + DOM 结构。
 * 用法：<ToggleSwitch v-model="enabled" size="sm" />
 * size 不传=标准(34×18)，"sm"=小号(28×16)。
 */
defineProps<{
  modelValue: boolean
  size?: 'sm'
}>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()
function onChange(e: Event) {
  emit('update:modelValue', (e.target as HTMLInputElement).checked)
}
</script>

<template>
  <label class="toggle" :class="{ sm: size === 'sm' }">
    <input type="checkbox" :checked="modelValue" @change="onChange">
    <span class="track"><span class="knob"></span></span>
  </label>
</template>

<style scoped>
.toggle { position: relative; width: 34px; height: 18px; flex: none; cursor: pointer; display: inline-block; }
.toggle.sm { width: 28px; height: 16px; }
.toggle input { position: absolute; opacity: 0; pointer-events: none; }
.toggle .track { position: absolute; inset: 0; background: var(--surface-2); border: 1px solid var(--border-soft); border-radius: var(--r-md); transition: all 0.18s var(--ease); }
.toggle .knob { position: absolute; top: 2px; left: 2px; width: 12px; height: 12px; background: var(--muted); border-radius: var(--r-sm); transition: all 0.18s var(--ease); }
.toggle.sm .knob { width: 10px; height: 10px; }
.toggle input:checked + .track { background: rgba(91,154,74,.22); border-color: rgba(91,154,74,.5); }
.toggle input:checked + .track .knob { left: 18px; background: var(--color-success); }
.toggle.sm input:checked + .track .knob { left: 14px; }
</style>
