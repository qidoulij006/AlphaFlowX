import React, { useState, useEffect } from 'react'
import { Check, ChevronLeft, ExternalLink, MessageCircle, Unlink, ArrowRight } from 'lucide-react'
import { toast } from 'sonner'
import { filterVisibleModels } from './model-constants'
import { api } from '../../lib/api'
import type { TelegramConfig, AIModel } from '../../types'
import { t, type Language } from '../../i18n/translations'

// Step indicator (reused pattern from ExchangeConfigModal)
function StepIndicator({ currentStep, labels }: { currentStep: number; labels: string[] }) {
  return (
    <div className="flex items-center justify-center gap-2 mb-6">
      {labels.map((label, index) => (
        <React.Fragment key={index}>
          <div className="flex items-center gap-2">
            <div
              className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-all"
              style={{
                background: index < currentStep ? '#0ECB81' : index === currentStep ? '#2AABEE' : 'var(--panel-border)',
                color: index <= currentStep ? '#000' : 'var(--text-secondary)',
              }}
            >
              {index < currentStep ? <Check className="w-4 h-4" /> : index + 1}
            </div>
            <span
              className="text-xs font-medium hidden sm:block"
              style={{ color: index === currentStep ? 'var(--text-primary)' : 'var(--text-secondary)' }}
            >
              {label}
            </span>
          </div>
          {index < labels.length - 1 && (
            <div
              className="w-8 h-0.5 mx-1"
              style={{ background: index < currentStep ? '#0ECB81' : 'var(--panel-border)' }}
            />
          )}
        </React.Fragment>
      ))}
    </div>
  )
}

interface TelegramConfigModalProps {
  onClose: () => void
  language: Language
}

