<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useOnlineStore } from '@/stores/online'
import { useTicker } from '@/composables/useTicker'
import { formatRemainingTime } from '@/lib/utils'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Send, Star, Zap, Coins as Coin, Timer as TimerIcon } from 'lucide-vue-next'
import HugButton from './HugButton.vue'
import UserTag from './UserTag.vue'

const props = defineProps<{
  user: {
    id: string
    username: string
    role: string
    display_name?: string | null
    tag?: string | null
    special_tag?: string | null
    is_telegram_linked?: boolean
    is_recently_active?: boolean
    avg_response_time?: number | null
    promoted_until?: string | null
    promotion_message?: string | null
    promotion_bid?: number | null
    vip_remaining_seconds?: number
  }
  isVip?: boolean
}>()

const auth = useAuthStore()
const onlineStore = useOnlineStore()
const { now } = useTicker()
const isMe = auth.user?.id === props.user.id

// Store the timestamp when the budget was last updated from backend
const budgetLastUpdatedAt = ref(Date.now())
watch(() => props.user.vip_remaining_seconds, () => {
  budgetLastUpdatedAt.value = Date.now()
})

const remainingTimeText = computed(() => {
  if (!props.user.promoted_until || props.user.vip_remaining_seconds === undefined) return ''
  
  let seconds = props.user.vip_remaining_seconds
  
  // If user is currently in Top 3, we simulate local tick-down for smoothness
  if (props.isVip && seconds > 0) {
    const elapsedSinceUpdate = Math.floor((now.value - budgetLastUpdatedAt.value) / 1000)
    seconds = Math.max(0, seconds - elapsedSinceUpdate)
  }
  
  return formatRemainingTime(seconds)
})

const formatResponseTime = (seconds: number) => {
  if (seconds < 60) return `${Math.round(seconds)}с`
  if (seconds < 3600) return `${Math.round(seconds / 60)}м`
  if (seconds < 86400) return `${Math.round(seconds / 3600)}ч`
  return `${Math.round(seconds / 86400)}д`
}
</script>

<template>
  <div
    class="flex items-center justify-between rounded-[10px] border p-3 transition-colors hover:bg-[#002D20]"
    :class="{ 'border-prod-yellow/50 bg-prod-yellow/5': isVip }"
  >
    <RouterLink
      :to="user.username ? `/user/@${user.username}` : `/user/${user.id}`"
      class="flex items-center gap-3 flex-1 min-w-0"
    >
      <div class="shrink-0 grid grid-cols-1 grid-rows-1 items-end justify-items-end">
        <Avatar class="size-9 col-start-1 row-start-1" :class="{ 'ring-2 ring-prod-yellow ring-offset-2 ring-offset-background': isVip }">
          <AvatarFallback class="text-xs">
            {{ (user.display_name || user.username).slice(0, 2).toUpperCase() }}
          </AvatarFallback>
        </Avatar>
        <span
          v-if="onlineStore.isOnline(user.id)"
          class="col-start-1 row-start-1 -mr-0.5 -mb-0.5 flex size-3 items-center justify-center rounded-full border-2 border-background bg-emerald-500 z-10 relative"
        />
      </div>
      <div class="min-w-0">
        <div class="flex items-center gap-1.5 min-w-0 flex-wrap">
          <p class="text-sm font-medium truncate shrink">
            {{ user.display_name || user.username }}
          </p>
          <UserTag :tag="user.tag" class="shrink-0" />
          <div v-if="user.promotion_bid" class="flex items-center gap-1 bg-prod-yellow/10 px-1 py-0.5 rounded border border-prod-yellow/20 shrink-0 relative">
            <Star v-if="isVip" class="size-2.5 text-prod-yellow fill-prod-yellow" />
            <Coin v-else class="size-2.5 text-prod-yellow" />
            <span class="text-[9px] font-bold text-prod-yellow">{{ user.promotion_bid }}</span>
            <template v-if="remainingTimeText">
              <span class="w-[1px] h-2.5 bg-prod-yellow/20 mx-0.5"></span>
              <TimerIcon class="size-2.5 text-prod-yellow/70" />
              <span class="text-[9px] font-medium text-prod-yellow/80 tabular-nums">{{ remainingTimeText }}</span>
            </template>
          </div>
          <Send v-if="user.is_telegram_linked" class="size-3 text-[#229ED9] relative" />
        </div>
        <p class="text-xs text-muted-foreground mt-1 flex items-center gap-1.5 flex-wrap">
          <span v-if="user.display_name" class="truncate">@{{ user.username }}</span>
          <span v-if="user.display_name" class="opacity-50">·</span>
          <span v-if="user.special_tag" class="text-prod-yellow truncate">{{ user.special_tag }}</span>
          <span v-else class="truncate">{{ user.role === 'admin' ? 'Админ' : 'Пользователь' }}</span>
          <template v-if="user.avg_response_time !== undefined && user.avg_response_time !== null">
            <span class="opacity-50">·</span>
            <span class="flex items-center gap-0.5 text-emerald-500">
              <Zap class="size-3 fill-emerald-500" />
              {{ formatResponseTime(user.avg_response_time) }}
            </span>
          </template>
        </p>
        <p v-if="isVip && user.promotion_message" class="text-[10px] text-prod-yellow mt-1 font-medium">
          {{ user.promotion_message }}
        </p>
      </div>
    </RouterLink>
    <HugButton v-if="!isMe" :userId="user.id" :username="user.display_name || user.username" size="sm" />
  </div>
</template>
