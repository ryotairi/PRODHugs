import { defineStore } from 'pinia'
import { ref } from 'vue'
import { announcementsApi } from '@/api/client'

export interface Announcement {
  id: string
  message: string
  created_at: string
}

export const useAnnouncementStore = defineStore('announcement', () => {
  const active = ref<Announcement | null>(null)

  async function fetchActive() {
    try {
      const res = await announcementsApi.getActive()
      if (res.status === 204 || !res.data) {
        active.value = null
      } else {
        active.value = res.data
      }
    } catch {
      active.value = null
    }
  }

  async function dismiss(id: string) {
    try {
      await announcementsApi.dismiss(id)
      active.value = null
    } catch {
      // Ignore
    }
  }

  function setFromWS(data: Announcement) {
    active.value = data
  }

  function clearFromWS() {
    active.value = null
  }

  return {
    active,
    fetchActive,
    dismiss,
    setFromWS,
    clearFromWS,
  }
})
