import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { useAuth } from "@/lib/auth"
import {
  getAuthConfig,
  login as apiLogin,
  register as apiRegister,
  type AuthConfig,
} from "@/api/auth"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent } from "@/components/ui/card"
import { Logo } from "@/components/brand/logo"
import { GithubIcon } from "@/components/icons/github"
import { GoogleIcon } from "@/components/icons/google"
import { AxiosError } from "axios"

function ProviderIcon({ name, className }: { name: string; className?: string }) {
  const lower = name.toLowerCase()
  if (lower.includes("github")) return <GithubIcon className={className} />
  if (lower.includes("google")) return <GoogleIcon className={className} />
  return null
}

type View = "login" | "register"
type Pos = "center" | "exit-left" | "exit-right" | "enter-from-left" | "enter-from-right"

function getErrorMessage(err: unknown): string {
  if (err instanceof AxiosError && err.response?.data?.error) {
    return err.response.data.error
  }
  return "Something went wrong"
}

export default function LoginPage() {
  const { login, user, loading } = useAuth()
  const navigate = useNavigate()
  const [authConfig, setAuthConfig] = useState<AuthConfig | null>(null)
  const [repoStats, setRepoStats] = useState<{ stars: number; forks: number } | null>(null)

  // Slide animation
  const [view, setView] = useState<View>("login")
  const [pos, setPos] = useState<Pos>("center")

  // Login form state
  const [loginUsername, setLoginUsername] = useState("")
  const [loginPassword, setLoginPassword] = useState("")
  const [loginError, setLoginError] = useState("")
  const [loginLoading, setLoginLoading] = useState(false)

  // Register form state
  const [regUsername, setRegUsername] = useState("")
  const [regDisplayName, setRegDisplayName] = useState("")
  const [regPassword, setRegPassword] = useState("")
  const [regConfirm, setRegConfirm] = useState("")
  const [regError, setRegError] = useState("")
  const [regLoading, setRegLoading] = useState(false)

  useEffect(() => {
    getAuthConfig()
      .then(setAuthConfig)
      .catch(() => {
        setAuthConfig({
          oidc_enabled: false,
          oidc_provider_name: "SSO",
          password_login_enabled: true,
          registration_enabled: true,
        })
      })
  }, [])

  useEffect(() => {
    fetch("https://api.github.com/repos/brian030128/config-generation")
      .then((r) => r.json())
      .then((d) => setRepoStats({ stars: d.stargazers_count, forks: d.forks_count }))
      .catch(() => {})
  }, [])

  useEffect(() => {
    if (!loading && user) {
      navigate("/projects", { replace: true })
    }
  }, [loading, navigate, user])

  function switchTo(to: View) {
    const forward = to === "register"
    setPos(forward ? "exit-left" : "exit-right")
    setTimeout(() => {
      setView(to)
      setPos(forward ? "enter-from-right" : "enter-from-left")
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          setPos("center")
        })
      })
    }, 220)
  }

  async function handleLogin(e: React.FormEvent) {
    e.preventDefault()
    setLoginError("")
    if (!loginUsername.trim() || !loginPassword) {
      setLoginError("Username and password are required")
      return
    }
    setLoginLoading(true)
    try {
      const res = await apiLogin(loginUsername.trim(), loginPassword)
      await login(res)
      navigate("/projects", { replace: true })
    } catch (err) {
      setLoginError(getErrorMessage(err))
    } finally {
      setLoginLoading(false)
    }
  }

  async function handleRegister(e: React.FormEvent) {
    e.preventDefault()
    setRegError("")
    if (!regUsername.trim() || !regPassword) {
      setRegError("Username and password are required")
      return
    }
    if (regPassword.length < 8) {
      setRegError("Password must be at least 8 characters")
      return
    }
    if (regPassword !== regConfirm) {
      setRegError("Passwords do not match")
      return
    }
    setRegLoading(true)
    try {
      const res = await apiRegister(
        regUsername.trim(),
        regPassword,
        regDisplayName.trim() || undefined,
      )
      await login(res)
      navigate("/projects", { replace: true })
    } catch (err) {
      setRegError(getErrorMessage(err))
    } finally {
      setRegLoading(false)
    }
  }

  function handleSSOLogin() {
    const returnTo = encodeURIComponent("/projects")
    window.location.href = `/api/auth/oidc/login?return_to=${returnTo}`
  }

  const showPasswordLogin = authConfig?.password_login_enabled ?? true
  const showRegistration = authConfig?.registration_enabled ?? true
  const showSSO = authConfig?.oidc_enabled ?? false

  const slideClass = [
    "w-full max-w-md",
    pos !== "enter-from-left" && pos !== "enter-from-right"
      ? "transition-all duration-[220ms] ease-in-out"
      : "",
    pos === "center"
      ? "translate-x-0 opacity-100"
      : pos === "exit-left" || pos === "enter-from-left"
        ? "-translate-x-full opacity-0"
        : "translate-x-full opacity-0",
  ].join(" ")

  return (
    <div className="flex min-h-screen flex-col">
      {/* Full-width header */}
      <div className="flex h-18 shrink-0 items-center justify-between border-b px-4">
        <Logo variant="wordmark" className="h-18 w-auto" />

        {/* GitHub repo card — top-right, hidden on mobile */}
        <a
          href="https://github.com/brian030128/config-generation"
          target="_blank"
          rel="noreferrer"
          className="hidden items-center gap-3 rounded-lg border bg-card px-4 py-2 text-sm text-muted-foreground shadow-xs transition-colors hover:text-foreground md:flex"
        >
          <div className="flex items-center gap-1.5 font-semibold text-foreground">
            <GithubIcon className="h-4 w-4" />
            brian030128/config-generation
          </div>
          {repoStats && (
            <ul className="flex items-center gap-3 text-xs">
              <li className="flex items-center gap-1">
                <span aria-label="stars">★</span>
                {repoStats.stars.toLocaleString()}
              </li>
              <li className="flex items-center gap-1">
                <span aria-label="forks">⑂</span>
                {repoStats.forks.toLocaleString()}
              </li>
            </ul>
          )}
        </a>
      </div>

      {/* Content row */}
      <div className="flex flex-1">
        {/* Left panel: full width on mobile, half on md+ */}
        <div className="flex w-full flex-col md:w-1/2">
          {/* overflow-hidden clips the card as it slides */}
          <div className="relative flex flex-1 overflow-hidden">
            <div className="absolute inset-0 flex items-center justify-center px-6 py-8">
              <div className={slideClass}>
                <Card>
                  <CardContent className="pt-6">
                    {/* SSO button — login view only */}
                    {showSSO && view === "login" && (
                      <div className="space-y-4">
                        <Button
                          type="button"
                          variant="outline"
                          className="w-full"
                          onClick={handleSSOLogin}
                        >
                          <ProviderIcon
                            name={authConfig?.oidc_provider_name ?? ""}
                            className="mr-2 h-4 w-4"
                          />
                          Continue with {authConfig?.oidc_provider_name ?? "SSO"}
                        </Button>
                        {(showPasswordLogin || showRegistration) && (
                          <div className="flex items-center gap-3 text-xs font-medium text-muted-foreground">
                            <div className="h-px flex-1 bg-border" />
                            <span>OR</span>
                            <div className="h-px flex-1 bg-border" />
                          </div>
                        )}
                      </div>
                    )}

                    {/* Login form */}
                    {view === "login" && showPasswordLogin && (
                      <form
                        onSubmit={handleLogin}
                        className={"space-y-4" + (showSSO ? " mt-4" : "")}
                      >
                        <h2 className="text-xl font-semibold">Sign In</h2>
                        <div className="space-y-2">
                          <Label htmlFor="login-username">Username</Label>
                          <Input
                            id="login-username"
                            type="text"
                            autoComplete="username"
                            placeholder="Enter your username"
                            value={loginUsername}
                            onChange={(e) => setLoginUsername(e.target.value)}
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="login-password">Password</Label>
                          <Input
                            id="login-password"
                            type="password"
                            autoComplete="current-password"
                            placeholder="Enter your password"
                            value={loginPassword}
                            onChange={(e) => setLoginPassword(e.target.value)}
                          />
                        </div>
                        {loginError && (
                          <p className="text-sm text-destructive">{loginError}</p>
                        )}
                        <Button type="submit" className="w-full" disabled={loginLoading}>
                          {loginLoading ? "Signing in..." : "Sign In"}
                        </Button>
                        {showRegistration && (
                          <p className="text-center text-sm text-muted-foreground">
                            Don&apos;t have an account?{" "}
                            <button
                              type="button"
                              className="font-medium text-foreground underline-offset-4 hover:underline"
                              onClick={() => switchTo("register")}
                            >
                              Register
                            </button>
                          </p>
                        )}
                      </form>
                    )}

                    {/* Register form */}
                    {view === "register" && showRegistration && (
                      <form
                        onSubmit={handleRegister}
                        className={"space-y-4" + (showSSO ? " mt-4" : "")}
                      >
                        <h2 className="text-xl font-semibold">Create Account</h2>
                        <div className="space-y-2">
                          <Label htmlFor="reg-username">Username</Label>
                          <Input
                            id="reg-username"
                            type="text"
                            autoComplete="username"
                            placeholder="Choose a username"
                            value={regUsername}
                            onChange={(e) => setRegUsername(e.target.value)}
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="reg-display-name">Display Name</Label>
                          <Input
                            id="reg-display-name"
                            type="text"
                            autoComplete="name"
                            placeholder="Optional"
                            value={regDisplayName}
                            onChange={(e) => setRegDisplayName(e.target.value)}
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="reg-password">Password</Label>
                          <Input
                            id="reg-password"
                            type="password"
                            autoComplete="new-password"
                            placeholder="At least 8 characters"
                            value={regPassword}
                            onChange={(e) => setRegPassword(e.target.value)}
                          />
                        </div>
                        <div className="space-y-2">
                          <Label htmlFor="reg-confirm">Confirm Password</Label>
                          <Input
                            id="reg-confirm"
                            type="password"
                            autoComplete="new-password"
                            placeholder="Repeat your password"
                            value={regConfirm}
                            onChange={(e) => setRegConfirm(e.target.value)}
                          />
                        </div>
                        {regError && (
                          <p className="text-sm text-destructive">{regError}</p>
                        )}
                        <Button type="submit" className="w-full" disabled={regLoading}>
                          {regLoading ? "Creating account..." : "Create Account"}
                        </Button>
                        {showPasswordLogin && (
                          <p className="text-center text-sm text-muted-foreground">
                            Already have an account?{" "}
                            <button
                              type="button"
                              className="font-medium text-foreground underline-offset-4 hover:underline"
                              onClick={() => switchTo("login")}
                            >
                              Sign in
                            </button>
                          </p>
                        )}
                      </form>
                    )}
                  </CardContent>
                </Card>
              </div>
            </div>
          </div>
        </div>

        {/* Right panel: demo video, hidden on mobile */}
        <div className="hidden md:flex md:flex-1 md:items-center md:justify-center md:bg-muted/30 md:p-8">
          <div className="w-full max-w-2xl overflow-hidden rounded-xl border shadow-lg">
            <video
              src="/demo.mp4"
              autoPlay
              muted
              loop
              playsInline
              preload="auto"
              className="w-full"
            />
          </div>
        </div>
      </div>
    </div>
  )
}
