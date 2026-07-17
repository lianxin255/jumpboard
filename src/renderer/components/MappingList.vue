<script setup lang="ts">
import { useMappingsStore } from '@renderer/stores/mappings'
const store = useMappingsStore()
const emit = defineEmits<{ (e: 'new'): void; (e: 'edit', id: string): void }>()
</script>

<template>
  <div class="mapping-list">
    <div v-if="store.list.length === 0" class="empty">
      还没有映射规则，点下方按钮新建一条 TCP 转发试试
    </div>
    <div v-for="m in store.list" :key="m.id" class="card" @click="emit('edit', m.id)">
      <span class="dot" :class="m.healthStatus"></span>
      <div class="info">
        <div class="name">{{ m.name }}</div>
        <div class="meta">{{ m.protocol.toUpperCase() }} · {{ m.listenPort }} → {{ m.upstreamHost }}:{{ m.upstreamPort }}</div>
      </div>
      <input type="checkbox" :checked="m.enabled === 1"
        @click.stop @change="store.toggle(m.id, m.enabled === 1 ? 0 : 1)" />
    </div>
    <button class="new-btn" @click="emit('new')">+ 新建映射</button>
  </div>
</template>

<style scoped>
.mapping-list { display: flex; flex-direction: column; gap: 8px; padding: 16px; }
.empty { color: #888; padding: 32px; text-align: center; }
.card { display: flex; align-items: center; gap: 12px; padding: 12px 16px;
  background: #fff; border: 1px solid #e5e5e5; border-radius: 12px; cursor: pointer; }
.card:hover { border-color: #0A84FF; }
.dot { width: 8px; height: 8px; border-radius: 50%; background: #ccc; }
.dot.healthy { background: #34c759; }
.dot.unhealthy { background: #ff3b30; }
.info { flex: 1; }
.name { font-weight: 600; }
.meta { font-size: 13px; color: #888; margin-top: 2px; }
.new-btn { margin-top: 8px; padding: 10px; border: none; border-radius: 12px;
  background: #0A84FF; color: #fff; font-size: 15px; cursor: pointer; }
</style>
