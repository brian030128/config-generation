import axios from "axios"

export const client = axios.create({
  baseURL: "/api",
  withCredentials: true,
})

function getCookie(name: string): string | null {
  const prefix = `${name}=`
  return (
    document.cookie
      .split(";")
      .map((part) => part.trim())
      .find((part) => part.startsWith(prefix))
      ?.slice(prefix.length) ?? null
  )
}

function isUnsafeMethod(method?: string): boolean {
  return !["get", "head", "options", "trace"].includes(
    (method ?? "get").toLowerCase(),
  )
}

client.interceptors.request.use((config) => {
  if (isUnsafeMethod(config.method)) {
    const csrf = getCookie("configgen_csrf")
    if (csrf) {
      config.headers["X-CSRF-Token"] = csrf
    }
  }
  return config
})

client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401 && globalThis.location.pathname !== "/login") {
      localStorage.removeItem("auth_token")
      globalThis.location.href = "/login"
    }
    return Promise.reject(error)
  },
)
