<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { captchaApi } from '@/api/client'
import { toast } from 'vue-sonner'
import { Loader2, Coins } from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  'success': [token: string]
  'failed': []
}>()

const loading = ref(true)
const captchaId = ref<string>('')
const spinning = ref(false)
const rotation = ref(0)
const resultWin = ref<boolean | null>(null)

watch(() => props.open, async (newVal) => {
  if (newVal) {
    await initCasino()
  }
})

onMounted(async () => {
  if (props.open) {
    await initCasino()
  }
})

async function initCasino() {
  loading.value = true
  resultWin.value = null
  rotation.value = 0
  try {
    const res = await captchaApi.getCasino()
    captchaId.value = res.data.id
  } catch {
    toast.error('Ошибка инициализации казино')
    emit('update:open', false)
  } finally {
    loading.value = false
  }
}

async function spin() {
  if (spinning.value) return
  spinning.value = true
  
  try {
    const res = await captchaApi.spinCasino(captchaId.value)
    const { win, captcha_token } = res.data
    
    // Calculate rotation
    // 4 segments: 0-90 (loss), 90-180 (loss), 180-270 (loss), 270-360 (win)
    // Centers: 45, 135, 225, 315
    const baseSpins = 5 + Math.floor(Math.random() * 5)
    let targetDegree = 0 // Rotation needed to bring segment center to 0deg (top)
    
    if (win) {
      // WIN segment center is 315deg. To bring it to top: 360 - 315 = 45deg
      targetDegree = 45
    } else {
      // LOSE segment centers: 45, 135, 225. 
      // To bring them to top: 315, 225, 135
      const lossRotations = [135, 225, 315]
      targetDegree = lossRotations[Math.floor(Math.random() * lossRotations.length)]!
    }
    
    rotation.value = baseSpins * 360 + targetDegree
    
    setTimeout(() => {
      spinning.value = false
      resultWin.value = win
      if (win && captcha_token) {
        toast.success('Победа! Вы доказали, что вы не робот (или очень везучий робот)')
        emit('success', captcha_token)
        setTimeout(() => {
          emit('update:open', false)
        }, 1500)
      } else {
        toast.error('Лудоман! Обнимашки запрещены на 10 минут')
        emit('failed')
        setTimeout(() => {
          emit('update:open', false)
        }, 2000)
      }
    }, 4000)
    
  } catch (e: unknown) {
    spinning.value = false
    const err = e as { response?: { status?: number } }
    if (err.response?.status === 410) {
      toast.error('Сессия истекла')
    } else {
      toast.error('Ошибка при крутке')
    }
    emit('update:open', false)
  }
}

function handleOpenChange(val: boolean) {
  if (!spinning.value) {
    emit('update:open', val)
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="handleOpenChange">
    <DialogContent class="sm:max-w-md select-none">
      <DialogHeader>
        <DialogTitle>Испытай удачу</DialogTitle>
        <DialogDescription>
          Чтобы обниматься, нужно доказать, что фортуна на твоей стороне.
          Шанс 1 к 4. Проигрыш = бан на обнимашки на 10 минут.
        </DialogDescription>
      </DialogHeader>

      <div v-if="loading" class="flex h-64 items-center justify-center">
        <Loader2 class="size-8 animate-spin text-muted-foreground" />
      </div>

      <div v-else class="flex flex-col items-center gap-8 py-4">
        <div class="relative size-64">
          <!-- Pointer -->
          <div class="absolute -top-4 left-1/2 z-10 -translate-x-1/2 text-prod-yellow">
             <div class="size-0 border-l-[12px] border-r-[12px] border-t-[20px] border-l-transparent border-r-transparent border-t-current" />
          </div>
          
          <!-- Wheel -->
          <div 
            class="size-full rounded-full border-4 border-muted bg-muted shadow-xl transition-transform duration-[4000ms] cubic-bezier(0.15, 0, 0.15, 1)"
            :style="{ transform: `rotate(${rotation}deg)` }"
          >
            <div class="relative size-full overflow-hidden rounded-full">
              <!-- Segments -->
              <div class="absolute inset-0 bg-red-500/20" style="clip-path: polygon(50% 50%, 50% 0, 100% 0, 100% 50%)" />
              <div class="absolute inset-0 bg-red-500/40" style="clip-path: polygon(50% 50%, 100% 50%, 100% 100%, 50% 100%)" />
              <div class="absolute inset-0 bg-red-500/20" style="clip-path: polygon(50% 50%, 50% 100%, 0 100%, 0 50%)" />
              <div class="absolute inset-0 bg-emerald-500/40" style="clip-path: polygon(50% 50%, 0 50%, 0 0, 50% 0)" />
              
              <!-- Text labels -->
              <div class="absolute inset-0">
                <!-- WIN segment center at 315deg -->
                <div class="absolute inset-0 rotate-[315deg]">
                  <span class="absolute top-10 left-1/2 -translate-x-1/2 font-bold text-emerald-500">WIN</span>
                </div>
                <!-- LOSE segment centers at 45deg, 135deg, 225deg -->
                <div class="absolute inset-0 rotate-[45deg]">
                  <span class="absolute top-10 left-1/2 -translate-x-1/2 font-bold text-red-500/60">LOSE</span>
                </div>
                <div class="absolute inset-0 rotate-[135deg]">
                  <span class="absolute top-10 left-1/2 -translate-x-1/2 font-bold text-red-500/60">LOSE</span>
                </div>
                <div class="absolute inset-0 rotate-[225deg]">
                  <span class="absolute top-10 left-1/2 -translate-x-1/2 font-bold text-red-500/60">LOSE</span>
                </div>
              </div>
              
              <!-- Center hole -->
              <div class="absolute left-1/2 top-1/2 size-12 -translate-x-1/2 -translate-y-1/2 rounded-full border-4 border-muted bg-background flex items-center justify-center">
                <Coins class="size-6 text-prod-yellow" />
              </div>
            </div>
          </div>
        </div>

        <Button 
          variant="yellow" 
          class="w-full rounded-[21px] py-6 text-lg font-bold" 
          :disabled="spinning || resultWin !== null"
          @click="spin"
        >
          <template v-if="spinning">Крутим...</template>
          <template v-else-if="resultWin === true">ПОБЕДА!</template>
          <template v-else-if="resultWin === false">ЛУЗЕР :(</template>
          <template v-else>КРУТИТЬ КОЛЕСО</template>
        </Button>
      </div>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
.cubic-bezier {
  transition-timing-function: cubic-bezier(0.15, 0, 0.15, 1);
}
</style>
