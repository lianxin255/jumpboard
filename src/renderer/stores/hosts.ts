import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@renderer/api/ipc'

export interface Host {
  id: string
  name: string
  type: 'local' | 'remote'
  isDefault: number
  remoteConfig?: {
    host?: string
    port?: number
    username?: string
    authType?: string
    passwordEncrypted?: string | null
    keyPath?: string | null
    nginxPath?: string
    confPath?: string
    reloadCmd?: string
    testCmd?: string
  } | null
  localConfig?: { nginxPath?: string; confDir?: string; logDir?: string } | null
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

  async function create(input: any) {
    await api.createHost(input)
    await load()
  }
  async function update(id: string, patch: any) {
    await api.updateHost(id, patch)
    await load()
  }
  async function remove(id: string) {
    await api.deleteHost(id)
    await load()
  }
  async function testConnection(remoteConfig: any) {
    return await api.testHostConnection(remoteConfig)
  }
  async function refreshStatus() {
    return await api.refreshEngineStatus()
  }

  return { hosts, currentHostId, load, setCurrent, create, update, remove, testConnection, refreshStatus }
})
