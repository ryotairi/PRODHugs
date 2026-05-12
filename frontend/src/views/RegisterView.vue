<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { authApi, usersApi } from '@/api/client'
import {
  validateRegisterForm,
  validateUsername,
  parseBackendError,
  type FieldError,
} from '@/lib/validation'
import { useTelegramLogin } from '@/composables/useTelegramLogin'
import { useMatrixLogin } from '@/composables/useMatrixLogin'
import MatrixSignupDialog from '@/components/MatrixSignupDialog.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import PasswordRequirements from '@/components/PasswordRequirements.vue'
import { Send, Loader2, MessageSquare } from 'lucide-vue-next'

const auth = useAuthStore()
const {
  telegramPolling,
  telegramError,
  telegramLoading,
  startTelegramLogin,
  cancelTelegramLogin,
} = useTelegramLogin()
const {
  matrixPolling,
  matrixError,
  matrixLoading,
  matrixSession,
  startMatrixLogin,
  cancelMatrixLogin,
} = useMatrixLogin()
const username = ref('')
const displayName = ref('')
const password = ref('')
const passwordConfirm = ref('')
const gender = ref('')
const serverError = ref('')
const fieldErrors = ref<FieldError[]>([])
const submitted = ref(false)

// ── Live username availability check ──
const usernameAvailable = ref<boolean | null>(null) // null = not checked yet
const checkingUsername = ref(false)
let usernameCheckTimer: ReturnType<typeof setTimeout> | null = null
let usernameCheckGeneration = 0

function scheduleUsernameCheck() {
  // Reset state immediately
  usernameAvailable.value = null

  if (usernameCheckTimer) clearTimeout(usernameCheckTimer)

  // Only check if the username passes local validation
  const localError = validateUsername(username.value)
  if (localError) return

  usernameCheckTimer = setTimeout(async () => {
    const gen = ++usernameCheckGeneration
    checkingUsername.value = true
    try {
      const res = await authApi.checkUsername(username.value.trim())
      if (gen === usernameCheckGeneration) {
        usernameAvailable.value = res.data.available
      }
    } catch {
      // Network error — don't show anything
      if (gen === usernameCheckGeneration) {
        usernameAvailable.value = null
      }
    } finally {
      if (gen === usernameCheckGeneration) {
        checkingUsername.value = false
      }
    }
  }, 400)
}

watch(username, scheduleUsernameCheck)

onUnmounted(() => {
  if (usernameCheckTimer) clearTimeout(usernameCheckTimer)
  usernameCheckGeneration++
})

function errorFor(field: string): string | undefined {
  return fieldErrors.value.find((e) => e.field === field)?.message
}

const hasErrors = computed(() => fieldErrors.value.length > 0)

function validate() {
  fieldErrors.value = validateRegisterForm(username.value, password.value, passwordConfirm.value)
}

