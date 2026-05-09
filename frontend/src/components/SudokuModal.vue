<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
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
import { Loader2 } from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
  targetId: string
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  'success': [token: string]
  'failed': []
}>()

const loading = ref(true)
const captchaId = ref<string>('')
const puzzle = ref<number[][]>([])
const errors = ref(0)
const verifying = ref(false)
const hiddenInput = ref<HTMLInputElement | null>(null)

const activeRow = ref<number | null>(null)
const activeCol = ref<number | null>(null)

const isComplete = computed(() => {
  if (puzzle.value.length !== 9) return false
  for (let r = 0; r < 9; r++) {
    if (!puzzle.value[r]) return false
    for (let c = 0; c < 9; c++) {
      if (puzzle.value[r]![c] === 0) return false
    }
  }
  return true
})

watch(() => props.open, async (newVal) => {
  if (newVal) {
    await initSudoku()
  }
})

onMounted(async () => {
  if (props.open) {
    await initSudoku()
  }
})

async function initSudoku() {
  loading.value = true
  errors.value = 0
  activeRow.value = null
  activeCol.value = null
  try {
    const res = await captchaApi.getSudoku(props.targetId)
    const data = res.data as any
    if (data) {
      captchaId.value = data.id
      puzzle.value = data.puzzle
    }
  } catch (e) {
    toast.error('Ошибка загрузки судоку')
    emit('update:open', false)
  } finally {
    loading.value = false
  }
}

function handleOpenChange(val: boolean) {
  emit('update:open', val)
}

function selectCell(r: number, c: number) {
  if (puzzle.value[r]![c] !== 0) return // Already filled
  activeRow.value = r
  activeCol.value = c
  hiddenInput.value?.focus()
}

async function handleInput(e: Event) {
  const target = e.target as HTMLInputElement
  const valStr = target.value
  target.value = '' // Clear immediately

  if (activeRow.value === null || activeCol.value === null || verifying.value) return

  const val = parseInt(valStr)
  if (val >= 1 && val <= 9) {
    await verifyCell(activeRow.value, activeCol.value, val)
  }
}

async function handleKeydown(e: KeyboardEvent) {
  // Only handle if not already handled by input event or if we need specific keys
  if (e.key === 'Backspace' || e.key === 'Delete') {
    // Optional: handle clearing a cell if needed, though current logic doesn't allow it
  }
}

async function verifyCell(r: number, c: number, val: number) {
  verifying.value = true
  try {
    const res = await captchaApi.verifyCell(captchaId.value, r, c, val)
    const data = res.data as any
    if (data?.correct) {
      puzzle.value[r]![c] = val
      activeRow.value = null
      activeCol.value = null

      if (isComplete.value) {
        await completeSudoku()
      }
    } else if (data) {
      errors.value = data.errors
      toast.error('Неверная цифра!')
      if (data.failed) {
        toast.error('Эх ты, робот. Недостоен обниматься в течение 10 минут')
        emit('failed')
        emit('update:open', false)
      }
    }
  } catch (e: any) {
    if (e.response?.status === 410) {
      toast.error('Время истекло или судоку уже завершено')
      emit('update:open', false)
    }
  } finally {
    verifying.value = false
  }
}

async function completeSudoku() {
  verifying.value = true
  try {
    const res = await captchaApi.completeSudoku(captchaId.value)
    const data = res.data as any
    if (data) {
      emit('success', data.captcha_token)
    }
  } catch (e) {
    toast.error('Ошибка при завершении судоку')
  } finally {
    verifying.value = false
    emit('update:open', false)
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="handleOpenChange">
    <DialogContent class="sm:max-w-md select-none outline-none" @keydown="handleKeydown" tabindex="0">
      <DialogHeader>
        <DialogTitle>Проверка на человечность</DialogTitle>
        <DialogDescription>
          Есть подозрение на то, что ты робот. Реши судоку. Роботы точно-точно не умеют решать судоку! Ошибок: <span class="font-bold text-destructive">{{ errors }} / 3</span>
        </DialogDescription>
      </DialogHeader>

      <div v-if="loading" class="flex h-64 items-center justify-center">
        <Loader2 class="size-8 animate-spin text-muted-foreground" />
      </div>

      <div v-else class="mx-auto flex flex-col items-center gap-4 outline-none">
        <input
          ref="hiddenInput"
          type="number"
          class="absolute opacity-0 pointer-events-none"
          @input="handleInput"
        />
        <div class="grid grid-cols-9 gap-0 border-2 border-primary/50 bg-background w-max p-0.5" @keydown="handleKeydown">
          <template v-for="(row, r) in puzzle" :key="'r' + r">
            <div
              v-for="(cell, c) in row"
              :key="'c' + c"
              class="flex size-8 items-center justify-center border border-border/40 text-lg font-medium sm:size-10 cursor-pointer transition-colors"
              :class="{
                'border-r-2 border-r-primary/50': c === 2 || c === 5,
                'border-b-2 border-b-primary/50': r === 2 || r === 5,
                'bg-muted text-muted-foreground cursor-default': cell !== 0 && (activeRow !== r || activeCol !== c),
                'bg-primary/20 text-primary font-bold': cell !== 0 && (activeRow === r && activeCol === c), // Should not happen directly via clicks but conceptually
                'bg-background hover:bg-muted/50': cell === 0,
                'bg-prod-yellow/20 ring-2 ring-prod-yellow ring-inset z-10': activeRow === r && activeCol === c,
              }"
              @click="selectCell(r, c)"
            >
              {{ cell !== 0 ? cell : '' }}
            </div>
          </template>
        </div>

        <p class="text-xs text-muted-foreground text-center">
          Нажми на пустую клетку и введи цифру (1-9) на клавиатуре.
        </p>
      </div>
    </DialogContent>
  </Dialog>
</template>
