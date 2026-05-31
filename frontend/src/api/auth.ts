import axios from "axios"
import { client } from "./client"

export interface AuthResponse {
  token: string
  user: {
    id: number
    username: string
    display_name: string | null
    created_at: string
  }
}

export interface AuthUser {
  id: number
  username: string
  display_name: string | null
  created_at: string
  // superuser is only populated by GET /api/auth/me. Other endpoints that
  // return users (search, role members, project members) deliberately omit it.
  superuser: boolean
}

export interface AuthConfig {
  oidc_enabled: boolean
  oidc_provider_name: string
  password_login_enabled: boolean
  registration_enabled: boolean
}

const authClient = axios.create({ baseURL: "/api", withCredentials: true })

export async function login(
  username: string,
  password: string,
): Promise<AuthResponse> {
  const { data } = await authClient.post<AuthResponse>("/auth/login", {
    username,
    password,
  })
  return data
}

export async function getAuthConfig(): Promise<AuthConfig> {
  const { data } = await authClient.get<AuthConfig>("/auth/config")
  return data
}

export async function me(): Promise<AuthUser> {
  const { data } = await authClient.get<AuthUser>("/auth/me")
  return data
}

export async function createSession(token: string): Promise<AuthResponse> {
  const { data } = await authClient.post<AuthResponse>("/auth/session", {
    token,
  })
  return data
}

export async function logout(): Promise<void> {
  await client.post("/auth/logout")
}

export async function register(
  username: string,
  password: string,
  displayName?: string,
): Promise<AuthResponse> {
  const { data } = await authClient.post<AuthResponse>("/auth/register", {
    username,
    password,
    display_name: displayName || undefined,
  })
  return data
}
