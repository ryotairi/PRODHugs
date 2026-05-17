import { ref } from 'vue'
import { ensureAccessToken } from '@/lib/token'

const socket = ref<WebSocket | null>(null)
const connected = ref(false)
const listeners = new Map<string, Set<(data: unknown) => void>>()

// WebSocket typed message wrapper
export interface WSMessage<T = unknown> {
  type:
    | 'hug_completed'
    | 'hug_suggestion'
    | 'hug_declined'
    | 'hug_cancelled'
    | 'hug_expired'
    | 'inbox_count'
    | 'online_count'
    | 'online_users'
    | 'announcement'
    | 'announcement_removed'
    | 'vips_updated'
  data: T
}

let reconnectTimeout: ReturnType<typeof setTimeout> | null = null
let isIntentionallyDisconnected = false
// Monotonically increasing ID to scope onclose handlers to the correct socket instance.
let connectGeneration = 0

export function useWebSocket() {
  async function connect() {
    // Prevent duplicate connections
    if (socket.value && socket.value.readyState !== WebSocket.CLOSED) {
      return
    }

    isIntentionallyDisconnected = false

    // Get JWT token from localStorage
    const token = await ensureAccessToken()
    if (!token) return

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${proto}//${window.location.host}/api/v1/ws`

    // Increment generation so stale onclose handlers from previous sockets are ignored.
    const gen = ++connectGeneration
    const ws = new WebSocket(url)

    // Assign immediately to prevent duplicate connect() calls passing the null guard.
    socket.value = ws

    ws.onopen = () => {
      ws.send(JSON.stringify({ type: 'auth', token }))
      connected.value = true
      // Clear any pending reconnect timeout
      if (reconnectTimeout) {
        clearTimeout(reconnectTimeout)
        reconnectTimeout = null
      }
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as WSMessage
        const handlers = listeners.get(msg.type)
        if (handlers) {
          handlers.forEach((fn) => fn(msg.data))
        }
      } catch {
        // Ignore malformed messages
      }
    }

    ws.onclose = () => {
      // Only process if this is still the current socket (prevents stale onclose
      // from an old socket clobbering a newly created connection).
      if (gen !== connectGeneration) return

      connected.value = false
      socket.value = null
      // Auto-reconnect after 3 seconds, unless intentionally disconnected
      if (!isIntentionallyDisconnected && !reconnectTimeout) {
        reconnectTimeout = setTimeout(() => {
          reconnectTimeout = null
          connect()
        }, 3000)
      }
    }

    ws.onerror = () => {
      ws.close()
    }
  }

  function on<T = unknown>(type: WSMessage<T>['type'], handler: (data: T) => void): () => void {
    if (!listeners.has(type)) {
      listeners.set(type, new Set())
    }
    // Cast is safe: the handler will receive the correct type at runtime
    // because the backend sends typed messages matching the 'type' field
    const h = handler as (data: unknown) => void
    listeners.get(type)!.add(h)

    // Return cleanup function
    return () => {
      listeners.get(type)?.delete(h)
    }
  }

  function disconnect() {
    isIntentionallyDisconnected = true
    // Increment generation so any pending onclose from the current socket is ignored.
    connectGeneration++
    // Clear reconnect timeout
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }
    if (socket.value) {
      socket.value.close()
      socket.value = null
    }
    connected.value = false
    // Clear all listeners to prevent stale handlers leaking across auth sessions.
    listeners.clear()
  }

  return { connected, connect, disconnect, on }
}
