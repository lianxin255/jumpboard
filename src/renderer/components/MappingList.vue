<script setup lang="ts">
import { PlusOutlined } from '@antdv-next/icons'
import { useMappingsStore } from '@renderer/stores/mappings'
const store = useMappingsStore()
const emit = defineEmits<{ (e: 'new'): void; (e: 'edit', id: string): void }>()
</script>

<template>
  <div class="mapping-list">
    <div v-if="store.list.length === 0" class="empty">
      还没有映射规则，点下方按钮新建一条 TCP 转发试试
    </div>
    <div v-for="m in store.list" :key="m.id" class="card glass-card" @click="emit('edit', m.id)">
      <span class="dot" :class="m.healthStatus"></span>
      <div class="info">
        <div class="name">{{ m.name }}</div>
        <div class="meta">{{ m.protocol.toUpperCase() }} · {{ m.listenPort }} → {{ m.upstreamHost }}:{{ m.upstreamPort }}</div>
      </div>
      <a-switch size="small" :checked="m.enabled === 1"
        @click.stop @change="store.toggle(m.id, $event ? 1 : 0)" />
    </div>
    <a-button class="new-btn" type="primary" @click="emit('new')">
      <template #icon><PlusOutlined /></template>
      新建映射
    </a-button>
  </div>
</template>

<style scoped>
.mapping-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-4);
}

.empty {
  color: var(--text-tertiary);
  padding: var(--space-8);
  text-align: center;
  font-size: var(--font-size-base);
}

.card {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  cursor: pointer;
  transition: transform var(--transition-fast), box-shadow var(--transition-fast), border-color var(--transition-fast);
}

.card:hover {
  transform: translateY(-1px);
  box-shadow: var(--shadow-lg);
  border-color: var(--color-accent);
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: var(--radius-pill);
  background: var(--text-tertiary);
  flex-shrink: 0;
}
.dot.healthy { background: var(--color-success); }
.dot.unhealthy { background: var(--color-danger); }

.info { flex: 1; min-width: 0; }
.name { font-weight: 600; font-size: var(--font-size-md); }
.meta {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin-top: 2px;
}

.new-btn { margin-top: var(--space-2); }
</style>
