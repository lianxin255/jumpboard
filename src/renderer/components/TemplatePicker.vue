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
    <div class="picker-card glass-card">
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
        <button class="btn btn-ghost" @click="emit('cancel')">取消</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.picker-overlay {
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

.picker-card {
  padding: var(--space-6);
  width: 640px;
  max-width: 92vw;
  max-height: 80vh;
  overflow-y: auto;
}

h3 { margin-bottom: var(--space-4); font-size: var(--font-size-lg); }

.empty { color: var(--text-tertiary); padding: var(--space-8); text-align: center; }

.group { margin-bottom: var(--space-5); }

.group-title {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  font-weight: 600;
  margin-bottom: var(--space-2);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: var(--space-2);
}

.tpl-card {
  display: flex;
  gap: var(--space-3);
  padding: var(--space-3);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-glass);
  cursor: pointer;
  text-align: left;
  align-items: flex-start;
  transition: all var(--transition-fast);
}

.tpl-card:hover {
  border-color: var(--color-accent);
  background: var(--color-accent-soft);
  transform: translateY(-1px);
}

.icon { font-size: 24px; }
.info { flex: 1; min-width: 0; }
.name {
  font-weight: 600;
  font-size: var(--font-size-base);
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.custom-tag {
  font-size: var(--font-size-xs);
  background: var(--color-accent);
  color: #fff;
  padding: 1px var(--space-1);
  border-radius: var(--radius-sm);
}

.desc {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin-top: 2px;
  line-height: 1.4;
}

.actions { display: flex; justify-content: flex-end; margin-top: var(--space-4); }
</style>
