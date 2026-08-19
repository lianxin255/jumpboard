import { api } from '@web/api/http'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface LogEntry {
  id: string
  hostId: string | null
  level: string
  source: string
  message: string
  timestamp: number
}

export const useLogsStore = defineStore('logs', () => {
  const list = ref<LogEntry[]>([])
  const loading = ref(false)

  async function load(hostId: string | null, limit = 500) {
    loading.value = true
    try {
      list.value = await api.listLogs(hostId, limit)
    }
    finally {
      loading.value = false
    }
  }

  return { list, loading, load }
})
