<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { Heart, Clock, Loader2, Hourglass, ChevronDown, MessageSquare } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { useAuthStore } from '@/stores/auth'
import { useHugsStore, type CooldownInfo, type IntimacyInfo, type HugType } from '@/stores/hugs'
import { suggestVerb, hugTypeLabel } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import SudokuModal from '@/components/SudokuModal.vue'

const props = defineProps<{
  userId: string
  username: string
  size?: 'default' | 'sm' | 'lg'
}>()

const emit = defineEmits<{
  hugged: []
}>()

const auth = useAuthStore()
const hugsStore = useHugsStore()
const loading = ref(false)
const cooldown = ref<CooldownInfo | null>(null)
const remaining = ref(0)
const sudokuRemaining = ref(0)
const btnRef = ref<HTMLButtonElement | null>(null)
const intimacy = ref<IntimacyInfo | null>(null)
let timer: ReturnType<typeof setInterval> | null = null

// Comment dialog state
const showCommentDialog = ref(false)
const commentText = ref('')
const pendingHugType = ref<string | undefined>(undefined)

// Sudoku state
const showSudokuModal = ref(false)
const cachedComment = ref<string | undefined>(undefined)

const totalRemaining = computed(() => Math.max(remaining.value, sudokuRemaining.value))

const availableHugTypes = computed<HugType[]>(() => {
  if (!intimacy.value) return ['standard']
  return intimacy.value.available_hug_types
})
const hasMultipleTypes = computed(() => availableHugTypes.value.length > 1)

// Comment cost based on intimacy tier bonus coins + 1 base
const commentCost = computed(() => {
  const bonus = intimacy.value?.bonus_coins ?? 0
  return 1 + bonus
})

// Pending state computeds
const allSlotsFull = computed(
  () => hugsStore.outgoingHugs.length >= hugsStore.slotInfo.total_slots,
)
const isPendingWithThisUser = computed(
  () => hugsStore.outgoingHugs.some((h) => h.receiver_id === props.userId),
)
const hasIncomingPending = computed(() => hugsStore.inbox.some((h) => h.giver_id === props.userId))

const isDisabled = computed(
  () =>
    loading.value ||
    totalRemaining.value > 0 ||
    allSlotsFull.value ||
    hasIncomingPending.value ||
    isPendingWithThisUser.value,
)

const buttonVariant = computed(() => {
  if (
    totalRemaining.value > 0 ||
    allSlotsFull.value ||
    hasIncomingPending.value ||
    isPendingWithThisUser.value
  )
    return 'secondary'
  return 'yellow'
})

async function loadCooldown() {
  try {
    cooldown.value = await hugsStore.getCooldown(props.userId)
    remaining.value = cooldown.value.remaining_seconds
    updateSudokuRemaining()
    startTimer()
  } catch {
    // Ignore
  }
  try {
    intimacy.value = await hugsStore.getPairIntimacy(props.userId)
  } catch {
    // Ignore — defaults to standard only
  }
}

function updateSudokuRemaining() {
  if (!auth.user?.sudoku_cooldown_until) {
    sudokuRemaining.value = 0
    return
  }
  const end = new Date(auth.user.sudoku_cooldown_until).getTime()
  const now = Date.now()
  sudokuRemaining.value = Math.max(0, Math.floor((end - now) / 1000))
}

function startTimer() {
  if (timer) clearInterval(timer)
  timer = setInterval(() => {
    if (remaining.value > 0) remaining.value--
    updateSudokuRemaining()
    
    if (remaining.value <= 0 && sudokuRemaining.value <= 0) {
      remaining.value = 0
      sudokuRemaining.value = 0
      if (timer) clearInterval(timer)
    }
  }, 1000)
}

let suggesting = false

function openCommentDialog(hugType?: string) {
  pendingHugType.value = hugType
  commentText.value = ''
  showCommentDialog.value = true
}

async function suggest(hugType?: string, comment?: string, captchaToken?: string) {
  if (suggesting || isDisabled.value) return
  
  if (auth.user?.requires_sudoku && !captchaToken) {
    pendingHugType.value = hugType
    cachedComment.value = comment
    showSudokuModal.value = true
    return
  }
  
  suggesting = true
  loading.value = true
  showCommentDialog.value = false
  try {
    await hugsStore.suggestHug(props.userId, hugType, comment, captchaToken)
    toast.success(`Ты ${suggestVerb(auth.user?.gender)} обнимашку ${props.username}!`)
    emit('hugged')
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string, code?: string } } }
    if (err.response?.data?.code === 'CAPTCHA_REQUIRED') {
      pendingHugType.value = hugType
      cachedComment.value = comment
      showSudokuModal.value = true
      return
    }
    toast.error(err.response?.data?.message || `Не удалось предложить обнимашку ${props.username}`)
  } finally {
    loading.value = false
    suggesting = false
  }
}

