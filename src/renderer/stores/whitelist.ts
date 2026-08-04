import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@renderer/api/ipc'

export interface WhitelistEntry {
  id: string
  ip: string
  note: string | null
  enabled: number
  createdAt: number
}

export const useWhitelistStore = defineStore('whitelist', () => {
  const list = ref<WhitelistEntry[]>([])
  const enabled = ref(false)

  async function load() {
    list.value = await api.listWhitelist()
    enabled.value = await api.isWhitelistEnabled()
  }

  async function add(ip: string, note?: string) {
    await api.addWhitelist(ip, note)
    await load()
  }

  async function remove(id: string) {
    await api.removeWhitelist(id)
    await load()
  }

  async function toggleIp(id: string, on: boolean) {
    await api.toggleWhitelistIp(id, on ? 1 : 0)
    await load()
  }

  async function toggleEnabled(on: boolean) {
    await api.setWhitelistEnabled(on)
    enabled.value = on
  }

  return { list, enabled, load, add, remove, toggleIp, toggleEnabled }
})
