import { api } from '@web/api/http'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Mapping {
  id: string
  name: string
  enabled: number
  protocol: string
  listenPort: number | null
  serverNames: string | null
  listenPath: string | null
  upstreamHost: string
  upstreamPort: number
  tlsCertPath: string | null
  tlsKeyPath: string | null
  redirectHttp: number
  extraDirectives: string | null
  healthCheckEnabled: number
  healthStatus: string
  lastCheckedAt: number | null
  sortOrder: number
  createdAt: number
  updatedAt: number
}

export const useMappingsStore = defineStore('mappings', () => {
  const list = ref<Mapping[]>([])
  const loading = ref(false)

  async function load(hostId: string) {
    loading.value = true
    try {
      list.value = await api.listMappings(hostId)
    }
    finally {
      loading.value = false
    }
  }

  async function create(input: any) {
    await api.createMapping(input)
  }

  async function remove(id: string) {
    await api.deleteMapping(id)
    // 乐观更新：立即从本地列表移除，UI 即时响应
    list.value = list.value.filter(m => m.id !== id)
  }

  async function toggle(id: string, enabled: number) {
    // 乐观更新：先改本地状态，UI 立即响应
    const item = list.value.find(m => m.id === id)
    if (item)
      item.enabled = enabled
    try {
      await api.updateMapping(id, { enabled })
    }
    catch (e) {
      // 失败则回滚
      if (item)
        item.enabled = enabled === 1 ? 0 : 1
      throw e
    }
  }

  // 批量启停：复用单条 toggle（乐观更新逐条生效）。返回成功数，供前端准确 toast。
  async function batchToggle(ids: string[], enabled: number): Promise<number> {
    const results = await Promise.all(ids.map(id => toggle(id, enabled).then(() => true).catch(() => false)))
    return results.filter(Boolean).length
  }

  // 批量删除：复用单条 remove。返回成功数。
  async function batchRemove(ids: string[]): Promise<number> {
    const results = await Promise.all(ids.map(id => remove(id).then(() => true).catch(() => false)))
    return results.filter(Boolean).length
  }

  return { list, loading, load, create, remove, toggle, batchToggle, batchRemove }
})
