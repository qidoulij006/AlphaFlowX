import { motion, AnimatePresence } from 'framer-motion'
import { LogIn, UserPlus, X, AlertTriangle, Terminal } from 'lucide-react'
import { DeepVoidBackground } from '../common/DeepVoidBackground'
import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'

interface LoginRequiredOverlayProps {
  isOpen: boolean
  onClose: () => void
  featureName?: string
}

export function LoginRequiredOverlay({ isOpen, onClose, featureName }: LoginRequiredOverlayProps) {
  const { language } = useLanguage()

  const tr = (key: string, params?: Record<string, string | number>) =>
    t(`loginRequired.${key}`, language, params)

  const subtitle = featureName
    ? tr('subtitleWithFeature', { featureName })
    : tr('subtitleDefault')

  const benefits = [
    tr('benefit1'),
    tr('benefit2'),
    tr('benefit4'),
  ]

  return (
    <AnimatePresence>
      {isOpen && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 z-50 flex items-center justify-center"
        >
          <DeepVoidBackground
            className="w-full h-full bg-nofx-bg/95 backdrop-blur-md flex items-center justify-center p-4 text-nofx-text"
            disableAnimation
            onClick={onClose}
          >
            <div className="flex min-h-screen w-full items-center justify-center">
              <motion.div
                initial={{ opacity: 0, scale: 0.95, y: 10 }}
                animate={{ opacity: 1, scale: 1, y: 0 }}
                exit={{ opacity: 0, scale: 0.95, y: 10 }}
                transition={{ type: 'spring', damping: 20, stiffness: 300 }}
                className="relative my-auto w-full max-w-md overflow-hidden rounded-sm border shadow-neon group font-mono"
                style={{
                  background: 'var(--panel-bg-solid)',
                  borderColor:
                    'color-mix(in srgb, var(--accent-primary) 28%, var(--panel-border))',
                }}
                onClick={(e) => e.stopPropagation()}
              >
                <div
                  className="flex items-center justify-between px-3 py-2 border-b"
                  style={{
                    background: 'var(--panel-bg)',
                    borderColor:
                      'color-mix(in srgb, var(--accent-primary) 14%, var(--panel-border))',
                  }}
                >
                  <div className="flex items-center gap-2">
                    <Terminal size={12} style={{ color: 'var(--accent-primary)' }} />
                  </div>
                  <button
                    onClick={onClose}
                    className="transition-colors"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    <X size={14} />
                  </button>
                </div>

                <div className="relative p-8">
                  <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808008_1px,transparent_1px),linear-gradient(to_bottom,#80808008_1px,transparent_1px)] bg-[size:14px_14px] pointer-events-none"></div>

                  <div className="relative z-10 flex flex-col items-center text-center">
                    <div className="mb-6 flex justify-center self-stretch">
                      <div className="relative">
                        <div className="absolute inset-0 bg-red-500/20 blur-xl animate-pulse"></div>
                        <div
                          className="flex items-center gap-3 px-4 py-2 shadow-[0_0_15px_rgba(239,68,68,0.2)]"
                          style={{
                            background: 'var(--panel-bg-solid)',
                            border: '1px solid rgba(239,68,68,0.45)',
                            color: 'rgb(220, 38, 38)',
                          }}
                        >
                          <AlertTriangle size={18} className="animate-pulse" />
                          <span className="text-sm font-bold uppercase tracking-widest">{tr('accessDenied')}</span>
                        </div>
                      </div>
                    </div>

                    <div className="mb-8 w-full space-y-4">
                      <div className="text-center">
                        <h2
                          className="mb-2 text-xl font-bold uppercase tracking-wider"
                          style={{ color: 'var(--text-primary)' }}
                        >
                          {tr('title')}
                        </h2>
                        <p
                          className="inline-block border-b pb-4 text-xs uppercase tracking-widest"
                          style={{
                            color: 'var(--accent-primary)',
                            borderColor:
                              'color-mix(in srgb, var(--accent-primary) 18%, transparent)',
                          }}
                        >
                          {subtitle}
                        </p>
                      </div>

                      <div
                        className="my-4 p-3"
                        style={{
                          background: 'var(--panel-bg)',
                          borderLeft: '2px solid color-mix(in srgb, var(--accent-primary) 22%, transparent)',
                        }}
                      >
                        <p
                          className="font-mono text-xs leading-relaxed"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          <span className="mr-2 text-green-500">$</span>
                          {tr('description')}
                        </p>
                      </div>

                      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                        {benefits.map((benefit, i) => (
                          <div
                            key={i}
                            className="flex items-center justify-center gap-2 text-[10px] uppercase tracking-wide"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            <span style={{ color: 'var(--accent-primary)' }}>✓</span>
                            {benefit}
                          </div>
                        ))}
                      </div>
                    </div>

                    <div className="w-full space-y-3">
                      <a
                        href="/login"
                        className="group flex w-full items-center justify-center gap-2 py-3 text-xs font-bold uppercase tracking-widest transition-all shadow-neon"
                        style={{ background: 'var(--accent-primary)', color: '#111111' }}
                      >
                        <LogIn size={14} />
                        <span>{tr('loginButton')}</span>
                        <span className="-ml-2 opacity-0 transition-opacity group-hover:ml-0 group-hover:opacity-100">-&gt;</span>
                      </a>

                      <a
                        href="/register"
                        className="flex w-full items-center justify-center gap-2 border py-3 text-xs font-bold uppercase tracking-widest transition-all"
                        style={{
                          background: 'transparent',
                          borderColor:
                            'color-mix(in srgb, var(--accent-primary) 18%, var(--panel-border))',
                          color: 'var(--text-secondary)',
                        }}
                      >
                        <UserPlus size={14} />
                        <span>{tr('registerButton')}</span>
                      </a>
                    </div>

                    <div className="mt-4 text-center">
                      <button
                        onClick={onClose}
                        className="text-[10px] uppercase tracking-widest hover:underline"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        [ {tr('abort')} ]
                      </button>
                    </div>
                  </div>
                </div>
                <div className="absolute top-0 right-0 h-2 w-2 border-t border-r" style={{ borderColor: 'var(--accent-primary)' }}></div>
                <div className="absolute bottom-0 left-0 h-2 w-2 border-b border-l" style={{ borderColor: 'var(--accent-primary)' }}></div>
              </motion.div>

            </div>
          </DeepVoidBackground>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
