import { cn } from "@/lib/utils"
import iconDark from "@/assets/brand/icon-dark.svg"
import iconLight from "@/assets/brand/icon-light.svg"
import wordmarkDark from "@/assets/brand/wordmark-dark.svg"
import wordmarkLight from "@/assets/brand/wordmark-light.svg"

const SOURCES = {
  icon: { light: iconLight, dark: iconDark },
  wordmark: { light: wordmarkLight, dark: wordmarkDark },
} as const

export type LogoVariant = keyof typeof SOURCES

interface LogoProps {
  variant?: LogoVariant
  className?: string
  alt?: string
}

/**
 * Brand mark renderer. Switches light/dark assets via Tailwind's `dark:` class
 * so callers don't need to read a theme context.
 *
 * Usage:
 *   <Logo variant="icon" className="h-9 w-9" />
 *   <Logo variant="wordmark" className="h-18 w-auto" />
 */
export function Logo({ variant = "wordmark", className, alt = "ConfiGen" }: Readonly<LogoProps>) {
  const src = SOURCES[variant]
  return (
    <>
      <img src={src.light} alt={alt} className={cn("dark:hidden", className)} />
      <img src={src.dark} alt={alt} className={cn("hidden dark:block", className)} />
    </>
  )
}
