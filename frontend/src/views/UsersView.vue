<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { Search, Loader2, Star, Zap, Crown, Coins as Coin, Timer as TimerIcon } from 'lucide-vue-next'
import { useHugsStore } from '@/stores/hugs'
import { useOnlineStore } from '@/stores/online'
import { useAuthStore } from '@/stores/auth'
import { useWebSocket } from '@/composables/useWebSocket'
import { useTicker } from '@/composables/useTicker'
import { formatRemainingTime } from '@/lib/utils'
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
const ws = useWebSocket()
const { now } = useTicker()
const query = ref('')
const sortBySpeed = ref(false)
const users = ref<any[]>([])
const loading = ref(false)
const loadingVips = ref(false)
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
  return hugsStore.vips.slice(0, 3)
})

// ── Main list users (excluding those shown in sponsored slots) ──
const mainListUsers = computed(() => {
  const sponsoredIds = new Set(sponsoredUsers.value.map(u => u.id.toLowerCase()))
  
  let list = users.value.filter(u => !sponsoredIds.has(u.id.toLowerCase()))
  
  if (sortBySpeed.value) {
    return [...list].sort((a, b) => {
      // 1. VIPs always first
      const aPromoted = a.promoted_until && new Date(a.promoted_until) > new Date() ? 0 : 1
      const bPromoted = b.promoted_until && new Date(b.promoted_until) > new Date() ? 0 : 1
      if (aPromoted !== bPromoted) return aPromoted - bPromoted
      
      // 2. Sort by response speed (fastest first)
      // Treat null/negative as very slow (Infinity)
      const aSpeed = (a.avg_response_time !== null && a.avg_response_time >= 0) ? a.avg_response_time : Infinity
      const bSpeed = (b.avg_response_time !== null && b.avg_response_time >= 0) ? b.avg_response_time : Infinity
      
      if (aSpeed !== bSpeed) return aSpeed - bSpeed
      
      // 3. Activity status
      const aActive = a.is_recently_active ? 0 : 1
      const bActive = b.is_recently_active ? 0 : 1
      if (aActive !== bActive) return aActive - bActive
      
      return 0
    })
  }

  return list.sort((a, b) => {
    // 1. Other VIPs (if > 3)
    const aPromoted = a.promoted_until && new Date(a.promoted_until) > new Date() ? 0 : 1
    const bPromoted = b.promoted_until && new Date(b.promoted_until) > new Date() ? 0 : 1
    if (aPromoted !== bPromoted) return aPromoted - bPromoted

    // 2. Online status
    const aOnline = onlineStore.isOnline(a.id) ? 0 : 1
    const bOnline = onlineStore.isOnline(b.id) ? 0 : 1
    if (aOnline !== bOnline) return aOnline - bOnline

    // 3. Activity status
    const aActive = a.is_recently_active ? 0 : 1
    const bActive = b.is_recently_active ? 0 : 1
    if (aActive !== bActive) return aActive - bActive

    // 4. Preserve backend order (default)
    return 0
  })
})

const isMePromoted = computed(() => {
  if (!auth.user?.promoted_until) return false
  return new Date(auth.user.promoted_until) > new Date()
})

const isMeOnCooldown = computed(() => {
  if (!auth.user?.vip_cooldown_until) return false
  return new Date(auth.user.vip_cooldown_until) > new Date()
})

const isMeInTop3 = computed(() => {
  if (!auth.user?.id) return false
  const myId = auth.user.id.toLowerCase()
  return sponsoredUsers.value.some(u => u.id.toLowerCase() === myId)
})

const isMeOutbid = computed(() => {
  // Don't show "outbid" status while VIPs are loading to prevent flickering
  if (loadingVips.value) return false
  return isMePromoted.value && !isMeInTop3.value
})

// Store the timestamp when my budget was last updated
const myBudgetLastUpdatedAt = ref(Date.now())
watch(() => auth.user?.vip_remaining_seconds, () => {
  myBudgetLastUpdatedAt.value = Date.now()
})

const myRemainingTime = computed(() => {
  if (!isMePromoted.value || auth.user?.vip_remaining_seconds === undefined) return ''
  
  let seconds = auth.user.vip_remaining_seconds
  if (isMeInTop3.value && seconds > 0) {
    const elapsedSinceUpdate = Math.floor((now.value - myBudgetLastUpdatedAt.value) / 1000)
    seconds = Math.max(0, seconds - elapsedSinceUpdate)
  }
  
  return formatRemainingTime(seconds)
})

const cooldownTimeText = computed(() => {
  if (!isMeOnCooldown.value || !auth.user?.vip_cooldown_until) return ''
  const diff = new Date(auth.user.vip_cooldown_until).getTime() - now.value
  const seconds = Math.max(0, Math.floor(diff / 1000))
  return formatRemainingTime(seconds)
})

let debounceTimer: ReturnType<typeof setTimeout> | null = null
// Monotonic counter to discard out-of-order search responses.
let searchGeneration = 0
let observer: IntersectionObserver | null = null

async function search() {
  const gen = ++searchGeneration
  loading.value = true
  loadingVips.value = true
  hasMore.value = true
  try {
    const [userList] = await Promise.all([
      hugsStore.searchUsers(query.value, PAGE_SIZE, 0),
      query.value ? Promise.resolve([]) : hugsStore.fetchVIPs()
    ])
    
    if (gen !== searchGeneration) return
    users.value = userList
    hasMore.value = userList.length >= PAGE_SIZE
  } finally {
    if (gen === searchGeneration) {
      loading.value = false
      loadingVips.value = false
    }
  }
  await nextTick()
  observeSentinel()
}

