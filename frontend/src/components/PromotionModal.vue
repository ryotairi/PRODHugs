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

const bid = ref(10)
const message = ref('')

const minBid = computed(() => {
  // If there are less than 3 VIPs, we just need the base price
  if (hugsStore.vips.length < 3) return 5
  
  // If all slots are full, we need to outbid the 3rd person
  const lowestVIPBid = hugsStore.vips[2]?.promotion_bid ?? 0
  return lowestVIPBid + 1
})

const canAfford = computed(() => {
  const balance = auth.user?.balance ?? 0
  return balance >= bid.value
})

async function handlePromote() {
  if (bid.value < minBid.value) {
    toast.error(`Минимальная ставка: ${minBid.value}`)
    return
  }
  if (!canAfford.value) return
  
  loading.value = true
  try {
    await usersApi.promote(bid.value, message.value || undefined)
    toast.success(`VIP-статус активирован! Ты в топе за ${bid.value} монет.`)
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
          Стать VIP-пользователем
        </DialogTitle>
        <DialogDescription>
          Предложи самую высокую ставку, чтобы занять место в топе! VIP-статус действует ровно 24 часа.
        </DialogDescription>
      </DialogHeader>

      <div class="grid gap-4 py-4">
        <div class="space-y-2">
          <Label for="bid">Твоя ставка</Label>
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
          <p class="text-[10px] text-muted-foreground">Минимальная ставка: {{ minBid }} монет. Твоя позиция зависит от суммы.</p>
        </div>

        <div class="rounded-lg bg-muted p-3">
          <div class="flex items-center justify-between text-sm">
            <span>Твой баланс:</span>
            <span class="font-bold text-prod-yellow">{{ auth.user?.balance ?? 0 }} монет</span>
          </div>
        </div>
        
        <p v-if="!canAfford" class="text-xs text-destructive text-center">
          Недостаточно монет
        </p>
      </div>

      <DialogFooter>
        <Button
          class="w-full bg-prod-yellow text-black hover:bg-prod-yellow/90"
          :disabled="loading || !canAfford || bid < minBid"
          @click="handlePromote"
        >
          <Loader2 v-if="loading" class="mr-2 size-4 animate-spin" />
          Перебить ставку
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
