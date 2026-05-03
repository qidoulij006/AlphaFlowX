import React, { useEffect, useState } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import PasswordChecklist from 'react-password-checklist'
import { toast } from 'sonner'
import { useAuth } from '../../contexts/AuthContext'
import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'
import { getSystemConfig } from '../../lib/config'
import { BRAND_INFO } from '../../constants/branding'
import { DeepVoidBackground } from '../common/DeepVoidBackground'
import { RegistrationDisabled } from './RegistrationDisabled'
import { WhitelistFullPage } from '../common/WhitelistFullPage'

export function RegisterPage() {
  const { language } = useLanguage()
  const { register } = useAuth()
  const [view, setView] = useState<'register' | 'whitelist-full'>('register')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [betaCode, setBetaCode] = useState('')
  const [betaMode, setBetaMode] = useState(false)
  const [registrationEnabled, setRegistrationEnabled] = useState(true)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [passwordValid, setPasswordValid] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)

  useEffect(() => {
    getSystemConfig()
      .then((config) => {
        setBetaMode(config.beta_mode || false)
        setRegistrationEnabled(config.registration_enabled === true)
      })
      .catch((err) => {
        console.error('Failed to fetch system config:', err)
      })
  }, [])

  if (!registrationEnabled) {
    return <RegistrationDisabled />
  }

  if (view === 'whitelist-full') {
    return <WhitelistFullPage onBack={() => setView('register')} />
  }

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (!passwordValid) {
      setError(t('passwordNotMeetRequirements', language))
      return
    }

    if (betaMode && !betaCode.trim()) {
      setError(
        language === 'zh'
          ? '当前为内测阶段，注册需要提供内测码'
          : 'A beta access code is required during the closed beta period'
      )
      return
    }

    setLoading(true)
    try {
      const result = await register(email, password, betaCode.trim() || undefined)

      const isWhitelistError = (msg: string) => {
        const lowerMsg = msg.toLowerCase()
        return (
          lowerMsg.includes('whitelist') ||
          lowerMsg.includes('capacity') ||
          lowerMsg.includes('limit') ||
          lowerMsg.includes('permission denied') ||
          lowerMsg.includes('not on whitelist')
        )
      }

      if (!result.success) {
        const msg = result.message || t('registrationFailed', language)
        if (isWhitelistError(msg)) {
          setView('whitelist-full')
          return
        }
        setError(msg)
        toast.error(msg)
      }
      // success path is handled in AuthContext (auto login + navigation)
    } catch (e) {
      console.error('Registration error:', e)
      const errorMsg =
        e instanceof Error
          ? e.message
          : language === 'zh'
            ? '服务器异常，注册失败'
            : 'Registration failed due to server error'
      const lowerMsg = errorMsg.toLowerCase()
      if (
        lowerMsg.includes('whitelist') ||
        lowerMsg.includes('capacity') ||
        lowerMsg.includes('limit') ||
        lowerMsg.includes('permission denied') ||
        lowerMsg.includes('not on whitelist')
      ) {
        setView('whitelist-full')
        return
      }
      setError(errorMsg)
      toast.error(errorMsg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <DeepVoidBackground className="min-h-screen flex items-center justify-center px-4 py-12 font-mono" disableAnimation showFloatingThemeToggle>
      <div className="w-full max-w-3xl relative z-10 mx-auto">
        <div className="mb-8 text-center">
          <div className="flex justify-center mb-5">
            <div className="relative">
              <div className="absolute -inset-4 bg-nofx-gold/15 rounded-full blur-2xl"></div>
              <div className="relative flex h-20 w-20 items-center justify-center rounded-3xl border border-[var(--panel-border)] bg-[var(--panel-bg)]/65 backdrop-blur-md shadow-[0_0_60px_rgba(255,215,0,0.08)]">
                <img src={BRAND_INFO.iconPath} alt={`${BRAND_INFO.name} Logo`} className="w-12 h-12 object-contain relative z-10 opacity-90" />
              </div>
            </div>
          </div>
          <h1 className="text-3xl md:text-4xl font-bold tracking-tight text-nofx-text-main mb-2">
            {language === 'zh' ? '用户注册' : 'User Registration'}
          </h1>
        </div>

        <div className="mx-auto w-full max-w-xl theme-surface backdrop-blur-md rounded-2xl overflow-hidden shadow-2xl relative group">
          <div className="absolute inset-0 bg-[var(--panel-bg-hover)] opacity-0 group-hover:opacity-100 transition duration-700 pointer-events-none"></div>

          <div className="p-6 md:p-8 relative">
            <form onSubmit={handleRegister} className="space-y-5">
              <div>
                <label className="block text-xs uppercase tracking-wider text-nofx-text-muted mb-1.5 ml-1 font-bold">{t('email', language)}</label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="theme-input theme-ring w-full rounded px-4 py-3 text-sm transition-all font-mono"
                  placeholder="user@alphaflowx.com"
                  required
                />
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs uppercase tracking-wider text-nofx-text-muted mb-1.5 ml-1 font-bold">{t('password', language)}</label>
                  <div className="relative">
                    <input
                      type={showPassword ? 'text' : 'password'}
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      className="theme-input theme-ring w-full rounded px-4 py-3 text-sm transition-all font-mono pr-10"
                      placeholder="••••••••"
                      required
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--text-tertiary)] hover:text-nofx-text-muted transition-colors"
                    >
                      {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                    </button>
                  </div>
                </div>

                <div>
                  <label className="block text-xs uppercase tracking-wider text-nofx-text-muted mb-1.5 ml-1 font-bold">{t('confirmPassword', language)}</label>
                  <div className="relative">
                    <input
                      type={showConfirmPassword ? 'text' : 'password'}
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      className="theme-input theme-ring w-full rounded px-4 py-3 text-sm transition-all font-mono pr-10"
                      placeholder="••••••••"
                      required
                    />
                    <button
                      type="button"
                      onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--text-tertiary)] hover:text-nofx-text-muted transition-colors"
                    >
                      {showConfirmPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                    </button>
                  </div>
                </div>
              </div>

              <div className="bg-[var(--panel-bg-solid)]/55 p-4 rounded-xl border border-[var(--panel-border)]/80">
                <div className="text-[10px] uppercase tracking-wider text-nofx-text-muted mb-2 font-bold flex items-center justify-center gap-2 text-center">
                  <div className="w-1 h-1 rounded-full bg-[var(--text-tertiary)]"></div>
                  {language === 'zh' ? '密码要求' : 'Password Rules'}
                </div>
                <div className="text-xs font-mono text-nofx-text-muted">
                  <PasswordChecklist
                    rules={['minLength', 'capital', 'lowercase', 'number', 'specialChar', 'match']}
                    minLength={8}
                    value={password}
                    valueAgain={confirmPassword}
                    messages={{
                      minLength: t('passwordRuleMinLength', language),
                      capital: t('passwordRuleUppercase', language),
                      lowercase: t('passwordRuleLowercase', language),
                      number: t('passwordRuleNumber', language),
                      specialChar: t('passwordRuleSpecial', language),
                      match: t('passwordRuleMatch', language),
                    }}
                    className="grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-1"
                    onChange={(isValid) => setPasswordValid(isValid)}
                    iconSize={10}
                  />
                </div>
              </div>

              {betaMode && (
                <div>
                  <label className="block text-xs uppercase tracking-wider text-nofx-gold mb-1.5 ml-1 font-bold">Priority Access Code</label>
                  <input
                    type="text"
                  value={betaCode}
                  onChange={(e) => setBetaCode(e.target.value.replace(/[^a-z0-9]/gi, '').toLowerCase())}
                  className="theme-input theme-ring w-full rounded px-4 py-3 text-sm transition-all font-mono tracking-widest"
                  placeholder="XXXXXX"
                  maxLength={6}
                  required={betaMode}
                />
                  <p className="text-[10px] text-[var(--text-tertiary)] font-mono mt-1 ml-1">
                    {language === 'zh' ? '* 区分大小写，仅支持字母和数字' : '* Case-sensitive letters and numbers only'}
                  </p>
                </div>
              )}

              {error && (
                <div className="text-xs bg-red-500/10 border border-red-500/30 text-red-500 px-3 py-2 rounded font-mono">
                  [REGISTRATION_ERROR]: {error}
                </div>
              )}

              <button
                type="submit"
                disabled={loading || (betaMode && !betaCode.trim()) || !passwordValid}
                className="w-full bg-nofx-gold text-black font-bold py-3 px-4 rounded text-sm tracking-wide uppercase hover:bg-yellow-400 transition-all transform active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed font-mono shadow-[0_0_15px_rgba(255,215,0,0.1)] hover:shadow-[0_0_25px_rgba(255,215,0,0.25)] flex items-center justify-center gap-2 group mt-4"
              >
                {loading ? (
                  <span className="animate-pulse">{language === 'zh' ? '注册中...' : 'Creating account...'}</span>
                ) : (
                  <>
                    <span>{t('registerButton', language)}</span>
                    <span className="group-hover:translate-x-1 transition-transform">-&gt;</span>
                  </>
                )}
              </button>
            </form>
          </div>
        </div>

        <div className="mx-auto mt-8 w-full max-w-xl text-center">
          <p className="text-xs font-mono text-nofx-text-muted">
            {language === 'zh' ? '已有账号？' : 'Already have an account?'}{' '}
            <button
              onClick={() => (window.location.href = '/login')}
              className="inline-flex items-center justify-center rounded-full border border-[var(--panel-border)] bg-[var(--panel-bg)]/55 px-3 py-1.5 text-nofx-gold hover:text-yellow-300 transition-colors ml-1 uppercase"
            >
              {language === 'zh' ? '立即登录' : 'Login'}
            </button>
          </p>
        </div>
      </div>
    </DeepVoidBackground>
  )
}
