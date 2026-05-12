import { ref, onUnmounted } from 'vue'
import { type AxiosError } from 'axios'
import { authApi } from '@/api/client'
import { setAccessToken } from '@/lib/token'
import { useAuthStore } from '@/stores/auth'
import router from '@/router'

export interface MatrixSignupSession {
  botUserId: string
  botUrl: string
  command: string
}

/**
 * Matrix signup/login composable. Mirrors useTelegramLogin, but instead of
 * opening a deep-link directly, it returns a session (bot URL + copyable
 * command) that the UI displays in a popup. The user opens the bot in their
 * Matrix client and sends the command; we poll the backend until the session
 * resolves.
 */
export function useMatrixLogin() {
  const auth = useAuthStore()

  const matrixPolling = ref(false)
  const matrixError = ref<string | null>(null)
  const matrixLoading = ref(false)
  const matrixSession = ref<MatrixSignupSession | null>(null)

  let pollInterval: ReturnType<typeof setInterval> | null = null
  let pollToken: string | null = null
  let pollAttempts = 0
  const MAX_POLL_ATTEMPTS = 150 // 5 minutes at 2-second intervals

  function stopPolling() {
    if (pollInterval) {
      clearInterval(pollInterval)
      pollInterval = null
    }
    matrixPolling.value = false
    matrixSession.value = null
    pollToken = null
    pollAttempts = 0
  }

  async function startMatrixLogin() {
    matrixError.value = null
    matrixLoading.value = true

    try {
      const res = await authApi.initMatrixLogin()
      const { bot_user_id, bot_url, command, poll_token } = res.data

      pollToken = poll_token
      pollAttempts = 0
      matrixPolling.value = true
      matrixSession.value = {
        botUserId: bot_user_id,
        botUrl: bot_url,
        command,
      }

      pollInterval = setInterval(async () => {
        if (!pollToken) {
          stopPolling()
          return
        }

        pollAttempts++
        if (pollAttempts >= MAX_POLL_ATTEMPTS) {
          stopPolling()
          matrixError.value = 'Время ожидания истекло. Попробуйте снова'
          return
        }

        try {
          const pollRes = await authApi.pollMatrixLogin(pollToken)

          if (pollRes.status === 200) {
            stopPolling()
            const data = pollRes.data
            auth.token = data.token
            auth.user = data.user
            setAccessToken(data.token)
            localStorage.setItem('user', JSON.stringify(data.user))
            await router.push('/dashboard')
          }
          // 202 = still pending, keep polling
        } catch (err: unknown) {
          const axiosErr = err as AxiosError<{ message?: string }>
          if (axiosErr.response?.status === 403) {
            stopPolling()
            matrixError.value = axiosErr.response?.data?.message || 'Аккаунт заблокирован'
          } else if (axiosErr.response?.status === 404) {
            stopPolling()
            matrixError.value = 'Сессия истекла. Попробуйте снова'
          }
          // Other errors: keep polling (transient network issues)
        }
      }, 2000)
    } catch (err: unknown) {
      const axiosErr = err as AxiosError
      if (axiosErr.response?.status === 503) {
        matrixError.value = 'Вход через Matrix недоступен'
      } else {
        matrixError.value = 'Не удалось начать вход через Matrix'
      }
    } finally {
      matrixLoading.value = false
    }
  }

  function cancelMatrixLogin() {
    stopPolling()
    matrixError.value = null
  }

  onUnmounted(() => {
    stopPolling()
  })

  return {
    matrixPolling,
    matrixError,
    matrixLoading,
    matrixSession,
    startMatrixLogin,
    cancelMatrixLogin,
  }
}