function sendWithComment() {
  const comment = commentText.value.trim() || undefined
  suggest(pendingHugType.value, comment)
}

function handleSudokuSuccess(token: string) {
  suggest(pendingHugType.value, cachedComment.value, token)
}

async function handleSudokuFailed() {
  // Cooldown is set by backend, we should refresh auth user
  await auth.fetchMe()
  updateSudokuRemaining()
  startTimer()
}

function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m}:${s.toString().padStart(2, '0')}`
}

watch(
  () => auth.user?.sudoku_cooldown_until,
  () => {
    updateSudokuRemaining()
    if (sudokuRemaining.value > 0) {
      startTimer()
    }
  },
)

onMounted(loadCooldown)
onUnmounted(() => {
  if (timer) clearInterval(timer)
})

watch(
  () => hugsStore.cooldownRefreshes[props.userId],
  (newVal, oldVal) => {
    if (newVal && newVal !== oldVal) {
      loadCooldown()
    }
  },
)
</script>

<template>
  <div ref="btnRef" class="relative inline-flex">
    <!-- Disabled states: single non-interactive button -->
    <Button
      v-if="isDisabled"
      :disabled="true"
      :size="size ?? 'default'"
      :variant="buttonVariant"
      class="rounded-[21px]"
    >
      <Loader2 v-if="loading" class="size-4 animate-spin" />
      <Clock v-else-if="totalRemaining > 0" class="size-4" />
      <Hourglass v-else-if="allSlotsFull || hasIncomingPending" class="size-4" />
      <Heart v-else class="size-4" />
      <span v-if="totalRemaining > 0">{{ formatTime(totalRemaining) }}</span>
      <span v-else-if="isPendingWithThisUser">Ожидание...</span>
      <span v-else-if="hasIncomingPending">Ждет твоего ответа</span>
      <span v-else-if="allSlotsFull">Все слоты заняты</span>
      <span v-else>Обняться</span>
    </Button>

    <!-- Active state: split button — main sends standard, dropdown has types + comment -->
    <div v-else class="inline-flex">
      <Button
        @click="suggest()"
        :size="size ?? 'default'"
        :variant="buttonVariant"
        class="rounded-l-[21px] rounded-r-none"
      >
        <Heart class="size-4" />
        <span>Обняться</span>
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button
            :size="size ?? 'default'"
            :variant="buttonVariant"
            class="rounded-l-none rounded-r-[21px] border-l border-background/20 px-1.5"
          >
            <ChevronDown class="size-3.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-48">
          <!-- Hug types -->
          <DropdownMenuItem
            v-for="ht in availableHugTypes"
            :key="ht"
            @click="suggest(ht)"
          >
            {{ hugTypeLabel(ht) }}
          </DropdownMenuItem>

          <DropdownMenuSeparator />

          <!-- Comment option — opens dialog for standard type -->
          <DropdownMenuItem @click="openCommentDialog()">
            <MessageSquare class="size-4" />
            С комментарием
          </DropdownMenuItem>

          <!-- Comment + type options (if multiple types unlocked) -->
          <template v-if="hasMultipleTypes">
            <DropdownMenuItem
              v-for="ht in availableHugTypes"
              :key="`comment-${ht}`"
              @click="openCommentDialog(ht)"
              class="text-muted-foreground"
            >
              <MessageSquare class="size-4" />
              {{ hugTypeLabel(ht) }} + комм.
            </DropdownMenuItem>
          </template>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>

    <!-- Comment dialog -->
    <Dialog v-model:open="showCommentDialog">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Комментарий к обнимашке</DialogTitle>
          <DialogDescription>
            Виден только тебе и получателю. Стоимость:
            {{ commentCost }} {{ commentCost === 1 ? 'монета' : 'монеты' }} (при принятии).
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-2">
          <Textarea
            v-model="commentText"
            placeholder="Напиши что-нибудь приятное..."
            :maxlength="140"
            class="resize-none"
            rows="3"
          />
          <div class="text-right text-xs text-muted-foreground">
            {{ commentText.length }}/140
          </div>
        </div>
        <DialogFooter>
          <Button
            variant="yellow"
            class="w-full rounded-[21px]"
            :disabled="commentText.trim().length === 0"
            @click="sendWithComment"
          >
            <MessageSquare class="size-4" />
            Отправить
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <SudokuModal 
      v-model:open="showSudokuModal" 
      :target-id="props.userId"
      @success="handleSudokuSuccess" 
      @failed="handleSudokuFailed" 
    />
  </div>
</template>
