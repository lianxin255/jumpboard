import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@renderer/api/ipc'

export interface Mapping {
  id: string
  name: string
  enabled: number
  protocol: string
  listenPort: number | null
  upstreamHost: string
  upstreamPort: number
  healthStatus: string
}

export const useMappingsStore = defineStore('mappings', () => {
  const list = ref<Mapping[]>([])
  const loading = ref(false)

  async function load(hostId: string) {
    loading.value = true
    try {
      list.value = await api.listMappings(hostId)
    } finally {
      loading.value = false
    }
  }

  async function create(input: any) {
    await api.createMapping(input)
  }

  async function remove(id: string) {
    await api.deleteMapping(id)
  }

  async function toggle(id: string, enabled: number) {
    // 乐观更新：先改本地状态，UI 立即响应
    const item = list.value.find(m => m.id === id)
    if (item) item.enabled = enabled
    try {
      await api.updateMapping(id, { enabled })
    } catch (e) {
      // 失败则回滚
      if (item) item.enabled = enabled === 1 ? 0 : 1
      throw e
    }
  }

  return { list, loading, load, create, remove, toggle }
})
