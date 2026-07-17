import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@renderer/api/ipc'

export interface Host {
  id: string
  name: string
  type: 'local' | 'remote'
  isDefault: number
}

export const useHostsStore = defineStore('hosts', () => {
  const hosts = ref<Host[]>([])
  const currentHostId = ref<string | null>(null)

  async function load() {
    hosts.value = await api.listHosts()
    const def = hosts.value.find(h => h.isDefault)
    currentHostId.value = def?.id ?? hosts.value[0]?.id ?? null
  }

  function setCurrent(id: string) { currentHostId.value = id }

  return { hosts, currentHostId, load, setCurrent }
})
