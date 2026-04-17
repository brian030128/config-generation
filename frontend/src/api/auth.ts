import axios from "axios"

export interface AuthResponse {
  token: string
  user: {
    id: number
    username: string
    display_name: string | null
    created_at: string
  }
}

const authClient = axios.create({ baseURL: "/api" })

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
