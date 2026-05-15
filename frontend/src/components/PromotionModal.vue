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
  // To get into top 3, we need to outbid the 3rd person if all slots are full
  // But for simplicity, let's just say min is current highest + 1 or a base price
  return 5
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
    toast.success(`VIP-статус активирован! Вы в топе за ${bid.value} монет.`)
    emit('success')
    emit('update:open', false)
    await Promise.all([auth.fetchMe(), hugsStore.fetchBalance()])
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
          Предложите самую высокую ставку, чтобы занять место в топе!
        </DialogDescription>
      </DialogHeader>

      <div class="grid gap-4 py-4">
        <div class="space-y-2">
          <Label for="bid">Ваша ставка</Label>
          <div class="relative">
            <Coin class="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-prod-yellow" />
            <Input
              id="bid"
              v-model.number="bid"
              type="number"
              class="pl-9"
              :min="minBid"
              placeholder="Введите количество монет"
            />
          </div>
          <p class="text-[10px] text-muted-foreground">Минимальная ставка: {{ minBid }} монет. Ваша позиция зависит от суммы.</p>
        </div>

        <div class="space-y-2">
          <Label for="message">Текст в профиле</Label>
          <Input
            id="message"
            v-model="message"
            placeholder="Например: Спонсорское место"
            maxlength="100"
          />
        </div>

        <div class="rounded-lg bg-muted p-3">
          <div class="flex items-center justify-between text-sm">
            <span>Ваш баланс:</span>
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
