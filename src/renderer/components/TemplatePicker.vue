<script setup lang="ts">
import { computed } from 'vue'
import { useTemplatesStore } from '@renderer/stores/templates'
import type { Template } from '@renderer/stores/templates'

const store = useTemplatesStore()
const emit = defineEmits<{ (e: 'pick', tpl: Template): void; (e: 'cancel'): void }>()

// 按 category 分组
const grouped = computed(() => {
  const map = new Map<string, Template[]>()
  for (const t of store.list) {
    if (!map.has(t.category)) map.set(t.category, [])
    map.get(t.category)!.push(t)
  }
  return Array.from(map.entries())
})

// icon 映射到 emoji（毛玻璃版会换 SVG，Plan 4）
const iconEmoji: Record<string, string> = {
  network: '🔌', terminal: '💻', monitor: '🖥️', globe: '🌐', lock: '🔒',
  code: '⚙️', bolt: '⚡', database: '🗄️', message: '📡', default: '📦'
}
</script>

<template>
  <div class="picker-overlay" @click.self="emit('cancel')">
    <div class="picker-card">
      <h3>选择模板</h3>
      <div v-if="store.loading" class="empty">加载中...</div>
      <div v-else-if="store.list.length === 0" class="empty">没有可用模板</div>
      <div v-for="[cat, items] in grouped" :key="cat" class="group">
        <div class="group-title">{{ cat }}</div>
        <div class="cards">
          <button v-for="t in items" :key="t.id" class="tpl-card" @click="emit('pick', t)">
            <span class="icon">{{ iconEmoji[t.icon] ?? '📦' }}</span>
            <div class="info">
              <div class="name">{{ t.name }}<span v-if="!t.builtin" class="custom-tag">自定义</span></div>
              <div class="desc">{{ t.description }}</div>
            </div>
          </button>
        </div>
      </div>
      <div class="actions">
        <button class="ghost" @click="emit('cancel')">取消</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.picker-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.3); display: flex;
  align-items: center; justify-content: center; z-index: 10; }
.picker-card { background: #fff; border-radius: 16px; padding: 24px; width: 640px; max-width: 92vw;
  max-height: 80vh; overflow-y: auto; }
h3 { margin: 0 0 16px; }
.empty { color: #888; padding: 32px; text-align: center; }
.group { margin-bottom: 20px; }
.group-title { font-size: 13px; color: #888; font-weight: 600; margin-bottom: 8px; text-transform: uppercase; }
.cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 8px; }
.tpl-card { display: flex; gap: 10px; padding: 12px; border: 1px solid #e5e5e5; border-radius: 12px;
  background: #fff; cursor: pointer; text-align: left; align-items: flex-start; }
.tpl-card:hover { border-color: #0A84FF; background: #f0f7ff; }
.icon { font-size: 24px; }
.info { flex: 1; }
.name { font-weight: 600; font-size: 14px; display: flex; align-items: center; gap: 6px; }
.custom-tag { font-size: 10px; background: #0A84FF; color: #fff; padding: 1px 5px; border-radius: 4px; }
.desc { font-size: 12px; color: #888; margin-top: 2px; line-height: 1.4; }
.actions { display: flex; justify-content: flex-end; margin-top: 16px; }
button.ghost { padding: 8px 16px; border: none; border-radius: 8px; background: #eee; color: #333; cursor: pointer; }
</style>
