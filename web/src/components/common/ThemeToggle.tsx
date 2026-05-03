import { Laptop, Moon, Sun } from 'lucide-react'
import { useTheme, type ThemeMode } from '../../contexts/ThemeContext'

const THEME_OPTIONS: { value: ThemeMode; label: string; icon: typeof Sun }[] = [
  { value: 'system', label: 'Auto', icon: Laptop },
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
]

interface ThemeToggleProps {
  compact?: boolean
  className?: string
}

export function ThemeToggle({ compact = false, className = '' }: ThemeToggleProps) {
  const { theme, setTheme } = useTheme()

  return (
    <div
      className={`flex items-center rounded-2xl border border-[var(--panel-border)] bg-[var(--panel-bg)]/90 p-1 backdrop-blur-xl ${className}`.trim()}
    >
      {THEME_OPTIONS.map(({ value, label, icon: Icon }) => {
        const active = theme === value

        return (
          <button
            key={value}
            type="button"
            onClick={() => setTheme(value)}
            className={`inline-flex items-center justify-center gap-2 rounded-xl px-3 py-2 text-xs font-semibold transition-all ${
              active
                ? 'bg-nofx-gold text-black shadow-[0_10px_24px_rgba(240,185,11,0.24)]'
                : 'text-nofx-text-muted hover:bg-white/5 hover:text-nofx-text-main'
            }`}
            aria-pressed={active}
            aria-label={`Switch to ${label} theme`}
            title={label}
          >
            <Icon className="h-3.5 w-3.5" />
            {!compact && <span>{label}</span>}
          </button>
        )
      })}
    </div>
  )
}
