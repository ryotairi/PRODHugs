<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { usersApi } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { useHugsStore } from '@/stores/hugs'
import { toast } from 'vue-sonner'
import { Loader2, Star, Coins as Coin } from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'success'): void
}>()

const auth = useAuthStore()
const hugsStore = useHugsStore()
const loading = ref(false)

const { now } = useTicker()
const bid = ref(10)
const message = ref('')

const isMePromoted = computed(() => {
  if (!auth.user?.promoted_until) return false
  return new Date(auth.user.promoted_until) > now.value
})

const minBid = computed(() => {
  const vips = hugsStore.vips
  // If there are less than 3 VIPs, we just need the base price
  if (vips.length < 3) return 5
  
  // If I am already a VIP, I need to outbid the 3rd person to STAY in top 3
  // but usually I want to outbid the person ABOVE me or just add to my bid.
  // The system-wide minimum to be in TOP-3 is outbidding the current 3rd place.
  const lowestVIPBid = vips[2]?.promotion_bid ?? 0
  return lowestVIPBid + 1
})

const cost = computed(() => {
  const currentBid = isMePromoted.value ? (auth.user?.promotion_bid ?? 0) : 0
  const diff = (bid.value || 0) - currentBid
  return Math.max(0, diff)
})

const canAfford = computed(() => {
  const balance = auth.user?.balance ?? 0
  return balance >= cost.value
})

// Initialize bid when modal opens
watch(() => props.open, (isOpen) => {
  if (isOpen) {
    const current = auth.user?.promotion_bid ?? 0
    // Default to either current bid + 5 or minBid
    bid.value = Math.max(minBid.value, current + 5)
    message.value = auth.user?.promotion_message ?? ''
  }
})

async function handlePromote() {
  const finalBid = bid.value || 0
  if (finalBid < minBid.value) {
    toast.error(`Минимальная ставка: ${minBid.value}`)
    return
  }
  if (!canAfford.value) {
    toast.error('Недостаточно монет')
    return
  }
  
  loading.value = true
  try {
    await usersApi.promote(finalBid, message.value || undefined)
    toast.success(`VIP-статус активирован! Ты в топе со ставкой ${finalBid} монет.`)
    emit('success')
    emit('update:open', false)
    await Promise.all([
      auth.fetchMe(), 
      hugsStore.fetchBalance(),
      hugsStore.fetchVIPs()
    ])
  } catch (error: any) {
    toast.error('Ошибка', {
      description: error.response?.data?.message || 'Не удалось активировать VIP',
    })
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-[425px]">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2">
          <Star class="size-5 text-prod-yellow fill-prod-yellow" />
          {{ isMePromoted ? 'Управление VIP' : 'Стать VIP-пользователем' }}
        </DialogTitle>
        <DialogDescription>
          Предложи самую высокую ставку, чтобы занять место в топе! VIP-статус действует 24 часа.
        </DialogDescription>
      </DialogHeader>

      <div class="grid gap-4 py-4">
        <div class="space-y-2">
          <Label for="bid">Твоя новая ставка</Label>
          <div class="flex items-center gap-2 rounded-md border bg-background px-3 focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2">
            <Coin class="size-4 shrink-0 text-prod-yellow" />
            <input
              id="bid"
              v-model.number="bid"
              type="number"
              class="flex h-9 w-full bg-transparent py-1 text-sm outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50"
              :min="minBid"
              placeholder="Введите количество монет"
            />
          </div>
          <p class="text-[10px] text-muted-foreground">Минимальная ставка сейчас: {{ minBid }} монет. Твоя позиция зависит от суммы.</p>
        </div>

        <div class="space-y-2">
          <Label for="message">Твоё сообщение (необязательно)</Label>
          <Input 
            id="message" 
            v-model="message" 
            placeholder="Я самый лучший!" 
            maxlength="100"
          />
        </div>

        <div class="rounded-lg bg-muted p-3 space-y-2">
          <div v-if="isMePromoted" class="flex items-center justify-between text-xs">
            <span class="text-muted-foreground">Твоя текущая ставка:</span>
            <span class="font-bold text-muted-foreground">{{ auth.user?.promotion_bid }} монет</span>
          </div>
          <div class="flex items-center justify-between text-sm">
            <span class="text-muted-foreground">К оплате (доплата):</span>
            <span class="font-bold" :class="cost > 0 ? 'text-prod-yellow' : 'text-emerald-500'">{{ cost }} монет</span>
          </div>
          <div class="flex items-center justify-between text-sm">
            <span class="text-muted-foreground">Твой баланс:</span>
            <span class="font-bold">{{ auth.user?.balance ?? 0 }} монет</span>
          </div>
        </div>
        
        <p v-if="!canAfford" class="text-xs text-destructive text-center font-medium">
          Недостаточно монет для этой ставки
        </p>
      </div>

      <DialogFooter>
        <Button
          class="w-full bg-prod-yellow text-black hover:bg-prod-yellow/90"
          :disabled="loading || !canAfford || (bid || 0) < minBid"
          @click="handlePromote"
        >
          <Loader2 v-if="loading" class="mr-2 size-4 animate-spin" />
          {{ isMePromoted ? 'Обновить ставку' : 'Занять место' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
