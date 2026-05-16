import { ref, onMounted, onUnmounted } from 'vue'

const now = ref(Date.now())
let interval: ReturnType<typeof setInterval> | null = null
let useCount = 0

export function useTicker() {
  onMounted(() => {
    useCount++
    if (!interval) {
      interval = setInterval(() => {
        now.value = Date.now()
      }, 1000)
    }
  })

  onUnmounted(() => {
    useCount--
    if (useCount <= 0 && interval) {
      clearInterval(interval)
      interval = null
    }
  })

  return { now }
}
