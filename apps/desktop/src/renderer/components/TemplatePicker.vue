<script setup lang="ts">
import type { Template } from '@renderer/stores/templates'
import { api } from '@renderer/api/ipc'
import ModalShell from '@renderer/components/ModalShell.vue'
import { confirmBox, toast } from '@renderer/feedback'
import { useTemplatesStore } from '@renderer/stores/templates'
import { protocolIconPaths as iconPaths } from '@renderer/utils/icons'
import { computed, onMounted, ref } from 'vue'

const emit = defineEmits<{ (e: 'pick', tpl: Template | null): void, (e: 'cancel'): void }>()
const store = useTemplatesStore()
// 每次打开选择器都刷新列表：保存自定义模板后立即可见（否则要重启应用）
onMounted(() => {
  void store.load()
})
// 按 category 分组（保持设计稿顺序）
const categoryOrder = ['网络层', 'Web', '数据库', '远程访问', '消息', 'API']
const grouped = computed(() => {
  const map = new Map<string, Template[]>()
  for (const t of store.list) {
    if (!map.has(t.category))
      map.set(t.category, [])
    map.get(t.category)!.push(t)
  }
  // 按 categoryOrder 排序，未知分类放最后
  return categoryOrder
    .filter(c => map.has(c))
    .map(c => [c, map.get(c)!] as [string, Template[]])
    .concat(
      Array.from(map.entries()).filter(([c]) => !categoryOrder.includes(c)),
    )
})

// icon id → SVG path（用设计稿的图标，比 antd 图标更统一）
// tcp/udp/http/https 用于自定义模板（saveAsTemplate 存的 icon = protocol 值）
// iconPaths 已抽到 @renderer/utils/icons

const selected = ref<string | null>(null)

function pickTpl(t: Template) {
  selected.value = t.id
}
function pickCustom() {
  selected.value = '__custom__'
}

// 删除自定义模板
async function deleteCustom(t: Template, e: Event) {
  e.stopPropagation()
  const ok = await confirmBox({
    title: '删除模板',
    message: `确定删除自定义模板「${t.name}」吗？`,
    btnText: '删除',
    kind: 'danger',
    impact: '此操作不可撤销。',
  })
  if (!ok)
    return
  try {
    await api.deleteCustomTemplate(t.id)
    await store.load()
    // 删的是当前选中项时重置，避免 confirmPick 找不到项静默哑火
    if (selected.value === t.id)
      selected.value = null
    toast('模板已删除', 'ok')
  }
  catch (err: any) {
    toast(`删除失败: ${err.message}`, 'err')
  }
}
function confirmPick() {
  if (selected.value === '__custom__') {
    emit('pick', null)
    return
  }
  const t = store.list.find(x => x.id === selected.value)
  if (t)
    emit('pick', t)
}
</script>

<template>
  <ModalShell title="选择模板" sub="预填字段后进入表单，可随时修改" size="lg" @cancel="emit('cancel')">
    <div v-if="store.loading" class="empty-state" style="padding:40px">
      加载中…
    </div>
    <div v-else-if="store.list.length === 0" class="empty-state" style="padding:40px">
      没有可用模板
    </div>

    <template v-for="[cat, items] in grouped" :key="cat">
      <div class="tpl-cat-hd">
        {{ cat }}
      </div>
      <div class="tpl-grid">
        <div
          v-for="t in items" :key="t.id" class="tpl-card"
          :class="{ sel: selected === t.id }" @click="pickTpl(t)"
        >
          <button v-if="!t.builtin" class="tpl-del" title="删除" @click="deleteCustom(t, $event)">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12" /></svg>
          </button>
          <div class="t-ico">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <path :d="iconPaths[t.icon] || iconPaths.default" />
            </svg>
          </div>
          <div class="t-nm">
            {{ t.name }}<span v-if="!t.builtin" class="t-custom">自定义</span>
          </div>
          <div class="t-desc">
            {{ t.description }}
          </div>
          <div v-if="t.mapping?.upstreamPort" class="t-port">
            端口 {{ t.mapping.upstreamPort }}
          </div>
          <div v-else-if="t.mapping?.listenPort" class="t-port">
            端口 {{ t.mapping.listenPort }}
          </div>
        </div>
      </div>
    </template>

    <!-- 自定义 -->
    <div class="tpl-cat-hd">
      自定义
    </div>
    <div class="tpl-grid">
      <div class="tpl-card custom" :class="{ sel: selected === '__custom__' }" @click="pickCustom">
        <div class="t-ico">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M12 5v14M5 12h14" /></svg>
        </div>
        <div class="t-nm">
          自定义
        </div>
        <div class="t-desc">
          空白表单，全部字段自行填写
        </div>
        <div class="t-port">
          任意协议
        </div>
      </div>
    </div>

    <template #footer>
      <button class="btn btn-ghost" @click="emit('cancel')">
        取消
      </button>
      <button class="btn btn-accent" :disabled="!selected" @click="confirmPick">
        下一步：填写参数
      </button>
    </template>
  </ModalShell>
</template>

<style scoped>
/* overlay/modal 骨架抽为 ModalShell 组件 */

.tpl-cat-hd { font-family: var(--font-mono); font-size: 10.5px; color: var(--meta); letter-spacing: 0.08em; text-transform: uppercase; margin: 4px 0 10px; }
.tpl-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 10px; margin-bottom: 18px; }
.tpl-del { position: absolute; top: 6px; right: 6px; width: 20px; height: 20px; display: grid; place-items: center; border-radius: var(--r-sm); color: var(--muted); background: none; border: none; cursor: pointer; opacity: 0; transition: opacity var(--transition-fast); padding: 0; }
.tpl-del svg { width: 12px; height: 12px; }
.tpl-del:hover { color: var(--color-danger); background: rgba(181,51,51,.12); }
.tpl-card {
  position: relative;
  display: flex; flex-direction: column; gap: 7px;
  padding: 14px;
  border: 1px solid var(--border-soft);
  border-radius: var(--r);
  cursor: pointer;
  transition: all var(--transition-fast);
  background: var(--surface-3);
}
.tpl-card:hover .tpl-del { opacity: 1; }
.tpl-card:hover { border-color: var(--muted); background: var(--surface-2); transform: translateY(-1px); }
.tpl-card.sel { border-color: var(--color-accent); background: rgba(201,100,66,.06); }
.tpl-card.custom { border-style: dashed; }
.t-ico { width: 30px; height: 30px; border-radius: var(--r-sm); display: grid; place-items: center; background: var(--surface-2); color: var(--fg-2); }
.t-ico svg { width: 16px; height: 16px; }
.t-nm { font-size: 13px; font-weight: 500; color: var(--fg); display: flex; align-items: center; gap: 6px; }
.t-custom { font-family: var(--font-mono); font-size: 9px; padding: 1px 4px; border-radius: var(--r-sm); background: var(--color-accent); color: var(--fg); }
.t-desc { font-size: 11px; color: var(--muted); line-height: 1.4; }
.t-port { font-family: var(--font-mono); font-size: 10.5px; color: var(--color-accent); }
</style>
