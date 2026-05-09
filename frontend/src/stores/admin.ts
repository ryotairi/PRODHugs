import { defineStore } from 'pinia'
import { ref } from 'vue'
import { adminApi } from '@/api/client'
import type { Gender } from '@/stores/auth'

export interface AdminUser {
  id: string
  username: string
  role: string
  gender?: Gender | null
  display_name?: string | null
  tag?: string | null
  special_tag?: string | null
  banned_at?: string | null
  created_at?: string | null
  last_visit_at?: string | null
  balance: number
  requires_sudoku?: boolean
  sudoku_cooldown_until?: string | null
}

export interface AdminStats {
  total_users: number
  banned_users: number
}

const PAGE_SIZE = 20

export const useAdminStore = defineStore('admin', () => {
  const stats = ref<AdminStats | null>(null)
  const users = ref<AdminUser[]>([])
  const loading = ref(false)
  const loadingMore = ref(false)
  const hasMore = ref(true)
  const offset = ref(0)
  const searchQuery = ref('')

  async function fetchStats() {
    const res = await adminApi.getStats()
    stats.value = res.data
  }

  async function fetchUsers(query?: string) {
    if (query !== undefined) searchQuery.value = query
    loading.value = true
    offset.value = 0
    try {
      const res = await adminApi.getUsers(PAGE_SIZE, 0, searchQuery.value || undefined)
      users.value = res.data
      hasMore.value = res.data.length === PAGE_SIZE
      offset.value = res.data.length
    } finally {
      loading.value = false
    }
  }

  async function loadMore() {
    if (loadingMore.value || loading.value || !hasMore.value) return
    loadingMore.value = true
    try {
      const res = await adminApi.getUsers(PAGE_SIZE, offset.value, searchQuery.value || undefined)
      users.value.push(...res.data)
      hasMore.value = res.data.length === PAGE_SIZE
      offset.value += res.data.length
    } finally {
      loadingMore.value = false
    }
  }

  async function banUser(userId: string) {
    const res = await adminApi.banUser(userId)
    const updated: AdminUser = res.data
    const idx = users.value.findIndex((u) => u.id === userId)
    if (idx !== -1) users.value[idx] = updated
    if (stats.value) stats.value.banned_users++
    return updated
  }

  async function unbanUser(userId: string) {
    const res = await adminApi.unbanUser(userId)
    const updated: AdminUser = res.data
    const idx = users.value.findIndex((u) => u.id === userId)
    if (idx !== -1) users.value[idx] = updated
    if (stats.value) stats.value.banned_users--
    return updated
  }

  async function updateUsername(userId: string, username: string) {
    const res = await adminApi.updateUsername(userId, username)
    const updated: AdminUser = res.data
    const idx = users.value.findIndex((u) => u.id === userId)
    if (idx !== -1) users.value[idx] = updated
    return updated
  }

  async function updateGender(userId: string, gender: string | null) {
    const res = await adminApi.updateGender(userId, gender)
    const updated: AdminUser = res.data
    const idx = users.value.findIndex((u) => u.id === userId)
    if (idx !== -1) users.value[idx] = updated
    return updated
  }

  async function updateDisplayName(userId: string, displayName: string | null) {
    const res = await adminApi.updateDisplayName(userId, displayName)
    const updated: AdminUser = res.data
    const idx = users.value.findIndex((u) => u.id === userId)
    if (idx !== -1) users.value[idx] = updated
    return updated
  }

  async function updateTag(userId: string, tag: string | null) {
    const res = await adminApi.updateTag(userId, tag)
    const updated: AdminUser = res.data
    const idx = users.value.findIndex((u) => u.id === userId)
    if (idx !== -1) users.value[idx] = updated
    return updated
  }

  async function updateSpecialTag(userId: string, specialTag: string | null) {
    const res = await adminApi.updateSpecialTag(userId, specialTag)
    const updated: AdminUser = res.data
    const idx = users.value.findIndex((u) => u.id === userId)
    if (idx !== -1) users.value[idx] = updated
    return updated
  }

  async function updateRequiresSudoku(userId: string, requiresSudoku: boolean) {
    const res = await adminApi.updateRequiresSudoku(userId, requiresSudoku)
    const updated: AdminUser = res.data
    const idx = users.value.findIndex((u) => u.id === userId)
    if (idx !== -1) users.value[idx] = updated
    return updated
  }

  async function updatePassword(userId: string, password: string) {
    await adminApi.updatePassword(userId, password)
  }

  async function updateBalance(userId: string, amount: number) {
    await adminApi.updateBalance(userId, amount)
    const idx = users.value.findIndex((u) => u.id === userId)
    if (idx !== -1) users.value[idx]!.balance = amount
  }

  async function deleteUser(userId: string) {
    await adminApi.deleteUser(userId)
    const idx = users.value.findIndex((u) => u.id === userId)
    if (idx !== -1) users.value.splice(idx, 1)
    if (stats.value) stats.value.total_users--
  }

  return {
    stats,
    users,
    loading,
    loadingMore,
    hasMore,
    searchQuery,
    fetchStats,
    fetchUsers,
    loadMore,
    banUser,
    unbanUser,
    updateUsername,
    updateGender,
    updateDisplayName,
    updateTag,
    updateSpecialTag,
    updateRequiresSudoku,
    updatePassword,
    updateBalance,
    deleteUser,
  }
})
