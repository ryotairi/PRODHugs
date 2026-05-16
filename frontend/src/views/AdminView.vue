<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import {
  Users,
  ShieldBan,
  Search,
  MoreHorizontal,
  Ban,
  ShieldCheck,
  UserPen,
  KeyRound,
  Venus,
  Coins,
  Tag,
  Hash,
  Trash2,
  Radio,
  Megaphone,
  Puzzle,
  Star,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { announcementsApi, adminApi } from '@/api/client'
import { useAdminStore, type AdminUser } from '@/stores/admin'
import { useOnlineStore } from '@/stores/online'
import { useWebSocket } from '@/composables/useWebSocket'
import { parseBackendError, type FieldError } from '@/lib/validation'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
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
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import UserTag from '@/components/UserTag.vue'

const admin = useAdminStore()
const onlineStore = useOnlineStore()
const loading = ref(true)

// ── Sorted users: online first, backend order preserved within each group ──
const sortedUsers = computed(() =>
  [...admin.users].sort((a, b) => {
    const aOnline = onlineStore.isOnline(a.id) ? 0 : 1
    const bOnline = onlineStore.isOnline(b.id) ? 0 : 1
    return aOnline - bOnline
  }),
)

// ── Live online count ──
const onlineCount = ref<number | null>(null)
const { on } = useWebSocket()
const unsubOnline = on<{ count: number }>('online_count', (data) => {
  onlineCount.value = data.count
})

// ── Announcement management ──
const announcementMessage = ref('')
const currentAnnouncement = ref<{ id: string; message: string; created_at: string } | null>(null)
const publishingAnnouncement = ref(false)

async function fetchCurrentAnnouncement() {
  try {
    const res = await announcementsApi.getActive()
    currentAnnouncement.value = res.data ?? null
  } catch {
    currentAnnouncement.value = null
  }
}

async function publishAnnouncement() {
  if (!announcementMessage.value.trim()) return
  publishingAnnouncement.value = true
  try {
    await adminApi.createAnnouncement(announcementMessage.value.trim())
    announcementMessage.value = ''
    await fetchCurrentAnnouncement()
    toast.success('Объявление опубликовано')
  } catch (e) {
    const parsed = parseBackendError(e)
    toast.error(parsed.generalError ?? 'Ошибка публикации')
  } finally {
    publishingAnnouncement.value = false
  }
}

async function removeAnnouncement() {
  if (!currentAnnouncement.value) return
  try {
    await adminApi.deleteAnnouncement(currentAnnouncement.value.id)
    currentAnnouncement.value = null
    toast.success('Объявление удалено')
  } catch (e) {
    const parsed = parseBackendError(e)
    toast.error(parsed.generalError ?? 'Ошибка удаления')
  }
}

// ── Infinite scroll ──
const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

onMounted(async () => {
  await Promise.all([admin.fetchStats(), admin.fetchUsers(), fetchCurrentAnnouncement()])
  loading.value = false

  await nextTick()

  observer = new IntersectionObserver(
    (entries) => {
      if (entries[0]?.isIntersecting && admin.hasMore && !admin.loadingMore) {
        admin.loadMore()
      }
    },
    { rootMargin: '200px' },
  )
  if (sentinel.value) observer.observe(sentinel.value)
})

// ── Search ──
const adminSearchQuery = ref('')
let searchDebounce: ReturnType<typeof setTimeout> | null = null

watch(adminSearchQuery, (q) => {
  if (searchDebounce) clearTimeout(searchDebounce)
  searchDebounce = setTimeout(async () => {
    loading.value = true
    await admin.fetchUsers(q)
    loading.value = false
  }, 300)
})

onUnmounted(() => {
  observer?.disconnect()
  unsubOnline()
  if (searchDebounce) clearTimeout(searchDebounce)
})

// ── Dialogs ──
const editingUser = ref<AdminUser | null>(null)

// Username dialog
const usernameDialogOpen = ref(false)
const newUsername = ref('')
const savingUsername = ref(false)
const usernameError = ref('')

function openUsernameDialog(user: AdminUser) {
  editingUser.value = user
  newUsername.value = user.username
  usernameError.value = ''
  usernameDialogOpen.value = true
}

async function saveUsername() {
  if (!editingUser.value || !newUsername.value.trim()) return
  savingUsername.value = true
  usernameError.value = ''
  try {
    await admin.updateUsername(editingUser.value.id, newUsername.value.trim())
    toast.success('Имя пользователя изменено')
    usernameDialogOpen.value = false
  } catch (e) {
    const parsed = parseBackendError(e)
    usernameError.value = parsed.generalError ?? 'Ошибка сохранения'
  } finally {
    savingUsername.value = false
  }
}

// Gender dialog
const genderDialogOpen = ref(false)
const newGender = ref<string>('')
const savingGender = ref(false)

function openGenderDialog(user: AdminUser) {
  editingUser.value = user
  newGender.value = user.gender ?? ''
  genderDialogOpen.value = true
}

async function saveGender() {
  if (!editingUser.value) return
  savingGender.value = true
  try {
    await admin.updateGender(editingUser.value.id, newGender.value || null)
    toast.success('Пол изменён')
    genderDialogOpen.value = false
  } catch {
    toast.error('Ошибка сохранения')
  } finally {
    savingGender.value = false
  }
}

// Password dialog
const passwordDialogOpen = ref(false)
const newPassword = ref('')
const newPasswordConfirm = ref('')
const savingPassword = ref(false)
const passwordErrors = ref<FieldError[]>([])
const passwordServerError = ref('')

function openPasswordDialog(user: AdminUser) {
  editingUser.value = user
  newPassword.value = ''
  newPasswordConfirm.value = ''
  passwordErrors.value = []
  passwordServerError.value = ''
  passwordDialogOpen.value = true
}

function passwordErrorFor(field: string): string | undefined {
  return passwordErrors.value.find((e) => e.field === field)?.message
}

async function savePassword() {
  passwordErrors.value = []
  passwordServerError.value = ''

  if (!newPassword.value) {
    passwordErrors.value.push({ field: 'newPassword', message: 'Введите пароль' })
  } else if (newPassword.value.length < 8) {
    passwordErrors.value.push({ field: 'newPassword', message: 'Минимум 8 символов' })
  }
  if (newPassword.value !== newPasswordConfirm.value) {
    passwordErrors.value.push({ field: 'newPasswordConfirm', message: 'Пароли не совпадают' })
  }
  if (passwordErrors.value.length > 0) return

  if (!editingUser.value) return
  savingPassword.value = true
  try {
    await admin.updatePassword(editingUser.value.id, newPassword.value)
    toast.success('Пароль изменён')
    passwordDialogOpen.value = false
  } catch (e) {
    const parsed = parseBackendError(e)
    passwordServerError.value = parsed.generalError ?? 'Ошибка сохранения'
  } finally {
    savingPassword.value = false
  }
}

// Balance dialog
const balanceDialogOpen = ref(false)
const newBalance = ref(0)
const savingBalance = ref(false)
const balanceError = ref('')

function openBalanceDialog(user: AdminUser) {
  editingUser.value = user
  newBalance.value = user.balance
  balanceError.value = ''
  balanceDialogOpen.value = true
}

async function saveBalance() {
  if (!editingUser.value) return
  if (newBalance.value < 0) {
    balanceError.value = 'Сумма не может быть отрицательной'
    return
  }
  savingBalance.value = true
  balanceError.value = ''
  try {
    await admin.updateBalance(editingUser.value.id, newBalance.value)
    toast.success('Баланс изменён')
    balanceDialogOpen.value = false
  } catch (e) {
    const parsed = parseBackendError(e)
    balanceError.value = parsed.generalError ?? 'Ошибка сохранения'
  } finally {
    savingBalance.value = false
  }
}

// Display name dialog
const displayNameDialogOpen = ref(false)
const newDisplayName = ref('')
const savingDisplayName = ref(false)
const displayNameError = ref('')

function openDisplayNameDialog(user: AdminUser) {
  editingUser.value = user
  newDisplayName.value = user.display_name ?? ''
  displayNameError.value = ''
  displayNameDialogOpen.value = true
}

async function saveDisplayName() {
  if (!editingUser.value) return
  savingDisplayName.value = true
  displayNameError.value = ''
  try {
    const value = newDisplayName.value.trim() || null
    await admin.updateDisplayName(editingUser.value.id, value)
    toast.success('Отображаемое имя изменено')
    displayNameDialogOpen.value = false
  } catch (e) {
    const parsed = parseBackendError(e)
    displayNameError.value = parsed.generalError ?? 'Ошибка сохранения'
  } finally {
    savingDisplayName.value = false
  }
}

// Tag dialog
const tagDialogOpen = ref(false)
const newTag = ref('')
const savingTag = ref(false)
const tagError = ref('')

function openTagDialog(user: AdminUser) {
  editingUser.value = user
  newTag.value = user.tag ?? ''
  tagError.value = ''
  tagDialogOpen.value = true
}

async function saveTag() {
  if (!editingUser.value) return
  savingTag.value = true
  tagError.value = ''
  try {
    const value = newTag.value.trim() || null
    await admin.updateTag(editingUser.value.id, value)
    toast.success('Тег изменён')
    tagDialogOpen.value = false
  } catch (e) {
    const parsed = parseBackendError(e)
    tagError.value = parsed.generalError ?? 'Ошибка сохранения'
  } finally {
    savingTag.value = false
  }
}

// Special tag dialog
const specialTagDialogOpen = ref(false)
const newSpecialTag = ref('')
const savingSpecialTag = ref(false)
const specialTagError = ref('')

// Captcha type dialog
const captchaTypeDialogOpen = ref(false)
const newCaptchaType = ref<AdminUser['captcha_type']>('none')
const savingCaptchaType = ref(false)

function openCaptchaTypeDialog(user: AdminUser) {
  editingUser.value = user
  newCaptchaType.value = user.captcha_type
  captchaTypeDialogOpen.value = true
}

async function saveCaptchaType() {
  if (!editingUser.value) return
  savingCaptchaType.value = true
  try {
    await admin.updateCaptchaType(editingUser.value.id, newCaptchaType.value)
    toast.success('Тип капчи изменён')
    captchaTypeDialogOpen.value = false
  } catch {
    toast.error('Ошибка сохранения')
  } finally {
    savingCaptchaType.value = false
  }
}

function openSpecialTagDialog(user: AdminUser) {
  editingUser.value = user
  newSpecialTag.value = user.special_tag ?? ''
  specialTagError.value = ''
  specialTagDialogOpen.value = true
}

async function saveSpecialTag() {
  if (!editingUser.value) return
  savingSpecialTag.value = true
  specialTagError.value = ''
  try {
    const value = newSpecialTag.value.trim() || null
    await admin.updateSpecialTag(editingUser.value.id, value)
    toast.success('Спецтег изменён')
    specialTagDialogOpen.value = false
  } catch (e) {
    const parsed = parseBackendError(e)
    specialTagError.value = parsed.generalError ?? 'Ошибка сохранения'
  } finally {
    savingSpecialTag.value = false
  }
}

async function handleClearPromotion(user: AdminUser) {
  try {
    await admin.clearPromotion(user.id)
    toast.success(`Продвижение для ${user.username} удалено`)
  } catch {
    toast.error('Ошибка при удалении продвижения')
  }
}

// ── Ban / Unban ──
async function toggleBan(user: AdminUser) {
  try {
    if (user.banned_at) {
      await admin.unbanUser(user.id)
      toast.success(`${user.username} разблокирован`)
    } else {
      await admin.banUser(user.id)
      toast.success(`${user.username} заблокирован`)
    }
  } catch (e) {
    const parsed = parseBackendError(e)
    toast.error(parsed.generalError ?? 'Ошибка')
  }
}

// Delete dialog
const deleteDialogOpen = ref(false)
const deleteConfirmUsername = ref('')
const deletingUser = ref(false)

function openDeleteDialog(user: AdminUser) {
  editingUser.value = user
  deleteConfirmUsername.value = ''
  deletingUser.value = false
  deleteDialogOpen.value = true
}

async function confirmDeleteUser() {
  if (!editingUser.value) return
  if (deleteConfirmUsername.value !== editingUser.value.username) return
  deletingUser.value = true
  try {
    await admin.deleteUser(editingUser.value.id)
    toast.success(`Пользователь ${editingUser.value.username} удалён`)
    deleteDialogOpen.value = false
  } catch (e) {
    const parsed = parseBackendError(e)
    toast.error(parsed.generalError ?? 'Ошибка удаления')
  } finally {
    deletingUser.value = false
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('ru-RU', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<template>
  <div class="mx-auto max-w-3xl space-y-6">
    <!-- Header -->
    <div>
      <h1 class="text-2xl font-semibold tracking-tight">Панель администратора</h1>
      <p class="text-muted-foreground">Управление пользователями</p>
    </div>

    <!-- Announcement management -->
    <Card>
      <CardHeader class="pb-3">
        <CardTitle class="flex items-center gap-2 text-base">
          <Megaphone class="size-5 text-prod-yellow" />
          Объявление
        </CardTitle>
      </CardHeader>
      <CardContent class="space-y-3">
        <div
          v-if="currentAnnouncement"
          class="flex items-start justify-between gap-3 rounded-lg border p-3"
        >
          <p class="text-sm whitespace-pre-wrap">{{ currentAnnouncement.message }}</p>
          <Button variant="ghost" size="sm" class="shrink-0 text-destructive" @click="removeAnnouncement">
            Удалить
          </Button>
        </div>
        <Textarea
          v-model="announcementMessage"
          placeholder="Текст объявления..."
          class="min-h-20 resize-none"
        />
        <Button
          variant="yellow"
          class="w-full rounded-[21px]"
          :disabled="publishingAnnouncement || !announcementMessage.trim()"
          @click="publishAnnouncement"
        >
          {{ publishingAnnouncement ? 'Публикация...' : 'Опубликовать' }}
        </Button>
      </CardContent>
    </Card>

    <!-- Stats cards -->
    <div v-if="loading" class="grid grid-cols-2 gap-4 sm:grid-cols-3">
      <Skeleton class="h-24 rounded-lg" />
      <Skeleton class="h-24 rounded-lg" />
      <Skeleton class="h-24 rounded-lg" />
    </div>
    <div v-else class="grid grid-cols-2 gap-4 sm:grid-cols-3">
      <Card>
        <CardHeader class="pb-2">
          <CardTitle class="text-sm font-medium text-muted-foreground"
            >Всего пользователей</CardTitle
          >
        </CardHeader>
        <CardContent>
          <div class="flex items-center gap-2">
            <Users class="size-5 text-prod-yellow" />
            <span class="text-2xl font-bold tabular-nums">{{ admin.stats?.total_users ?? 0 }}</span>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader class="pb-2">
          <CardTitle class="text-sm font-medium text-muted-foreground">Заблокировано</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="flex items-center gap-2">
            <ShieldBan class="size-5 text-destructive" />
            <span class="text-2xl font-bold tabular-nums">{{
              admin.stats?.banned_users ?? 0
            }}</span>
          </div>
        </CardContent>
      </Card>
      <Card class="col-span-2 sm:col-span-1">
        <CardHeader class="pb-2">
          <CardTitle class="text-sm font-medium text-muted-foreground">Сейчас онлайн</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="flex items-center gap-2">
            <Radio class="size-5 text-emerald-500" />
            <span class="text-2xl font-bold tabular-nums">{{ onlineCount ?? '—' }}</span>
            <span
              v-if="onlineCount != null"
              class="relative flex size-2"
            >
              <span
                class="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400 opacity-75"
              />
              <span class="relative inline-flex size-2 rounded-full bg-emerald-500" />
            </span>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Users list -->
    <div>
      <h2 class="mb-3 text-lg font-medium">Пользователи</h2>

      <div class="relative mb-3">
        <Search class="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          v-model="adminSearchQuery"
          type="text"
          class="pl-9"
          placeholder="Поиск по имени..."
          maxlength="64"
        />
      </div>

      <div v-if="loading" class="space-y-3">
        <Skeleton v-for="i in 8" :key="i" class="h-16 w-full rounded-lg" />
      </div>

      <div v-else class="space-y-2">
        <TransitionGroup name="user-list" tag="div" class="space-y-2">
          <div
            v-for="user in sortedUsers"
            :key="user.id"
            class="flex items-center justify-between rounded-[10px] border p-3 transition-colors hover:bg-accent/50"
          >
          <div class="flex items-center gap-3 min-w-0 flex-1">
            <div class="relative shrink-0">
              <Avatar class="size-9">
                <AvatarFallback class="text-xs">
                  {{ (user.display_name || user.username).slice(0, 2).toUpperCase() }}
                </AvatarFallback>
              </Avatar>
              <span
                v-if="onlineStore.isOnline(user.id)"
                class="absolute -bottom-0.5 -right-0.5 flex size-3 items-center justify-center rounded-full border-2 border-background bg-emerald-500"
              />
            </div>
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <p class="text-sm font-medium truncate">
                  {{ user.display_name || user.username }}
                </p>
                <Badge
                  v-if="user.role === 'admin'"
                  variant="outline"
                  class="text-[10px] px-1.5 py-0 border-prod-yellow/40 text-prod-yellow"
                >
                  Админ
                </Badge>
                <Badge v-if="user.banned_at" variant="destructive" class="text-[10px] px-1.5 py-0">
                  Бан
                </Badge>
                <UserTag :tag="user.tag" size="md" />
                <Star
                  v-if="user.promoted_until && new Date(user.promoted_until) > new Date()"
                  class="size-3 text-prod-yellow fill-prod-yellow"
                />
                <span
                  v-if="user.special_tag"
                  class="text-[10px] text-prod-yellow"
                >{{ user.special_tag }}</span>
              </div>
              <div class="flex items-center gap-2 mt-1">
                <p v-if="user.display_name" class="text-xs text-muted-foreground">
                  @{{ user.username }}
                </p>
                <span v-if="user.display_name" class="text-xs text-muted-foreground">·</span>
                <p class="text-xs text-muted-foreground">
                  {{ user.gender === 'male' ? 'М' : user.gender === 'female' ? 'Ж' : '—' }}
                </p>
                <span class="text-xs text-muted-foreground">·</span>
                <p class="text-xs text-muted-foreground tabular-nums">
                  <Coins class="inline size-3 mr-0.5" />{{ user.balance }} обнимань
                </p>
                <p v-if="user.banned_at" class="text-xs text-destructive/70">
                  с {{ formatDate(user.banned_at) }}
                </p>
                <p v-if="user.created_at" class="text-xs text-muted-foreground">
                  рег. {{ formatDate(user.created_at) }}
                </p>
                <p v-if="user.last_visit_at" class="text-xs text-muted-foreground">
                  визит {{ formatDate(user.last_visit_at) }}
                </p>
              </div>
            </div>
          </div>

          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon-sm">
                <MoreHorizontal class="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-48">
              <template v-if="user.role !== 'admin'">
                <DropdownMenuItem @click="toggleBan(user)">
                  <template v-if="user.banned_at">
                    <ShieldCheck class="size-4" />
                    Разблокировать
                  </template>
                  <template v-else>
                    <Ban class="size-4" />
                    Заблокировать
                  </template>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
              </template>
              <DropdownMenuItem @click="openUsernameDialog(user)">
                <UserPen class="size-4" />
                Изменить логин
              </DropdownMenuItem>
              <DropdownMenuItem @click="openDisplayNameDialog(user)">
                <Tag class="size-4" />
                Изменить имя
              </DropdownMenuItem>
              <DropdownMenuItem @click="openTagDialog(user)">
                <Hash class="size-4" />
                Изменить тег
              </DropdownMenuItem>
              <DropdownMenuItem @click="openSpecialTagDialog(user)">
                <Tag class="size-4" />
                Спецтег
              </DropdownMenuItem>
              <DropdownMenuItem
                v-if="user.promoted_until && new Date(user.promoted_until) > new Date()"
                class="text-prod-yellow"
                @click="handleClearPromotion(user)"
              >
                <Star class="size-4 fill-prod-yellow" />
                Убрать из топа
              </DropdownMenuItem>
              <DropdownMenuItem @click="openCaptchaTypeDialog(user)">
                <Puzzle class="size-4" />
                Капча: {{ user.captcha_type === 'none' ? 'Нет' : user.captcha_type === 'sudoku' ? 'Судоку' : 'Казино' }}
              </DropdownMenuItem>
              <DropdownMenuItem @click="openGenderDialog(user)">
                <Venus class="size-4" />
                Изменить пол
              </DropdownMenuItem>
              <DropdownMenuItem @click="openPasswordDialog(user)">
                <KeyRound class="size-4" />
                Сменить пароль
              </DropdownMenuItem>
              <DropdownMenuItem @click="openBalanceDialog(user)">
                <Coins class="size-4" />
                Изменить баланс
              </DropdownMenuItem>
              <template v-if="user.role !== 'admin'">
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  class="text-destructive focus:text-destructive"
                  @click="openDeleteDialog(user)"
                >
                  <Trash2 class="size-4" />
                  Удалить
                </DropdownMenuItem>
              </template>
            </DropdownMenuContent>
          </DropdownMenu>
          </div>
        </TransitionGroup>

        <!-- Infinite scroll sentinel -->
        <div ref="sentinel" class="h-1" />

        <div v-if="admin.loadingMore" class="space-y-3 pt-2">
          <Skeleton v-for="i in 3" :key="i" class="h-16 w-full rounded-lg" />
        </div>

        <p
          v-if="!admin.hasMore && admin.users.length > 0"
          class="py-4 text-center text-sm text-muted-foreground"
        >
          Все пользователи загружены
        </p>
      </div>
    </div>

    <!-- Username dialog -->
    <Dialog v-model:open="usernameDialogOpen">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Изменить имя</DialogTitle>
          <DialogDescription> Пользователь: {{ editingUser?.username }} </DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <div class="grid gap-1.5">
            <Label for="admin-username">Новое имя</Label>
            <Input
              id="admin-username"
              v-model="newUsername"
              maxlength="32"
              placeholder="username"
              @keydown.enter="saveUsername"
            />
          </div>
          <p v-if="usernameError" class="text-sm text-destructive">{{ usernameError }}</p>
          <Button
            variant="yellow"
            class="w-full rounded-[21px]"
            :disabled="savingUsername"
            @click="saveUsername"
          >
            {{ savingUsername ? 'Сохранение...' : 'Сохранить' }}
          </Button>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Gender dialog -->
    <Dialog v-model:open="genderDialogOpen">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Изменить пол</DialogTitle>
          <DialogDescription> Пользователь: {{ editingUser?.username }} </DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <RadioGroup v-model="newGender" class="flex gap-4">
            <div class="flex items-center gap-2">
              <RadioGroupItem id="admin-gender-male" value="male" />
              <Label for="admin-gender-male" class="cursor-pointer font-normal">Мужской</Label>
            </div>
            <div class="flex items-center gap-2">
              <RadioGroupItem id="admin-gender-female" value="female" />
              <Label for="admin-gender-female" class="cursor-pointer font-normal">Женский</Label>
            </div>
          </RadioGroup>
          <Button
            variant="yellow"
            class="w-full rounded-[21px]"
            :disabled="savingGender"
            @click="saveGender"
          >
            {{ savingGender ? 'Сохранение...' : 'Сохранить' }}
          </Button>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Balance dialog -->
    <Dialog v-model:open="balanceDialogOpen">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Изменить баланс</DialogTitle>
          <DialogDescription> Пользователь: {{ editingUser?.username }} </DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <div class="grid gap-1.5">
            <Label for="admin-balance">Количество обнимань</Label>
            <Input
              id="admin-balance"
              v-model.number="newBalance"
              type="number"
              min="0"
              placeholder="0"
              @keydown.enter="saveBalance"
            />
          </div>
          <p v-if="balanceError" class="text-sm text-destructive">{{ balanceError }}</p>
          <Button
            variant="yellow"
            class="w-full rounded-[21px]"
            :disabled="savingBalance"
            @click="saveBalance"
          >
            {{ savingBalance ? 'Сохранение...' : 'Сохранить' }}
          </Button>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Display name dialog -->
    <Dialog v-model:open="displayNameDialogOpen">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Изменить отображаемое имя</DialogTitle>
          <DialogDescription> Пользователь: {{ editingUser?.username }} </DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <div class="grid gap-1.5">
            <Label for="admin-display-name">Отображаемое имя</Label>
            <Input
              id="admin-display-name"
              v-model="newDisplayName"
              maxlength="32"
              placeholder="Оставьте пустым для сброса"
              @keydown.enter="saveDisplayName"
            />
          </div>
          <p v-if="displayNameError" class="text-sm text-destructive">{{ displayNameError }}</p>
          <Button
            variant="yellow"
            class="w-full rounded-[21px]"
            :disabled="savingDisplayName"
            @click="saveDisplayName"
          >
            {{ savingDisplayName ? 'Сохранение...' : 'Сохранить' }}
          </Button>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Tag dialog -->
    <Dialog v-model:open="tagDialogOpen">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Изменить тег</DialogTitle>
          <DialogDescription> Пользователь: {{ editingUser?.username }} </DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <div class="grid gap-1.5">
            <Label for="admin-tag">Тег</Label>
            <Input
              id="admin-tag"
              v-model="newTag"
              maxlength="20"
              placeholder="Оставьте пустым для сброса"
              @keydown.enter="saveTag"
            />
          </div>
          <p v-if="tagError" class="text-sm text-destructive">{{ tagError }}</p>
          <Button
            variant="yellow"
            class="w-full rounded-[21px]"
            :disabled="savingTag"
            @click="saveTag"
          >
            {{ savingTag ? 'Сохранение...' : 'Сохранить' }}
          </Button>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Special tag dialog -->
    <Dialog v-model:open="specialTagDialogOpen">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Спецтег</DialogTitle>
          <DialogDescription>
            Пользователь: {{ editingUser?.username }}. Спецтег отображается с особым стилем.
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <div class="grid gap-1.5">
            <Label for="admin-special-tag">Спецтег</Label>
            <Input
              id="admin-special-tag"
              v-model="newSpecialTag"
              maxlength="20"
              placeholder="Оставьте пустым для сброса"
              @keydown.enter="saveSpecialTag"
            />
          </div>
          <p v-if="specialTagError" class="text-sm text-destructive">{{ specialTagError }}</p>
          <Button
            variant="yellow"
            class="w-full rounded-[21px]"
            :disabled="savingSpecialTag"
            @click="saveSpecialTag"
          >
            {{ savingSpecialTag ? 'Сохранение...' : 'Сохранить' }}
          </Button>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Password dialog -->
    <Dialog v-model:open="passwordDialogOpen">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Сменить пароль</DialogTitle>
          <DialogDescription> Пользователь: {{ editingUser?.username }} </DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <div class="grid gap-1.5">
            <Label for="admin-new-password">Новый пароль</Label>
            <Input
              id="admin-new-password"
              v-model="newPassword"
              type="password"
              placeholder="********"
              :class="{ 'border-destructive': passwordErrorFor('newPassword') }"
            />
            <p v-if="passwordErrorFor('newPassword')" class="text-xs text-destructive">
              {{ passwordErrorFor('newPassword') }}
            </p>
          </div>
          <div class="grid gap-1.5">
            <Label for="admin-new-password-confirm">Подтвердите пароль</Label>
            <Input
              id="admin-new-password-confirm"
              v-model="newPasswordConfirm"
              type="password"
              placeholder="********"
              :class="{ 'border-destructive': passwordErrorFor('newPasswordConfirm') }"
            />
            <p v-if="passwordErrorFor('newPasswordConfirm')" class="text-xs text-destructive">
              {{ passwordErrorFor('newPasswordConfirm') }}
            </p>
          </div>
          <p v-if="passwordServerError" class="text-sm text-destructive">
            {{ passwordServerError }}
          </p>
          <Button
            variant="yellow"
            class="w-full rounded-[21px]"
            :disabled="savingPassword"
            @click="savePassword"
          >
            {{ savingPassword ? 'Сохранение...' : 'Сменить пароль' }}
          </Button>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Captcha type dialog -->
    <Dialog v-model:open="captchaTypeDialogOpen">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Тип капчи</DialogTitle>
          <DialogDescription> Пользователь: {{ editingUser?.username }} </DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <RadioGroup v-model="newCaptchaType" class="grid gap-2">
            <div class="flex items-center gap-2">
              <RadioGroupItem id="captcha-none" value="none" />
              <Label for="captcha-none" class="cursor-pointer font-normal">Без капчи</Label>
            </div>
            <div class="flex items-center gap-2">
              <RadioGroupItem id="captcha-sudoku" value="sudoku" />
              <Label for="captcha-sudoku" class="cursor-pointer font-normal">Судоку</Label>
            </div>
            <div class="flex items-center gap-2">
              <RadioGroupItem id="captcha-casino" value="casino" />
              <Label for="captcha-casino" class="cursor-pointer font-normal">Казино (1к4)</Label>
            </div>
          </RadioGroup>
          <Button
            variant="yellow"
            class="w-full rounded-[21px]"
            :disabled="savingCaptchaType"
            @click="saveCaptchaType"
          >
            {{ savingCaptchaType ? 'Сохранение...' : 'Сохранить' }}
          </Button>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Delete user dialog -->
    <Dialog v-model:open="deleteDialogOpen">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Удалить пользователя</DialogTitle>
          <DialogDescription>
            Это действие необратимо. Все данные пользователя будут удалены.
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-4">
          <div class="grid gap-1.5">
            <Label for="admin-delete-confirm">
              Введите <span class="font-mono font-semibold">{{ editingUser?.username }}</span> для
              подтверждения
            </Label>
            <Input
              id="admin-delete-confirm"
              v-model="deleteConfirmUsername"
              placeholder="имя пользователя"
              @keydown.enter="confirmDeleteUser"
            />
          </div>
          <Button
            variant="destructive"
            class="w-full rounded-[21px]"
            :disabled="deletingUser || deleteConfirmUsername !== editingUser?.username"
            @click="confirmDeleteUser"
          >
            {{ deletingUser ? 'Удаление...' : 'Удалить навсегда' }}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
.user-list-move {
  transition: transform 0.4s ease;
}
</style>
