import { api } from '@web/api/http'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Snapshot {
  id: string
  hostId: string
  content: string
  reason: string
  createdAt: number
}

export const useSnapshotsStore = defineStore('snapshots', () => {
  const list = ref<Snapshot[]>([])

  async function load(hostId: string) {
    list.value = await api.listSnapshots(hostId)
  }

  async function rollback(snapshotId: string): Promise<{ ok: boolean, output: string }> {
    return await api.rollbackSnapshot(snapshotId)
  }

  return { list, load, rollback }
})
