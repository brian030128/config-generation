import {
  createContext,
  useContext,
  useState,
  useCallback,
  type ReactNode,
} from "react"
import { jwtDecode } from "jwt-decode"

interface JwtClaims {
  user_id: number
  username: string
  exp?: number
}

interface AuthUser {
  id: number
  username: string
}

interface AuthContextValue {
  token: string | null
  user: AuthUser | null
  login: (token: string) => boolean
  logout: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

function parseToken(token: string): AuthUser | null {
  try {
    const claims = jwtDecode<JwtClaims>(token)
    if (claims.exp && claims.exp * 1000 < Date.now()) {
      return null
    }
    return { id: claims.user_id, username: claims.username }
  } catch {
    return null
  }
}

function getStoredAuth(): { token: string; user: AuthUser } | null {
  const token = localStorage.getItem("auth_token")
  if (!token) return null
  const user = parseToken(token)
  if (!user) {
    localStorage.removeItem("auth_token")
    return null
  }
  return { token, user }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [auth, setAuth] = useState(getStoredAuth)

  const login = useCallback((token: string) => {
    const user = parseToken(token)
    if (!user) return false
    localStorage.setItem("auth_token", token)
    setAuth({ token, user })
    return true
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem("auth_token")
    setAuth(null)
  }, [])

  return (
    <AuthContext.Provider value={{
      token: auth?.token ?? null,
      user: auth?.user ?? null,
      login,
      logout,
    }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within AuthProvider")
  return ctx
}
