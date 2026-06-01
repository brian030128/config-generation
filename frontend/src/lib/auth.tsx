import {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
  useMemo,
  type ReactNode,
} from "react"
import {
  createSession,
  logout as apiLogout,
  me,
  type AuthResponse,
  type AuthUser,
} from "@/api/auth"

interface AuthContextValue {
  token: string | null
  user: AuthUser | null
  loading: boolean
  login: (response: AuthResponse) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

// fromAuthResponse builds an AuthUser from the login/session response, which
// does not include the superuser flag. login() refetches via me() to fill it
// in, but we keep a conservative default in case that fetch fails.
function fromAuthResponse(user: AuthResponse["user"]): AuthUser {
  return { ...user, superuser: false }
}

export function AuthProvider({ children }: Readonly<{ children: ReactNode }>) {
  const [auth, setAuth] = useState<{ token: string; user: AuthUser } | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false

    async function loadAuth() {
      try {
        const user = await me()
        if (!cancelled) {
          setAuth({ token: "cookie-session", user })
        }
        return
      } catch {
        // Try one-time migration from the old localStorage bearer token.
      }

      const oldToken = localStorage.getItem("auth_token")
      if (oldToken) {
        try {
          await createSession(oldToken)
          localStorage.removeItem("auth_token")
          // After the session cookie is set, refetch via me() to pick up the
          // superuser flag (createSession's response shape omits it).
          const user = await me()
          if (!cancelled) {
            setAuth({ token: "cookie-session", user })
          }
          return
        } catch {
          localStorage.removeItem("auth_token")
        }
      }

      if (!cancelled) {
        setAuth(null)
      }
    }

    loadAuth().finally(() => {
      if (!cancelled) setLoading(false)
    })

    return () => {
      cancelled = true
    }
  }, [])

  const login = useCallback(async (response: AuthResponse) => {
    localStorage.removeItem("auth_token")
    // Seed immediately so the UI flips out of the loading state, then refetch
    // via /auth/me to obtain the superuser flag (not included in login).
    setAuth({ token: "cookie-session", user: fromAuthResponse(response.user) })
    try {
      const user = await me()
      setAuth({ token: "cookie-session", user })
    } catch {
      // Keep the seeded user; superuser stays false until next reload.
    }
  }, [])

  const logout = useCallback(async () => {
    try {
      await apiLogout()
    } finally {
      localStorage.removeItem("auth_token")
      setAuth(null)
    }
  }, [])

  // Memoize the context value so consumers don't re-render on every parent
  // render just because the provider's value object identity changed.
  const value = useMemo<AuthContextValue>(
    () => ({
      token: auth?.token ?? null,
      user: auth?.user ?? null,
      loading,
      login,
      logout,
    }),
    [auth, loading, login, logout],
  )

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within AuthProvider")
  return ctx
}
