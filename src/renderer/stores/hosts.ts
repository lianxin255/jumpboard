import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@renderer/api/ipc'

export interface Host {
  id: string
  name: string
  type: 'local' | 'remote'
  isDefault: number
  status?: string
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

  // 设为默认主机（落库）
  async function setDefault(id: string) {
    await api.setDefaultHost(id)
    await load()
  }

  // 切换活动主机：通知后端真正切换 nginx 引擎，成功时更新前端 currentHostId。
  // 返回完整结果（含 error 信息），让调用方能拿到失败原因展示给用户。
  async function switchTo(id: string) {
    const r = await api.switchHost(id)
    if (r.ok) currentHostId.value = id
    return r
  }

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

  return { hosts, currentHostId, load, setCurrent, setDefault, switchTo, create, update, remove, testConnection, refreshStatus }
})
