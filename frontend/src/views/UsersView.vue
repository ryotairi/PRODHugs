<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { Search, Loader2, Star, Zap, Crown, Coins as Coin } from 'lucide-vue-next'
import { useHugsStore } from '@/stores/hugs'
import { useOnlineStore } from '@/stores/online'
import { useAuthStore } from '@/stores/auth'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import UserCard from '@/components/UserCard.vue'
import OutgoingHugsSection from '@/components/OutgoingHugsSection.vue'
import PromotionModal from '@/components/PromotionModal.vue'

const PAGE_SIZE = 30

const hugsStore = useHugsStore()
const onlineStore = useOnlineStore()
const auth = useAuthStore()
const query = ref('')
const users = ref<any[]>([])
const loading = ref(false)
const loadingMore = ref(false)
const hasMore = ref(true)
const sentinel = ref<HTMLElement | null>(null)
const promotionOpen = ref(false)

watch(promotionOpen, (isOpen) => {
  if (isOpen) {
    auth.fetchMe()
  }
})

// ── Sponsored users (top 3) ──
const sponsoredUsers = computed(() => {
  const promoted = users.value.filter(u => u.promoted_until && new Date(u.promoted_until) > new Date())
  // Only take first 3 for the special slots
  return promoted.slice(0, 3)
})

// ── Main list users (excluding those shown in sponsored slots) ──
const mainListUsers = computed(() => {
  const sponsoredIds = new Set(sponsoredUsers.value.map(u => u.id))
  
  return users.value
    .filter(u => !sponsoredIds.has(u.id))
    .sort((a, b) => {
      // 1. Other promoted users (if > 3)
      const aPromoted = a.promoted_until && new Date(a.promoted_until) > new Date() ? 0 : 1
      const bPromoted = b.promoted_until && new Date(b.promoted_until) > new Date() ? 0 : 1
      if (aPromoted !== bPromoted) return aPromoted - bPromoted

      // 2. Online status
      const aOnline = onlineStore.isOnline(a.id) ? 0 : 1
      const bOnline = onlineStore.isOnline(b.id) ? 0 : 1
      if (aOnline !== bOnline) return aOnline - bOnline

      // 3. Preserve backend order (which is fastest response time first)
      return 0
    })
})

const isMePromoted = computed(() => {
  if (!auth.user?.promoted_until) return false
  return new Date(auth.user.promoted_until) > new Date()
})

let debounceTimer: ReturnType<typeof setTimeout> | null = null
// Monotonic counter to discard out-of-order search responses.
let searchGeneration = 0
let observer: IntersectionObserver | null = null

async function search() {
  const gen = ++searchGeneration
  loading.value = true
  hasMore.value = true
  try {
    const result = await hugsStore.searchUsers(query.value, PAGE_SIZE, 0)
    if (gen !== searchGeneration) return
    users.value = result
    hasMore.value = result.length >= PAGE_SIZE
  } finally {
    if (gen === searchGeneration) {
      loading.value = false
    }
  }
  await nextTick()
  observeSentinel()
}

async function loadMore() {
  if (loadingMore.value || !hasMore.value || loading.value) return
  const gen = searchGeneration
  loadingMore.value = true
  try {
    const result = await hugsStore.searchUsers(query.value, PAGE_SIZE, users.value.length)
    if (gen !== searchGeneration) return
    users.value.push(...result)
    hasMore.value = result.length >= PAGE_SIZE
  } finally {
    if (gen === searchGeneration) {
      loadingMore.value = false
    }
  }
}

function observeSentinel() {
  observer?.disconnect()
  if (!sentinel.value) return
  observer = new IntersectionObserver(
    (entries) => {
      if (entries[0]?.isIntersecting) {
        loadMore()
      }
    },
    { rootMargin: '200px' },
  )
  observer.observe(sentinel.value)
}

watch(query, () => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(search, 300)
})

onMounted(search)

onUnmounted(() => {
  if (debounceTimer) {
    clearTimeout(debounceTimer)
    debounceTimer = null
  }
  observer?.disconnect()
  // Increment generation so any in-flight search response is discarded.
  searchGeneration++
})
</script>

<template>
  <div class="mx-auto max-w-2xl space-y-6">
    <div>
      <h1 class="text-2xl font-semibold tracking-tight">Пользователи</h1>
      <p class="text-muted-foreground">Обнимись с кем-нибудь</p>
    </div>

    <OutgoingHugsSection />

    <div class="relative">
      <Search class="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        v-model="query"
        type="text"
        class="pl-9"
        placeholder="Поиск по имени..."
        maxlength="64"
      />
    </div>

    <!-- ── VIP Slots (Top 3) ── -->
    <div v-if="!query && !loading" class="space-y-2">
      <p class="text-[10px] uppercase font-bold text-muted-foreground tracking-wider ml-1 text-prod-yellow">VIP-места</p>
      <div class="grid gap-2">
        <!-- Real promoted users -->
        <UserCard v-for="user in sponsoredUsers" :key="user.id" :user="user" />
        
        <!-- Placeholders if less than 3 -->
        <div 
          v-for="i in (3 - sponsoredUsers.length)" 
          :key="'empty-' + i"
          class="flex items-center justify-center rounded-[10px] border border-dashed border-muted-foreground/30 h-[66px] bg-muted/5 group cursor-pointer hover:border-prod-yellow/50 transition-colors"
          @click="promotionOpen = true"
        >
          <div class="flex items-center gap-2 text-xs text-muted-foreground group-hover:text-prod-yellow transition-colors">
            <Star class="size-3" />
            <span>Свободный VIP-слот</span>
          </div>
        </div>
      </div>
    </div>

    <div v-if="loading && users.length === 0" class="space-y-3">
      <Skeleton v-for="i in 5" :key="i" class="h-16 w-full rounded-lg" />
    </div>

    <div v-else-if="users.length === 0" class="py-12 text-center text-muted-foreground">
      Такого не нашли(
    </div>

    <div v-else class="space-y-2">
      <TransitionGroup name="user-list" tag="div" class="space-y-2">
        <UserCard v-for="user in mainListUsers" :key="user.id" :user="user" />
      </TransitionGroup>

      <div v-if="hasMore" ref="sentinel" class="flex justify-center py-4">
        <Loader2 v-if="loadingMore" class="size-5 animate-spin text-muted-foreground" />
      </div>
    </div>

    <PromotionModal v-model:open="promotionOpen" @success="search" />
  </div>
</template>

<style scoped>
.user-list-move {
  transition: transform 0.4s ease;
}
</style>
