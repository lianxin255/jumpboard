import { api } from '@web/api/http'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Template {
  id: string
  name: string
  icon: string
  category: string
  description: string
  builtin: boolean
  mapping: Record<string, any>
  extraDirectives?: string
}

export const useTemplatesStore = defineStore('templates', () => {
  const list = ref<Template[]>([])
  const loading = ref(false)

  async function load() {
    loading.value = true
    try {
      list.value = await api.listTemplates()
    }
    finally {
      loading.value = false
    }
  }

  return { list, loading, load }
})