async function handleRegister() {
  submitted.value = true
  serverError.value = ''
  validate()
  if (hasErrors.value) return

  try {
    await auth.register(username.value, password.value, gender.value || undefined)
    // Set display name after successful registration (if provided)
    const dn = displayName.value.trim()
    if (dn) {
      try {
        await usersApi.updateSettings({ display_name: dn })
        await auth.fetchMe()
      } catch {
        // Non-critical — user can set it later in settings
      }
    }
  } catch (e: any) {
    const parsed = parseBackendError(e)
    if (parsed.fieldErrors.length > 0) {
      fieldErrors.value = [...fieldErrors.value, ...parsed.fieldErrors]
    }
    if (parsed.generalError) {
      serverError.value = parsed.generalError
    }
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-background p-4">
    <Card class="w-full max-w-sm relative">
      <!-- Telegram polling overlay -->
      <div
        v-if="telegramPolling"
        class="absolute inset-0 z-10 flex flex-col items-center justify-center gap-4 rounded-lg bg-background/95 backdrop-blur-sm"
      >
        <Loader2 class="h-8 w-8 animate-spin text-primary" />
        <p class="text-sm text-center text-muted-foreground px-6">
          Нажмите <strong>Start</strong> в открывшемся боте, чтобы зарегистрироваться
        </p>
        <Button variant="ghost" size="sm" @click="cancelTelegramLogin"> Отмена </Button>
      </div>

      <CardHeader class="text-center">
        <img src="/logo.webp" alt="PROD" class="mx-auto mb-2 size-12 rounded-lg object-contain" />
        <CardTitle class="text-xl">Регистрация</CardTitle>
        <CardDescription>Создай аккаунт в PRODнимашках</CardDescription>
      </CardHeader>
      <CardContent>
        <form @submit.prevent="handleRegister" class="grid gap-4">
          <div class="grid gap-2">
            <Label for="username">Имя пользователя</Label>
            <div class="relative">
              <Input
                id="username"
                v-model="username"
                type="text"
                placeholder="username"
                maxlength="32"
                :class="{
                  'border-destructive': submitted && errorFor('username'),
                  'border-prod-yellow/50': !submitted && usernameAvailable === true,
                  'border-destructive/50': !submitted && usernameAvailable === false,
                  'pr-8': username.length >= 3,
                }"
                @input="submitted && validate()"
              />
              <div
                v-if="username.length >= 3 && !errorFor('username')"
                class="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2"
              >
                <svg
                  v-if="checkingUsername"
                  class="size-4 animate-spin text-muted-foreground"
                  viewBox="0 0 24 24"
                  fill="none"
                >
                  <circle
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="2"
                    class="opacity-25"
                  />
                  <path
                    d="M4 12a8 8 0 018-8"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                  />
                </svg>
                <svg
                  v-else-if="usernameAvailable === true"
                  class="size-4 text-prod-yellow"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2.5"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path d="M5 12l5 5L20 7" />
                </svg>
                <svg
                  v-else-if="usernameAvailable === false"
                  class="size-4 text-destructive"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2.5"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path d="M18 6L6 18M6 6l12 12" />
                </svg>
              </div>
            </div>
            <p v-if="submitted && errorFor('username')" class="text-xs text-destructive">
              {{ errorFor('username') }}
            </p>
            <p
              v-else-if="usernameAvailable === false"
              class="text-xs text-destructive text-right"
            >
              Это имя уже занято
            </p>
            <p
              v-else-if="usernameAvailable === true"
              class="text-xs text-prod-yellow text-right"
            >
              Имя свободно
            </p>
          </div>
          <div class="grid gap-2">
            <Label for="display-name"
              >Отображаемое имя
              <span class="text-muted-foreground text-xs">(необязательно)</span></Label
            >
            <Input
              id="display-name"
              v-model="displayName"
              type="text"
              maxlength="32"
              placeholder="Как тебя называть"
            />
          </div>
          <div class="grid gap-2">
            <Label for="password">Пароль</Label>
            <Input
              id="password"
              v-model="password"
              type="password"
              placeholder="********"
              :class="{ 'border-destructive': submitted && errorFor('password') }"
              @input="submitted && validate()"
            />
            <p v-if="submitted && errorFor('password')" class="text-xs text-destructive">
              {{ errorFor('password') }}
            </p>
            <PasswordRequirements :password="password" />
          </div>
          <div class="grid gap-2">
            <Label for="password-confirm">Подтверждение пароля</Label>
            <Input
              id="password-confirm"
              v-model="passwordConfirm"
              type="password"
              placeholder="********"
              :class="{ 'border-destructive': submitted && errorFor('passwordConfirm') }"
              @input="submitted && validate()"
            />
            <p v-if="submitted && errorFor('passwordConfirm')" class="text-xs text-destructive">
              {{ errorFor('passwordConfirm') }}
            </p>
          </div>
          <div class="grid gap-2">
            <Label>Пол <span class="text-muted-foreground text-xs">(необязательно)</span></Label>
            <RadioGroup v-model="gender" class="flex gap-4">
              <div class="flex items-center gap-2">
                <RadioGroupItem id="reg-gender-male" value="male" />
                <Label for="reg-gender-male" class="font-normal cursor-pointer">Мужской</Label>
              </div>
              <div class="flex items-center gap-2">
                <RadioGroupItem id="reg-gender-female" value="female" />
                <Label for="reg-gender-female" class="font-normal cursor-pointer">Женский</Label>
              </div>
            </RadioGroup>
          </div>
          <p v-if="serverError" class="text-sm text-destructive text-center">
            {{ serverError }}
          </p>
          <Button
            type="submit"
            variant="yellow"
            class="w-full rounded-[21px]"
            :disabled="auth.loading"
          >
            {{ auth.loading ? 'Регистрация...' : 'Зарегистрироваться' }}
          </Button>
        </form>
        <!-- Telegram registration section -->
        <div class="mt-6 flex flex-col items-center gap-2">
          <Separator class="my-2 w-full" />
          <Button
            type="button"
            variant="outline"
            class="w-full flex items-center justify-center gap-2"
            :disabled="telegramLoading"
            @click="startTelegramLogin"
          >
            <Send class="w-4 h-4" />
            {{ telegramLoading ? 'Открывается бот...' : 'Регистрация через Telegram' }}
          </Button>
          <p v-if="telegramError" class="text-sm text-destructive text-center">
            {{ telegramError }}
          </p>

          <!-- Matrix registration -->
          <Button
            type="button"
            variant="outline"
            class="w-full flex items-center justify-center gap-2"
            :disabled="matrixLoading"
            @click="startMatrixLogin"
          >
            <MessageSquare class="w-4 h-4" />
            {{ matrixLoading ? 'Подготовка...' : 'Регистрация через Matrix' }}
          </Button>
          <p v-if="matrixError && !matrixPolling" class="text-sm text-destructive text-center">
            {{ matrixError }}
          </p>
        </div>
      </CardContent>
      <CardFooter class="justify-center">
        <p class="text-sm text-muted-foreground">
          Уже есть аккаунт?
          <RouterLink
            to="/login"
            class="text-foreground underline underline-offset-4 hover:text-primary"
          >
            Войти
          </RouterLink>
        </p>
      </CardFooter>
    </Card>

    <MatrixSignupDialog
      v-if="matrixSession"
      :open="matrixPolling"
      :bot-user-id="matrixSession.botUserId"
      :bot-url="matrixSession.botUrl"
      :command="matrixSession.command"
      :error="matrixError"
      mode="register"
      @cancel="cancelMatrixLogin"
    />
  </div>
</template>
