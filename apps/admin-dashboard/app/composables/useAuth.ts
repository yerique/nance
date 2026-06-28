import type { User } from '~/types/accelerator'

const TOKEN_KEY = 'nance_session_token'
const USER_KEY = 'nance_session_user'

export function useAuth() {
  const token = useState<string | null>('auth_token', () => null)
  const user = useState<User | null>('auth_user', () => null)
  const ready = useState('auth_ready', () => false)

  function loadFromStorage() {
    if (!import.meta.client) return
    token.value = localStorage.getItem(TOKEN_KEY)
    const raw = localStorage.getItem(USER_KEY)
    if (raw) {
      try {
        user.value = JSON.parse(raw) as User
      }
      catch {
        user.value = null
      }
    }
    ready.value = true
  }

  function setSession(t: string, u: User) {
    token.value = t
    user.value = u
    if (import.meta.client) {
      localStorage.setItem(TOKEN_KEY, t)
      localStorage.setItem(USER_KEY, JSON.stringify(u))
    }
  }

  function clearSession() {
    token.value = null
    user.value = null
    if (import.meta.client) {
      localStorage.removeItem(TOKEN_KEY)
      localStorage.removeItem(USER_KEY)
    }
  }

  const isLoggedIn = computed(() => !!token.value && !!user.value)

  return { token, user, ready, isLoggedIn, loadFromStorage, setSession, clearSession }
}
