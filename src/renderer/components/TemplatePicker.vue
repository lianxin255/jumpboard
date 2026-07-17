<script setup lang="ts">
import { computed, markRaw } from 'vue'
import {
  ApiOutlined,
  ConsoleSqlOutlined,
  CodeOutlined,
  DatabaseOutlined,
  DesktopOutlined,
  GlobalOutlined,
  LockOutlined,
  MessageOutlined,
  ThunderboltOutlined,
  AppstoreOutlined,
} from '@antdv-next/icons'
import type { Component } from 'vue'
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

// icon id → antd icon 组件（markRaw 避免被 Vue 响应式包装）
const iconMap: Record<string, Component> = markRaw({
  network: ApiOutlined, // tcp/udp
  terminal: ConsoleSqlOutlined, // ssh
  monitor: DesktopOutlined, // rdp
  globe: GlobalOutlined, // website
  lock: LockOutlined, // website-https
  code: CodeOutlined, // api
  bolt: ThunderboltOutlined, // websocket
  database: DatabaseOutlined, // mysql/postgresql/redis/mongodb
  message: MessageOutlined, // mqtt
  default: AppstoreOutlined,
})

function iconOf(t: Template): Component {
  return iconMap[t.icon] ?? iconMap.default
}
</script>

<template>
  <a-modal
    :open="true"
    title="选择模板"
    :footer="null"
    :width="680"
    :mask-closable="true"
    @cancel="emit('cancel')"
  >
    <div v-if="store.loading" class="empty">加载中...</div>
    <div v-else-if="store.list.length === 0" class="empty">没有可用模板</div>
    <div v-for="[cat, items] in grouped" :key="cat" class="group">
      <div class="group-title">{{ cat }}</div>
      <div class="cards">
        <button
          v-for="t in items"
          :key="t.id"
          class="tpl-card"
          @click="emit('pick', t)"
        >
          <span class="icon"><component :is="iconOf(t)" /></span>
          <div class="info">
            <div class="name">{{ t.name }}<span v-if="!t.builtin" class="custom-tag">自定义</span></div>
            <div class="desc">{{ t.description }}</div>
          </div>
        </button>
      </div>
    </div>
    <div class="actions">
      <a-button @click="emit('cancel')">取消</a-button>
    </div>
  </a-modal>
</template>

<style scoped>
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
  color: inherit;
  font: inherit;
}

.tpl-card:hover {
  border-color: var(--color-accent);
  background: var(--color-accent-soft);
  transform: translateY(-1px);
}

.icon {
  font-size: 22px;
  color: var(--color-accent);
  line-height: 1;
  display: inline-flex;
  align-items: center;
}
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