export function TelegramConfigModal({ onClose, language }: TelegramConfigModalProps) {
  const [step, setStep] = useState(0)
  const [token, setToken] = useState('')
  const [selectedModelId, setSelectedModelId] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [config, setConfig] = useState<TelegramConfig | null>(null)
  const [models, setModels] = useState<AIModel[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [isUnbinding, setIsUnbinding] = useState(false)

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  // Load current config and available models
  useEffect(() => {
    Promise.all([
      api.getTelegramConfig().catch(() => null),
      api.getModelConfigs().catch(() => [] as AIModel[]),
    ]).then(([cfg, allModels]) => {
      const enabledModels = allModels.filter((m) => m.enabled)
      setModels(enabledModels)

      if (cfg) {
        setConfig(cfg)
        setSelectedModelId(cfg.model_id ?? '')
        if (cfg.is_bound) {
          setStep(2)
        } else if (cfg.token_masked && cfg.token_masked !== '') {
          setStep(1)
        }
      }
    }).finally(() => setIsLoading(false))
  }, [])

  const handleSaveToken = async () => {
    if (!token.trim()) return
    if (isSaving) return

    // Basic format validation: looks like "123456789:ABCdef..."
    if (!/^\d+:[A-Za-z0-9_-]{35,}$/.test(token.trim())) {
      toast.error(t('telegram.invalidTokenFormat', language))
      return
    }

    setIsSaving(true)
    try {
      await api.updateTelegramConfig(token.trim(), selectedModelId || undefined)
      toast.success(t('telegram.tokenSaved', language))
      const updated = await api.getTelegramConfig()
      setConfig(updated)
      setToken('')
      setStep(1)
    } catch (err) {
      toast.error(t('telegram.saveFailed', language))
    } finally {
      setIsSaving(false)
    }
  }

  const handleUnbind = async () => {
    if (isUnbinding) return
    setIsUnbinding(true)
    try {
      await api.unbindTelegram()
      toast.success(t('telegram.unbound', language))
      const updated = await api.getTelegramConfig()
      setConfig(updated)
      setStep(updated.token_masked ? 1 : 0)
    } catch {
      toast.error(t('telegram.unbindFailed', language))
    } finally {
      setIsUnbinding(false)
    }
  }

  const stepLabels = [t('telegram.createBot', language), t('telegram.bindAccount', language), t('telegram.done', language)]

  // Model selector shared between steps
  const ModelSelector = () => (
    <div className="space-y-2">
      <label className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
        {t('telegram.selectAiModel', language)}
      </label>
      {models.length === 0 ? (
        <div
          className="px-4 py-3 rounded-xl text-xs"
          style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)', color: 'var(--text-secondary)' }}
        >
          {t('telegram.noEnabledModels', language)}
        </div>
      ) : (
        <select
          value={selectedModelId}
          onChange={(e) => setSelectedModelId(e.target.value)}
          className="w-full px-4 py-3 rounded-xl text-sm appearance-none"
          style={{
            background: 'var(--panel-bg-solid)',
            border: '1px solid var(--panel-border)',
            color: selectedModelId ? 'var(--text-primary)' : 'var(--text-secondary)',
          }}
        >
          <option value="">{t('telegram.autoSelect', language)}</option>
          {models.map((m) => (
            <option key={m.id} value={m.id}>
              {m.name} ({m.provider}{m.customModelName ? ` · ${m.customModelName}` : ''})
            </option>
          ))}
        </select>
      )}
      <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
        {t('telegram.autoUseEnabled', language)}
      </div>
    </div>
  )

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/60 p-4 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="rounded-2xl w-full max-w-lg relative my-8 shadow-2xl"
        style={{ background: 'linear-gradient(180deg, var(--panel-bg) 0%, color-mix(in srgb, var(--panel-bg-solid) 92%, black) 100%)', border: '1px solid var(--panel-border)' }}
        onClick={(event) => event.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 pb-2">
          <div className="flex items-center gap-3">
            {step > 0 && !config?.is_bound && (
              <button
                type="button"
                onClick={() => setStep(step - 1)}
                className="p-2 rounded-lg transition-colors"
                style={{ background: 'transparent' }}
              >
                <ChevronLeft className="w-5 h-5" style={{ color: 'var(--text-secondary)' }} />
              </button>
            )}
            <div className="flex items-center gap-2">
              <MessageCircle className="w-6 h-6" style={{ color: '#2AABEE' }} />
              <h3 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>
                {t('telegram.botSetup', language)}
              </h3>
            </div>
          </div>
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation()
              onClose()
            }}
            className="p-2 rounded-lg transition-colors"
            style={{ color: 'var(--text-secondary)' }}
          >
            ✕
          </button>
        </div>

        {/* Step Indicator */}
        <div className="px-6 pt-4">
          <StepIndicator currentStep={step} labels={stepLabels} />
        </div>

        {/* Content */}
        <div className="px-6 pb-6 space-y-5">
          {isLoading ? (
            <div className="text-center py-8 text-nofx-text-muted text-sm font-mono">
              {t('telegram.loading', language)}
            </div>
          ) : (
            <>
              {/* Step 0: Create bot via BotFather */}
              {step === 0 && (
                <div className="space-y-5">
                  <div
                    className="p-4 rounded-xl space-y-3"
                    style={{ background: 'rgba(42, 171, 238, 0.1)', border: '1px solid rgba(42, 171, 238, 0.3)' }}
                  >
                    <div className="flex items-start gap-3">
                      <span className="text-2xl">🤖</span>
                      <div>
                        <div className="font-semibold mb-1" style={{ color: '#2AABEE' }}>
                          {t('telegram.step1Title', language)}
                        </div>
                        <div className="text-xs space-y-1" style={{ color: 'var(--text-secondary)' }}>
                          <div>1. {t('telegram.step1Desc1', language)} <code className="text-blue-400">@BotFather</code></div>
                          <div>2. {t('telegram.step1Desc2', language)} <code className="text-blue-400">/newbot</code> {t('telegram.step1Desc2Suffix', language)}</div>
                          <div>3. {t('telegram.step1Desc3', language)}</div>
                          <div>4. {t('telegram.step1Desc4', language)}</div>
                        </div>
                      </div>
                    </div>
                  </div>

                  <a
                    href="https://t.me/BotFather"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="w-full flex items-center justify-center gap-2 px-4 py-3 rounded-xl font-semibold transition-all hover:scale-[1.02]"
                    style={{ background: '#2AABEE', color: '#000' }}
                  >
                    <ExternalLink className="w-4 h-4" />
                    {t('telegram.openBotFather', language)}
                  </a>

                  <div className="space-y-2">
                    <label className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                      {t('telegram.pasteToken', language)}
                    </label>
                    <input
                      type="password"
                      value={token}
                      onChange={(e) => setToken(e.target.value)}
                      placeholder="123456789:ABCdefGHIjklmNOPQRstuvwxYZ"
                      className="w-full px-4 py-3 rounded-xl font-mono text-sm"
                      style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                    />
                    <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                      {t('telegram.tokenFormat', language)}
                    </div>
                  </div>

                  <ModelSelector />

                  <button
                    onClick={handleSaveToken}
                    disabled={isSaving || !token.trim()}
                    className="w-full flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02] disabled:opacity-50 disabled:cursor-not-allowed"
                    style={{ background: '#2AABEE', color: '#000' }}
                  >
                    {isSaving
                      ? t('telegram.savingToken', language)
                      : (<>{t('telegram.saveAndContinue', language)} <ArrowRight className="w-4 h-4" /></>)
                    }
                  </button>
                </div>
              )}

              {/* Step 1: Send /start to activate */}
              {step === 1 && (
                <div className="space-y-5">
                  <div
                    className="p-4 rounded-xl space-y-3"
                    style={{ background: 'rgba(14, 203, 129, 0.1)', border: '1px solid rgba(14, 203, 129, 0.3)' }}
                  >
                    <div className="flex items-start gap-3">
                      <span className="text-2xl">📱</span>
                      <div>
                        <div className="font-semibold mb-1" style={{ color: '#0ECB81' }}>
                          {t('telegram.step2Title', language)}
                        </div>
                        <div className="text-xs space-y-1" style={{ color: 'var(--text-secondary)' }}>
                          <div>1. {t('telegram.step2Desc1', language)}</div>
                          <div>2. {t('telegram.step2Desc2', language)} <code className="text-green-400">/start</code></div>
                          <div>3. {t('telegram.step2Desc3', language)}</div>
                        </div>
                      </div>
                    </div>
                  </div>

                  {config?.token_masked && (
                    <div
                      className="p-3 rounded-xl flex items-center gap-3"
                      style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}
                    >
                      <div className="w-2 h-2 rounded-full bg-yellow-500 animate-pulse flex-shrink-0" />
                      <div>
                        <div className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                          {t('telegram.currentToken', language)}
                        </div>
                        <div className="text-sm font-mono" style={{ color: 'var(--text-primary)' }}>
                          {config.token_masked}
                        </div>
                      </div>
                    </div>
                  )}

                  <div
                    className="p-3 rounded-xl text-center"
                    style={{ background: 'rgba(240, 185, 11, 0.08)', border: '1px solid rgba(240, 185, 11, 0.2)' }}
                  >
                    <div className="text-xs" style={{ color: '#F0B90B' }}>
                      {t('telegram.waitingForStart', language)}
                    </div>
                  </div>

                  <div className="flex gap-3">
                    <button
                      onClick={() => { setStep(0); setToken('') }}
                      className="flex-1 px-4 py-3 rounded-xl text-sm font-semibold transition-all"
                      style={{ background: 'var(--panel-border)', color: 'var(--text-secondary)' }}
                    >
                      {t('telegram.reconfigureToken', language)}
                    </button>
                    <button
                      onClick={async () => {
                        try {
                          const updated = await api.getTelegramConfig()
                          setConfig(updated)
                          if (updated.is_bound) {
                            setStep(2)
                            toast.success(t('telegram.bindSuccess', language))
                          } else {
                            toast.info(t('telegram.noStartReceived', language))
                          }
                        } catch {
                          toast.error(t('telegram.checkFailed', language))
                        }
                      }}
                      className="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02]"
                      style={{ background: '#0ECB81', color: '#000' }}
                    >
                      <Check className="w-4 h-4" />
                      {t('telegram.checkStatus', language)}
                    </button>
                  </div>
                </div>
              )}

              {/* Step 2: Bound & active */}
              {step === 2 && (
                <div className="space-y-5">
                  <div
                    className="p-5 rounded-xl text-center space-y-3"
                    style={{ background: 'rgba(14, 203, 129, 0.1)', border: '1px solid rgba(14, 203, 129, 0.3)' }}
                  >
                    <div className="text-4xl">🎉</div>
                    <div className="font-bold text-lg" style={{ color: '#0ECB81' }}>
                      {t('telegram.botActive', language)}
                    </div>
                    <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                      {t('telegram.botActiveDesc', language)}
                    </div>
                  </div>

                  {config?.token_masked && (
                    <div
                      className="p-3 rounded-xl flex items-center gap-3"
                      style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}
                    >
                      <div className="w-2 h-2 rounded-full bg-green-500 flex-shrink-0" />
                      <div className="min-w-0">
                        <div className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                          Bot Token
                        </div>
                        <div className="text-sm font-mono truncate" style={{ color: 'var(--text-primary)' }}>
                          {config.token_masked}
                        </div>
                      </div>
                    </div>
                  )}

                  {/* AI Model selector — works on active bot */}
                  <BoundModelSelector
                    language={language}
                    models={models}
                    currentModelId={config?.model_id ?? ''}
                    onSaved={(modelId) => {
                      setConfig((prev) => prev ? { ...prev, model_id: modelId } : prev)
                    }}
                  />

                  {/* What you can do */}
                  <div
                    className="p-4 rounded-xl space-y-2"
                    style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}
                  >
                    <div className="text-xs font-semibold uppercase tracking-wide mb-2" style={{ color: 'var(--text-secondary)' }}>
                      {t('telegram.supportedCommands', language)}
                    </div>
                    {[
                      { cmd: '/help', desc: t('telegram.cmdHelp', language) },
                      { cmd: t('telegram.cmdStatus', language), desc: t('telegram.cmdNaturalLang', language) },
                      { cmd: t('telegram.cmdStartStop', language), desc: t('telegram.cmdControl', language) },
                      { cmd: t('telegram.cmdPositions', language), desc: t('telegram.cmdPositionsDesc', language) },
                      { cmd: t('telegram.cmdStrategy', language), desc: t('telegram.cmdStrategyDesc', language) },
                    ].map((item, i) => (
                      <div key={i} className="flex items-start gap-2 text-xs">
                        <code className="font-mono px-1.5 py-0.5 rounded flex-shrink-0" style={{ background: 'var(--panel-bg)', color: '#2AABEE' }}>
                          {item.cmd}
                        </code>
                        <span style={{ color: 'var(--text-secondary)' }}>{item.desc}</span>
                      </div>
                    ))}
                  </div>

                  <div className="flex gap-3">
                    <button
                      onClick={handleUnbind}
                      disabled={isUnbinding}
                      className="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-semibold transition-all disabled:opacity-50"
                      style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D', border: '1px solid rgba(246, 70, 93, 0.2)' }}
                    >
                      <Unlink className="w-4 h-4" />
                      {isUnbinding ? t('telegram.unbinding', language) : t('telegram.unbindAccount', language)}
                    </button>
                    <button
                      onClick={onClose}
                      className="flex-1 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02]"
                      style={{ background: '#2AABEE', color: '#000' }}
                    >
                      {t('telegram.done', language)}
                    </button>
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}

