import React, { useState, useEffect } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { toast } from 'sonner'
import { useAuth } from '../../contexts/AuthContext'
import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'
import { BRAND_INFO } from '../../constants/branding'
import { DeepVoidBackground } from '../common/DeepVoidBackground'

interface LoginPageProps {
  adminMode?: boolean
}

export function LoginPage({ adminMode = false }: LoginPageProps) {
  const { language } = useLanguage()
  const { login, loginAdmin } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [expiredToastId, setExpiredToastId] = useState<string | number | null>(null)

  useEffect(() => {
    if (sessionStorage.getItem('from401') === 'true') {
      const id = toast.warning(t('sessionExpired', language), { duration: Infinity })
      setExpiredToastId(id)
      sessionStorage.removeItem('from401')
    }
  }, [language])

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    const result = adminMode
      ? await loginAdmin(password)
      : await login(email, password)
    setLoading(false)
    if (result.success) {
      if (expiredToastId) toast.dismiss(expiredToastId)
    } else {
      const msg = result.message || t('loginFailed', language)
      setError(msg)
      toast.error(msg)
    }
  }

  return (
    <DeepVoidBackground disableAnimation showFloatingThemeToggle>
      <div className="flex-1 flex items-center justify-center px-4 py-16">
        <div className="w-full max-w-sm">

          {/* Logo + Title */}
          <div className="text-center mb-10">
            <div className="flex justify-center mb-5">
              <div className="relative">
                <div className="absolute -inset-3 bg-nofx-gold/15 rounded-full blur-2xl" />
                <img src={BRAND_INFO.iconPath} alt={BRAND_INFO.name} className="w-14 h-14 relative z-10" />
              </div>
            </div>
            <h1 className="text-2xl font-bold text-nofx-text-main mb-1.5">
              {adminMode ? t('adminMode', language) : 'Welcome back'}
            </h1>
            <p className="text-nofx-text-muted text-sm">
              {adminMode ? 'Sign in with the admin password' : 'Sign in to your account'}
            </p>
          </div>

          {/* Card */}
          <div className="theme-surface rounded-2xl p-8 shadow-2xl backdrop-blur-xl">
            <form onSubmit={handleLogin} className="space-y-5">

              {!adminMode && (
                <div>
                  <label className="block text-xs font-medium text-nofx-text-muted mb-2">
                    {t('email', language)}
                  </label>
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    className="theme-input theme-ring w-full rounded-xl px-4 py-3 text-sm transition-all"
                    placeholder="you@example.com"
                    required
                    autoFocus
                  />
                </div>
              )}

              {/* Password */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-xs font-medium text-nofx-text-muted">
                    {adminMode ? `${t('adminMode', language)} ${t('password', language)}` : t('password', language)}
                  </label>
                  {!adminMode && (
                    <span className="text-xs text-[var(--text-tertiary)]">
                      {t('forgotPassword', language)}
                    </span>
                  )}
                </div>
                <div className="relative">
                  <input
                    type={showPassword ? 'text' : 'password'}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="theme-input theme-ring w-full rounded-xl px-4 py-3 pr-11 text-sm transition-all"
                    placeholder={adminMode ? 'Admin password' : '••••••••'}
                    required
                    autoFocus={adminMode}
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3.5 top-1/2 -translate-y-1/2 text-nofx-text-muted hover:text-nofx-text-main transition-colors"
                  >
                    {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>

              {/* Error */}
              {error && (
                <p className="text-xs text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg px-3 py-2">
                  {error}
                </p>
              )}

              {/* Submit */}
              <button
                type="submit"
                disabled={loading}
                className="w-full bg-nofx-gold hover:bg-yellow-400 active:scale-[0.98] text-black font-semibold py-3 rounded-xl text-sm transition-all disabled:opacity-50 disabled:cursor-not-allowed mt-2"
              >
                {loading ? t('loggingIn', language) || 'Signing in...' : t('signIn', language) || 'Sign In'}
              </button>

              <div className="pt-3 space-y-3">
                {!adminMode && (
                  <a
                    href="/register"
                    className="flex w-full items-center justify-center rounded-xl border border-[var(--panel-border)] bg-[var(--panel-bg-solid)] px-4 py-3 text-sm font-semibold text-nofx-text-main transition-all hover:border-nofx-gold/40 hover:bg-white/5"
                  >
                    {language === 'zh' ? '注册新用户' : 'Create new account'}
                  </a>
                )}

                <div className="text-center">
                  <a
                    href="/"
                    className="text-xs text-nofx-text-muted hover:text-nofx-gold transition-colors"
                  >
                    {language === 'zh' ? '返回首页' : 'Back to home'}
                  </a>
                </div>
              </div>
            </form>
          </div>

        </div>
      </div>
    </DeepVoidBackground>
  )
}
