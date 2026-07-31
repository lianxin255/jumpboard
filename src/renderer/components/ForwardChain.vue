<script setup lang="ts">
/**
 * 转发关系链路可视化：外部 → :监听端口 → 内网IP:端口
 * 设计稿的灵魂元素，映射列表中每行的核心信息。
 */
const props = defineProps<{
  protocol: string
  listenPort: number | null
  upstreamHost: string
  upstreamPort: number
}>()
</script>

<template>
  <div class="fwd">
    <span class="node ext">外部</span>
    <span class="arr">→</span>
    <span class="node listen">:{{ props.listenPort ?? '?' }}</span>
    <span class="arr lit">→</span>
    <span class="node target">
      <span class="ip">{{ props.upstreamHost }}</span>:<span class="port">{{ props.upstreamPort }}</span>
    </span>
  </div>
</template>

<style scoped>
.fwd {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: var(--font-mono);
  font-size: 12.5px;
  min-width: 0;
}
.node {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 8px;
  border-radius: var(--r-sm);
  background: var(--surface-3);
  border: 1px solid var(--border-soft);
  white-space: nowrap;
}
.node.ext { color: var(--meta); }
.node.listen {
  color: var(--color-accent);
  border-color: rgba(201, 100, 66, 0.4);
  background: rgba(201, 100, 66, 0.07);
}
.node.target { color: var(--fg); }
.node .ip { color: var(--fg-2); }
.node .port { color: var(--fg); font-weight: 600; }
.arr { color: var(--meta); font-size: 11px; flex: none; }
.arr.lit { color: var(--color-accent); }
</style>