// BoundModelSelector — lets the user change the AI model when the bot is already active.
// It updates the model_id without requiring re-entry of the bot token.
function BoundModelSelector({
  language,
  models,
  currentModelId,
  onSaved,
}: {
  language: Language
  models: AIModel[]
  currentModelId: string
  onSaved: (modelId: string) => void
}) {
  const [modelId, setModelId] = useState(currentModelId)
  const [isSaving, setIsSaving] = useState(false)
  const visibleModels = filterVisibleModels(models)

  // Keep in sync if parent updates
  useEffect(() => { setModelId(currentModelId) }, [currentModelId])

  const handleSave = async () => {
    setIsSaving(true)
    try {
      // POST /api/telegram/model — lightweight endpoint for model-only update
      await api.updateTelegramModel(modelId)
      onSaved(modelId)
      toast.success(t('telegram.modelUpdated', language))
    } catch {
      toast.error(t('telegram.modelUpdateFailed', language))
    } finally {
      setIsSaving(false)
    }
  }

  if (visibleModels.length === 0) return null

  return (
    <div className="space-y-2">
      <label className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
        {t('telegram.aiModelLabel', language)}
      </label>
      <div className="flex gap-2">
        <select
          value={modelId}
          onChange={(e) => setModelId(e.target.value)}
          className="flex-1 px-3 py-2.5 rounded-xl text-sm appearance-none"
          style={{
            background: 'var(--panel-bg-solid)',
            border: '1px solid var(--panel-border)',
            color: modelId ? 'var(--text-primary)' : 'var(--text-secondary)',
          }}
        >
          <option value="">{t('telegram.aiModelAutoSelect', language)}</option>
          {visibleModels.map((m) => (
            <option key={m.id} value={m.id}>
              {m.name}{m.customModelName ? ` · ${m.customModelName}` : ''}
            </option>
          ))}
        </select>
        <button
          onClick={handleSave}
          disabled={isSaving || modelId === currentModelId}
          className="px-4 py-2.5 rounded-xl text-sm font-bold transition-all hover:scale-[1.02] disabled:opacity-40 disabled:cursor-not-allowed"
          style={{ background: '#F0B90B', color: '#000', whiteSpace: 'nowrap' }}
        >
          {isSaving ? '...' : t('telegram.save', language)}
        </button>
      </div>
    </div>
  )
}