async function refreshVIPs() {
  try {
    loadingVips.value = true
    await Promise.all([
      hugsStore.fetchVIPs(),
      auth.fetchMe(),
      // Re-fetch current search results to update stars/borders in the main list
      hugsStore.searchUsers(query.value, Math.max(PAGE_SIZE, users.value.length), 0).then(res => {
        users.value = res
      })
    ])
  } catch {
    // Ignore
  } finally {
    loadingVips.value = false
  }
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

let wsCleanup: (() => void) | null = null

onMounted(() => {
  search()
  wsCleanup = ws.on('vips_updated', () => {
    refreshVIPs()
  })
})

onUnmounted(() => {
  if (debounceTimer) {
    clearTimeout(debounceTimer)
    debounceTimer = null
  }
  observer?.disconnect()
  wsCleanup?.()
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

    <div 
      class="rounded-[10px] border p-4 flex items-center justify-between gap-4 transition-all duration-300"
      :class="{
        'border-prod-yellow/30 bg-prod-yellow/5': isMeInTop3,
        'border-destructive/30 bg-destructive/5': isMeOutbid,
        'border-blue-500/30 bg-blue-500/5': isMeOnCooldown,
        'border-muted bg-muted/5': !isMePromoted && !isMeOnCooldown
      }"
    >
      <div class="space-y-1 flex-1">
        <h3 class="text-sm font-semibold flex items-center gap-2 flex-wrap" :class="isMeInTop3 ? 'text-prod-yellow' : isMeOutbid ? 'text-destructive' : isMeOnCooldown ? 'text-blue-400' : ''">
          <div class="flex items-center gap-1.5">
            <template v-if="isMeOnCooldown">
              <TimerIcon class="size-4" />
              <span>Ты устал</span>
            </template>
            <template v-else>
              <Star class="size-4" :class="isMeInTop3 ? 'fill-prod-yellow text-prod-yellow' : ''" />
              <span>{{ isMeInTop3 ? 'Ты в ТОПе!' : isMeOutbid ? 'Твою ставку перебили!' : 'Хочешь в ТОП?' }}</span>
            </template>
          </div>

          <span v-if="isMeInTop3 && myRemainingTime" class="text-[10px] font-mono bg-prod-yellow/20 px-1.5 py-0.5 rounded animate-pulse">
            {{ myRemainingTime }}
          </span>
          <span v-if="isMeOnCooldown && cooldownTimeText" class="text-[10px] font-mono bg-blue-500/20 px-1.5 py-0.5 rounded">
            {{ cooldownTimeText }}
          </span>
        </h3>
        <p class="text-xs text-muted-foreground">
          <template v-if="isMeOnCooldown">
            Твой 24-часовой VIP-лимит исчерпан. Подожди 6 часов перед следующей ставкой.
          </template>
          <template v-else>
            {{ isMeInTop3 ? 'Ты занимаешь VIP-место. Повысь ставку, чтобы подняться ещё выше.' : isMeOutbid ? 'Тебя вытеснили из Топ-3. Подними ставку, чтобы вернуться!' : 'Займи VIP-место, чтобы тебя видели первым!' }}
          </template>
        </p>
      </div>
      <Button 
        variant="outline" 
        size="sm" 
        class="shrink-0" 
        :disabled="isMeOnCooldown"
        :class="isMeInTop3 ? 'border-prod-yellow text-prod-yellow hover:bg-prod-yellow hover:text-black' : isMeOutbid ? 'border-destructive text-destructive hover:bg-destructive hover:text-white' : isMeOnCooldown ? 'border-blue-500/50 text-blue-400' : 'border-prod-yellow text-prod-yellow hover:bg-prod-yellow hover:text-black'" 
        @click="promotionOpen = true"
      >
        {{ isMeOnCooldown ? 'Отдых' : isMeInTop3 ? 'Повысить' : isMeOutbid ? 'Вернуть место' : 'Занять место' }}
      </Button>
    </div>

    <div class="flex items-center gap-2">
      <div class="flex items-center gap-2 rounded-md border bg-background px-3 focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 flex-1">
        <Search class="size-4 shrink-0 text-muted-foreground" />
        <input
          v-model="query"
          type="text"
          class="flex h-9 w-full bg-transparent py-1 text-sm outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50"
          placeholder="Поиск по имени или @username"
          maxlength="64"
        />
      </div>
      <Button 
        variant="outline" 
        size="sm" 
        class="h-9 shrink-0 gap-2"
        :class="sortBySpeed ? 'bg-emerald-500/10 text-emerald-500 border-emerald-500/50 hover:bg-emerald-500/20' : ''"
        @click="sortBySpeed = !sortBySpeed"
      >
        <Zap class="size-3.5" :class="sortBySpeed ? 'fill-emerald-500' : ''" />
        <span class="hidden sm:inline">{{ sortBySpeed ? 'Сортировка: Быстрые' : 'Самые быстрые' }}</span>
      </Button>
    </div>

    <!-- ── VIP Slots (Top 3) ── -->
    <div v-if="!query && !loading" class="space-y-2">
      <p class="text-[10px] uppercase font-bold text-muted-foreground tracking-wider ml-1 text-prod-yellow">VIP-места</p>
      <div class="grid gap-2">
        <!-- Real promoted users -->
        <UserCard 
          v-for="user in sponsoredUsers" 
          :key="user.id" 
          :user="user" 
          is-vip
        />
        
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
        <UserCard 
          v-for="user in mainListUsers" 
          :key="user.id" 
          :user="user" 
          :is-vip="false"
        />
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
