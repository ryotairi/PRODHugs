<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import { ensureAccessToken } from '@/lib/token'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { useAuthStore } from '@/stores/auth'
import { useHugsStore, type HugFeedItem, type PendingHugInboxItem } from '@/stores/hugs'
import { useOnlineStore } from '@/stores/online'
import { useAnnouncementStore, type Announcement } from '@/stores/announcement'
import { useWebSocket } from '@/composables/useWebSocket'
import { hugCompletedToast } from '@/lib/utils'
import { SidebarProvider, SidebarInset, SidebarTrigger } from '@/components/ui/sidebar'
import { Toaster } from '@/components/ui/sonner'
import AppSidebar from '@/components/AppSidebar.vue'
import AppHeader from '@/components/AppHeader.vue'
import AppBottomNav from '@/components/AppBottomNav.vue'
import AnnouncementBanner from '@/components/AnnouncementBanner.vue'

const auth = useAuthStore()
const hugsStore = useHugsStore()
const onlineStore = useOnlineStore()
const announcementStore = useAnnouncementStore()
const route = useRoute()
const ws = useWebSocket()

const authReady = ref(false)

const showLayout = computed(() => {
  return auth.isAuthenticated && !['login', 'register'].includes(route.name as string)
})

const wsCleanups: Array<() => void> = []

// Connect WebSocket and fetch inbox count when authenticated
async function initAuthState() {
  if (!auth.isAuthenticated) {
    // Attempt silent token refresh (e.g. after page reload with a valid refresh cookie)
    await ensureAccessToken()
  }
  if (auth.isAuthenticated) {
    // Refresh user info from the server to get the latest role (admin flag)
    try {
      await auth.fetchMe()
    } catch {
      // If fetching fails, the auth store will handle logout.
    }
    ws.connect()
    hugsStore.fetchInboxCount()
    hugsStore.fetchOutgoing()
    announcementStore.fetchActive()
    setupGlobalWsListeners()
  }
  authReady.value = true
}

function clearGlobalWsListeners() {
  wsCleanups.forEach((fn) => fn())
  wsCleanups.length = 0
}

function setupGlobalWsListeners() {
  clearGlobalWsListeners()

  wsCleanups.push(
    ws.on<Record<string, unknown>>('hug_suggestion', (data) => {
      // Normalize keys because backend sends domain model over WS without json tags
      const item: PendingHugInboxItem = {
        id: String(data.id || data.ID),
        giver_id: String(data.giver_id || data.GiverID),
        receiver_id: String(data.receiver_id || data.ReceiverID),
        giver_username: String(data.giver_username || data.GiverUsername),
        giver_gender: data.giver_gender || data.GiverGender ? String(data.giver_gender || data.GiverGender) : null,
        giver_display_name: data.giver_display_name || data.GiverDisplayName ? String(data.giver_display_name || data.GiverDisplayName) : null,
        hug_type: (String(data.hug_type || data.HugType || 'standard') as PendingHugInboxItem['hug_type']),
        created_at: String(data.created_at || data.CreatedAt),
      }
      
      // Prevent duplicates in case of fast reconnections or simultaneous API fetch
      const exists = hugsStore.inbox.find((h) => h.id === item.id)
      if (!exists) {
        hugsStore.inbox.unshift(item)
        hugsStore.inboxCount++
      }
    }),
  )

  wsCleanups.push(
    ws.on<{ hug_id: string; receiver_id: string }>('hug_declined', (data) => {
      hugsStore.outgoingHugs = hugsStore.outgoingHugs.filter((h) => h.id !== data.hug_id)
      hugsStore.slotInfo.used_slots = hugsStore.outgoingHugs.length
      toast('Твоя обнимашка была отклонена')
      if (data.receiver_id) {
        hugsStore.triggerCooldownRefresh(data.receiver_id)
      }
    }),
  )

  wsCleanups.push(
    ws.on<{ hug_id: string }>('hug_cancelled', (data) => {
      hugsStore.inbox = hugsStore.inbox.filter((h) => h.id !== data.hug_id)
      hugsStore.inboxCount = Math.max(0, hugsStore.inboxCount - 1)
    }),
  )

  wsCleanups.push(
    ws.on<{ hug_id: string }>('hug_expired', (data) => {
      hugsStore.outgoingHugs = hugsStore.outgoingHugs.filter((h) => h.id !== data.hug_id)
      hugsStore.slotInfo.used_slots = hugsStore.outgoingHugs.length
      hugsStore.inbox = hugsStore.inbox.filter((h) => h.id !== data.hug_id)
      hugsStore.inboxCount = Math.max(0, hugsStore.inboxCount - 1)
    }),
  )

  wsCleanups.push(
    ws.on<HugFeedItem>('hug_completed', (data) => {
      if (data.giver_id === auth.user?.id) {
        hugsStore.outgoingHugs = hugsStore.outgoingHugs.filter(
          (h) => h.receiver_id !== data.receiver_id,
        )
        hugsStore.slotInfo.used_slots = hugsStore.outgoingHugs.length
        toast.success(hugCompletedToast(data.receiver_username, data.hug_type))
        hugsStore.fetchBalance()
        hugsStore.triggerCooldownRefresh(data.receiver_id)
      } else if (data.receiver_id === auth.user?.id) {
        hugsStore.triggerCooldownRefresh(data.giver_id)
      }
    }),
  )

  wsCleanups.push(
    ws.on<{ count: number }>('inbox_count', (data) => {
      hugsStore.inboxCount = data.count
    }),
  )

  wsCleanups.push(
    ws.on<{ user_ids: string[] }>('online_users', (data) => {
      onlineStore.setUsers(data.user_ids)
    }),
  )

  wsCleanups.push(
    ws.on<Announcement>('announcement', (data) => {
      announcementStore.setFromWS(data)
    }),
  )

  wsCleanups.push(
    ws.on<{ id: string }>('announcement_removed', () => {
      announcementStore.clearFromWS()
    }),
  )
}

// Watch for auth changes (login/logout)
watch(
  () => auth.isAuthenticated,
  (isAuth) => {
    if (isAuth) {
      ws.connect()
      hugsStore.fetchInboxCount()
      hugsStore.fetchOutgoing()
      setupGlobalWsListeners()
    } else {
      ws.disconnect()
      clearGlobalWsListeners()
      onlineStore.clear()
      announcementStore.clearFromWS()
      hugsStore.inboxCount = 0
      hugsStore.inbox = []
      hugsStore.outgoingHugs = []
      hugsStore.slotInfo = { total_slots: 1, used_slots: 0, next_slot_cost: 10 }
    }
  },
)

onMounted(() => {
  initAuthState()
})

onUnmounted(() => {
  clearGlobalWsListeners()
})
</script>

<template>
  <template v-if="!authReady">
    <!-- Prevent layout flash while initial auth check is in progress -->
  </template>
  <template v-else-if="showLayout">
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <header class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
          <SidebarTrigger class="-ml-1 hidden md:inline-flex" />
          <AppHeader />
        </header>
        <AnnouncementBanner />
        <main class="flex-1 p-3 pb-24 sm:p-6 sm:pb-24 md:pb-6">
          <RouterView />
        </main>
      </SidebarInset>
      <AppBottomNav />
    </SidebarProvider>
  </template>
  <template v-else>
    <RouterView />
  </template>
  <Toaster position="top-right" />
</template>
