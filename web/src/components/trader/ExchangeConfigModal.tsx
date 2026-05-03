import React, { useState, useEffect } from 'react'
import type { Exchange, ProxyServer } from '../../types'
import { t, type Language } from '../../i18n/translations'
import { getExchangeIcon } from '../common/ExchangeIcons'
import {
  TwoStageKeyModal,
  type TwoStageKeyModalResult,
} from '../modals/TwoStageKeyModal'
import {
  WebCryptoEnvironmentCheck,
  type WebCryptoCheckStatus,
} from '../common/WebCryptoEnvironmentCheck'
import {
  BookOpen, Trash2, HelpCircle, ExternalLink, UserPlus,
  Key, Shield, ChevronLeft, Check, ArrowRight
} from 'lucide-react'
import { toast } from 'sonner'
import { Tooltip } from './Tooltip'
import { getShortName } from './utils'

// Supported exchange templates
const SUPPORTED_EXCHANGE_TEMPLATES = [
  { exchange_type: 'binance', name: 'Binance Futures', type: 'cex' as const },
  { exchange_type: 'bybit', name: 'Bybit Futures', type: 'cex' as const },
  { exchange_type: 'okx', name: 'OKX Futures', type: 'cex' as const },
  { exchange_type: 'bitget', name: 'Bitget Futures', type: 'cex' as const },
  { exchange_type: 'gate', name: 'Gate.io Futures', type: 'cex' as const },
  { exchange_type: 'kucoin', name: 'KuCoin Futures', type: 'cex' as const },
  { exchange_type: 'hyperliquid', name: 'Hyperliquid', type: 'dex' as const },
  { exchange_type: 'aster', name: 'Aster DEX', type: 'dex' as const },
  { exchange_type: 'lighter', name: 'Lighter', type: 'dex' as const },
  { exchange_type: 'indodax', name: 'Indodax', type: 'cex' as const },
]

interface ExchangeConfigModalProps {
  allExchanges: Exchange[]
  proxyServers: ProxyServer[]
  editingExchangeId: string | null
  onSave: (
    exchangeId: string | null,
    exchangeType: string,
    accountName: string,
    apiKey: string,
    proxyUrl?: string,
    secretKey?: string,
    passphrase?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string,
    lighterWalletAddr?: string,
    lighterPrivateKey?: string,
    lighterApiKeyPrivateKey?: string,
    lighterApiKeyIndex?: number,
    proxyServerId?: string
  ) => Promise<void>
  onDelete: (exchangeId: string) => void
  onClose: () => void
  language: Language
}

// Step indicator component
function StepIndicator({ currentStep, labels }: { currentStep: number; labels: string[] }) {
  return (
    <div className="flex items-center justify-center gap-2 mb-6">
      {labels.map((label, index) => (
        <React.Fragment key={index}>
          <div className="flex items-center gap-2">
            <div
              className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-all"
              style={{
                background: index < currentStep ? '#0ECB81' : index === currentStep ? '#F0B90B' : 'var(--panel-border)',
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

// Exchange card component
function ExchangeCard({
  template,
  selected,
  onClick,
  disabled,
}: {
  template: typeof SUPPORTED_EXCHANGE_TEMPLATES[0]
  selected: boolean
  onClick: () => void
  disabled?: boolean
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="flex flex-col items-center gap-2 p-4 rounded-xl transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100"
      style={{
        background: selected ? 'rgba(240, 185, 11, 0.15)' : 'var(--panel-bg)',
        border: selected ? '2px solid #F0B90B' : '2px solid var(--panel-border)',
      }}
    >
      <div className="relative">
        {getExchangeIcon(template.exchange_type, { width: 48, height: 48 })}
        {selected && (
          <div
            className="absolute -top-1 -right-1 w-5 h-5 rounded-full flex items-center justify-center"
            style={{ background: '#0ECB81' }}
          >
            <Check className="w-3 h-3 text-black" />
          </div>
        )}
      </div>
      <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
        {getShortName(template.name)}
      </span>
      <span
        className="text-xs px-2 py-0.5 rounded-full"
        style={{
          background: template.type === 'cex' ? 'rgba(240, 185, 11, 0.2)' : 'rgba(139, 92, 246, 0.2)',
          color: template.type === 'cex' ? '#F0B90B' : '#A78BFA',
        }}
      >
        {template.type.toUpperCase()}
      </span>
    </button>
  )
}

export function ExchangeConfigModal({
  allExchanges,
  proxyServers,
  editingExchangeId,
  onSave,
  onDelete,
  onClose,
  language,
}: ExchangeConfigModalProps) {
  // Step: 0 = select exchange, 1 = configure
  const [currentStep, setCurrentStep] = useState(editingExchangeId ? 1 : 0)
  const [selectedExchangeType, setSelectedExchangeType] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [testnet, setTestnet] = useState(false)
  const [showGuide, setShowGuide] = useState(false)
  const [webCryptoStatus, setWebCryptoStatus] = useState<WebCryptoCheckStatus>('idle')
  const [showBinanceGuide, setShowBinanceGuide] = useState(false)

  // Aster fields
  const [asterUser, setAsterUser] = useState('')
  const [asterSigner, setAsterSigner] = useState('')
  const [asterPrivateKey, setAsterPrivateKey] = useState('')

  // Hyperliquid fields
  const [hyperliquidWalletAddr, setHyperliquidWalletAddr] = useState('')

  // Lighter fields
  const [lighterWalletAddr, setLighterWalletAddr] = useState('')
  const [lighterApiKeyPrivateKey, setLighterApiKeyPrivateKey] = useState('')
  const [lighterApiKeyIndex, setLighterApiKeyIndex] = useState(0)

  // Other state
  const [secureInputTarget, setSecureInputTarget] = useState<null | 'hyperliquid' | 'aster' | 'lighter'>(null)
  const [isSaving, setIsSaving] = useState(false)
  const [accountName, setAccountName] = useState('')
  const [selectedProxyServerId, setSelectedProxyServerId] = useState('')

  const selectedExchange = editingExchangeId
    ? allExchanges?.find((e) => e.id === editingExchangeId)
    : null

  const selectedTemplate = editingExchangeId
    ? SUPPORTED_EXCHANGE_TEMPLATES.find((t) => t.exchange_type === selectedExchange?.exchange_type)
    : SUPPORTED_EXCHANGE_TEMPLATES.find((t) => t.exchange_type === selectedExchangeType)

  const currentExchangeType = editingExchangeId
    ? selectedExchange?.exchange_type
    : selectedExchangeType

  const availableProxyServers = proxyServers.filter((proxy) => {
    return !proxy.bound_exchange_id || proxy.bound_exchange_id === editingExchangeId
  })

  const selectedProxyServer = proxyServers.find((proxy) => proxy.id === selectedProxyServerId)

  const exchangeRegistrationLinks: Record<string, { url: string; hasReferral?: boolean }> = {
    binance: { url: 'https://www.bsmkweb.cc/register?ref=AFX5698', hasReferral: true },
    okx: { url: 'https://www.glneokotyjv.com/join/10877646', hasReferral: true },
    bybit: { url: 'https://partner.bybit.com/' },
    bitget: { url: 'https://www.bitget.com/' },
    gate: { url: 'https://www.gatenode.xyz/' },
    kucoin: { url: 'https://www.kucoin.com' },
    hyperliquid: { url: 'https://app.hyperliquid.xyz/' },
    aster: { url: 'https://www.asterdex.com/' },
    lighter: { url: 'https://app.lighter.xyz' },
    indodax: { url: 'https://indodax.com/' },
  }

  // Initialize form when editing
  useEffect(() => {
    if (editingExchangeId && selectedExchange) {
      setAccountName(selectedExchange.account_name || '')
      setSelectedProxyServerId(selectedExchange.proxy_server_id || '')
      setApiKey(selectedExchange.apiKey || '')
      setSecretKey(selectedExchange.secretKey || '')
      setPassphrase('')
      setTestnet(selectedExchange.testnet || false)
      setAsterUser(selectedExchange.asterUser || '')
      setAsterSigner(selectedExchange.asterSigner || '')
      setAsterPrivateKey('')
      setHyperliquidWalletAddr(selectedExchange.hyperliquidWalletAddr || '')
      setLighterWalletAddr(selectedExchange.lighterWalletAddr || '')
      setLighterApiKeyPrivateKey('')
      setLighterApiKeyIndex(selectedExchange.lighterApiKeyIndex || 0)
    }
  }, [editingExchangeId, selectedExchange])

  useEffect(() => {
    if (!editingExchangeId) {
      setSelectedProxyServerId('')
    }
  }, [editingExchangeId, selectedExchangeType])

  const secureInputContextLabel =
    secureInputTarget === 'aster' ? t('asterExchangeName', language)
      : secureInputTarget === 'hyperliquid' ? t('hyperliquidExchangeName', language)
        : undefined

  const handleSecureInputComplete = ({ value }: TwoStageKeyModalResult) => {
    const trimmed = value.trim()
    if (secureInputTarget === 'hyperliquid') setApiKey(trimmed)
    if (secureInputTarget === 'aster') setAsterPrivateKey(trimmed)
    if (secureInputTarget === 'lighter') {
      setLighterApiKeyPrivateKey(trimmed)
      toast.success(t('lighterApiKeyImported', language))
    }
    setSecureInputTarget(null)
  }

  const maskSecret = (secret: string) => {
    if (!secret || secret.length === 0) return ''
    if (secret.length <= 8) return '*'.repeat(secret.length)
    return secret.slice(0, 4) + '*'.repeat(Math.max(secret.length - 8, 4)) + secret.slice(-4)
  }

  const handleSelectExchange = (exchangeType: string) => {
    setSelectedExchangeType(exchangeType)
    setCurrentStep(1)
  }

  const handleBack = () => {
    if (editingExchangeId) {
      onClose()
    } else {
      setCurrentStep(0)
      setSelectedExchangeType('')
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (isSaving) return
    if (!editingExchangeId && !selectedExchangeType) return

    const trimmedAccountName = accountName.trim()
    if (!trimmedAccountName) {
      toast.error(t('exchangeConfig.pleaseEnterAccountName', language))
      return
    }

    const exchangeId = editingExchangeId || null
    const exchangeType = currentExchangeType || ''
    const proxyServerId = selectedProxyServerId.trim()

    if (!proxyServerId) {
      toast.error(
        language === 'zh'
          ? '请先在代理服务器页面创建并选择一个代理服务器。'
          : 'Create and select a proxy server before saving.'
      )
      return
    }

    setIsSaving(true)
    try {
      if (currentExchangeType === 'binance' || currentExchangeType === 'bybit' || currentExchangeType === 'indodax') {
        if (!apiKey.trim() || !secretKey.trim()) return
        await onSave(exchangeId, exchangeType, trimmedAccountName, apiKey.trim(), '', secretKey.trim(), '', testnet, undefined, undefined, undefined, undefined, undefined, undefined, undefined, undefined, proxyServerId)
      } else if (currentExchangeType === 'okx' || currentExchangeType === 'bitget' || currentExchangeType === 'kucoin') {
        if (!apiKey.trim() || !secretKey.trim() || !passphrase.trim()) return
        await onSave(exchangeId, exchangeType, trimmedAccountName, apiKey.trim(), '', secretKey.trim(), passphrase.trim(), testnet, undefined, undefined, undefined, undefined, undefined, undefined, undefined, undefined, proxyServerId)
      } else if (currentExchangeType === 'hyperliquid') {
        if (!apiKey.trim() || !hyperliquidWalletAddr.trim()) return
        await onSave(exchangeId, exchangeType, trimmedAccountName, apiKey.trim(), '', '', '', testnet, hyperliquidWalletAddr.trim(), undefined, undefined, undefined, undefined, undefined, undefined, undefined, proxyServerId)
      } else if (currentExchangeType === 'aster') {
        if (!asterUser.trim() || !asterSigner.trim() || !asterPrivateKey.trim()) return
        await onSave(exchangeId, exchangeType, trimmedAccountName, '', '', '', '', testnet, undefined, asterUser.trim(), asterSigner.trim(), asterPrivateKey.trim(), undefined, undefined, undefined, undefined, proxyServerId)
      } else if (currentExchangeType === 'lighter') {
        if (!lighterWalletAddr.trim() || !lighterApiKeyPrivateKey.trim()) return
        await onSave(exchangeId, exchangeType, trimmedAccountName, '', '', '', '', testnet, undefined, undefined, undefined, undefined, lighterWalletAddr.trim(), '', lighterApiKeyPrivateKey.trim(), lighterApiKeyIndex, proxyServerId)
      } else {
        if (!apiKey.trim() || !secretKey.trim()) return
        await onSave(exchangeId, exchangeType, trimmedAccountName, apiKey.trim(), '', secretKey.trim(), '', testnet, undefined, undefined, undefined, undefined, undefined, undefined, undefined, undefined, proxyServerId)
      }
    } finally {
      setIsSaving(false)
    }
  }

  const stepLabels = [t('exchangeConfig.selectExchange', language), t('exchangeConfig.configure', language)]
  const cexExchanges = SUPPORTED_EXCHANGE_TEMPLATES.filter(t => t.type === 'cex')
  const dexExchanges = SUPPORTED_EXCHANGE_TEMPLATES.filter(t => t.type === 'dex')
  const accentCardStyle = {
    background: 'linear-gradient(180deg, color-mix(in srgb, var(--panel-bg-solid) 82%, #fff 18%) 0%, var(--panel-bg) 100%)',
    border: '1px solid color-mix(in srgb, var(--panel-border) 72%, #F0B90B 28%)',
  }
  const coolCardStyle = {
    background: 'linear-gradient(180deg, color-mix(in srgb, var(--panel-bg-solid) 88%, #fff 12%) 0%, var(--panel-bg) 100%)',
    border: '1px solid color-mix(in srgb, var(--panel-border) 80%, #7C9EF8 20%)',
  }
  const violetCardStyle = {
    background: 'linear-gradient(180deg, color-mix(in srgb, var(--panel-bg-solid) 88%, #fff 12%) 0%, var(--panel-bg) 100%)',
    border: '1px solid color-mix(in srgb, var(--panel-border) 76%, #A78BFA 24%)',
  }
  const tealCardStyle = {
    background: 'linear-gradient(180deg, color-mix(in srgb, var(--panel-bg-solid) 88%, #fff 12%) 0%, var(--panel-bg) 100%)',
    border: '1px solid color-mix(in srgb, var(--panel-border) 76%, #7FE7CC 24%)',
  }
  const blueCardStyle = {
    background: 'linear-gradient(180deg, color-mix(in srgb, var(--panel-bg-solid) 88%, #fff 12%) 0%, var(--panel-bg) 100%)',
    border: '1px solid color-mix(in srgb, var(--panel-border) 76%, #60A5FA 24%)',
  }

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4 overflow-y-auto backdrop-blur-sm">
      <div
        className="rounded-2xl w-full max-w-2xl relative my-8 shadow-2xl"
        style={{ background: 'linear-gradient(180deg, var(--panel-bg-solid) 0%, var(--panel-bg) 100%)', border: '1px solid var(--panel-border)', maxHeight: 'calc(100vh - 4rem)' }}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 pb-2">
          <div className="flex items-center gap-3">
            {currentStep > 0 && !editingExchangeId && (
              <button type="button" onClick={handleBack} className="p-2 rounded-lg transition-colors">
                <ChevronLeft className="w-5 h-5" style={{ color: 'var(--text-secondary)' }} />
              </button>
            )}
            <h3 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>
              {editingExchangeId ? t('editExchange', language) : t('addExchange', language)}
            </h3>
          </div>
          <div className="flex items-center gap-2">
            {currentExchangeType === 'binance' && currentStep === 1 && (
              <button
                type="button"
                onClick={() => setShowGuide(true)}
                className="px-3 py-2 rounded-lg text-sm font-semibold transition-all hover:scale-105 flex items-center gap-2"
                style={{ background: 'rgba(240, 185, 11, 0.1)', color: '#F0B90B' }}
              >
                <BookOpen className="w-4 h-4" />
                {t('viewGuide', language)}
              </button>
            )}
            {editingExchangeId && (
              <button
                type="button"
                onClick={() => onDelete(editingExchangeId)}
                className="p-2 rounded-lg hover:bg-red-500/20 transition-colors"
                style={{ color: '#F6465D' }}
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
            <button type="button" onClick={onClose} className="p-2 rounded-lg transition-colors" style={{ color: 'var(--text-secondary)' }}>
              ✕
            </button>
          </div>
        </div>

        {/* Step Indicator */}
        {!editingExchangeId && (
          <div className="px-6">
            <StepIndicator currentStep={currentStep} labels={stepLabels} />
          </div>
        )}

        {/* Content */}
        <div className="px-6 pb-6 overflow-y-auto" style={{ maxHeight: 'calc(100vh - 16rem)' }}>
          {/* Step 0: Select Exchange */}
          {currentStep === 0 && !editingExchangeId && (
            <div className="space-y-6">
              {/* WebCrypto Check */}
              <div className="space-y-2">
                <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide" style={{ color: 'var(--text-secondary)' }}>
                  <Shield className="w-4 h-4" />
                  {t('environmentSteps.checkTitle', language)}
                </div>
                <WebCryptoEnvironmentCheck language={language} variant="card" onStatusChange={setWebCryptoStatus} />
              </div>

              {/* Exchange Grid */}
              <div className="space-y-4">
                <div className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                  {t('exchangeConfig.chooseExchange', language)}
                </div>

                {/* CEX */}
                <div className="space-y-3">
                  <div className="text-xs font-medium uppercase tracking-wide" style={{ color: '#F0B90B' }}>
                    {t('exchangeConfig.centralizedExchanges', language)}
                  </div>
                  <div className="grid grid-cols-3 sm:grid-cols-5 gap-3">
                    {cexExchanges.map((template) => (
                      <ExchangeCard
                        key={template.exchange_type}
                        template={template}
                        selected={selectedExchangeType === template.exchange_type}
                        onClick={() => handleSelectExchange(template.exchange_type)}
                        disabled={webCryptoStatus !== 'secure' && webCryptoStatus !== 'disabled'}
                      />
                    ))}
                  </div>
                </div>

                {/* DEX */}
                <div className="space-y-3">
                  <div className="text-xs font-medium uppercase tracking-wide" style={{ color: '#A78BFA' }}>
                    {t('exchangeConfig.decentralizedExchanges', language)}
                  </div>
                  <div className="grid grid-cols-3 sm:grid-cols-5 gap-3">
                    {dexExchanges.map((template) => (
                      <ExchangeCard
                        key={template.exchange_type}
                        template={template}
                        selected={selectedExchangeType === template.exchange_type}
                        onClick={() => handleSelectExchange(template.exchange_type)}
                        disabled={webCryptoStatus !== 'secure' && webCryptoStatus !== 'disabled'}
                      />
                    ))}
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Step 1: Configure */}
          {(currentStep === 1 || editingExchangeId) && selectedTemplate && (
            <form onSubmit={handleSubmit} className="space-y-5">
              {/* Selected Exchange Header */}
              <div className="p-4 rounded-xl flex items-center gap-4" style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}>
                {getExchangeIcon(selectedTemplate.exchange_type, { width: 48, height: 48 })}
                <div className="flex-1">
                  <div className="font-semibold text-lg" style={{ color: 'var(--text-primary)' }}>
                    {getShortName(selectedTemplate.name)}
                  </div>
                  <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                    {selectedTemplate.type.toUpperCase()} • {selectedTemplate.exchange_type}
                  </div>
                </div>
                <a
                  href={exchangeRegistrationLinks[currentExchangeType || '']?.url || '#'}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-2 px-4 py-2 rounded-lg transition-all hover:scale-105"
                  style={accentCardStyle}
                >
                  <UserPlus className="w-4 h-4" style={{ color: '#F0B90B' }} />
                  <span className="text-sm font-medium" style={{ color: '#F0B90B' }}>
                    {t('exchangeConfig.register', language)}
                  </span>
                  {exchangeRegistrationLinks[currentExchangeType || '']?.hasReferral && (
                    <span className="text-xs px-1.5 py-0.5 rounded" style={{ background: 'rgba(14, 203, 129, 0.2)', color: '#0ECB81' }}>
                      {t('exchangeConfig.bonus', language)}
                    </span>
                  )}
                </a>
              </div>

              {/* Account Name */}
              <div className="space-y-2">
                <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                  <Key className="w-4 h-4" style={{ color: '#F0B90B' }} />
                  {t('exchangeConfig.accountName', language)} *
                </label>
                <input
                  type="text"
                  value={accountName}
                  onChange={(e) => setAccountName(e.target.value)}
                  placeholder={t('exchangeConfig.accountNamePlaceholder', language)}
                  className="w-full px-4 py-3 rounded-xl text-base"
                  style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                  required
                />
              </div>

              <div className="space-y-2">
                <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                  <Shield className="w-4 h-4" style={{ color: '#F0B90B' }} />
                  {language === 'zh' ? '代理服务器（一个交易所 API 绑定一个代理）' : 'Proxy Server'} *
                </label>
                <select
                  value={selectedProxyServerId}
                  onChange={(e) => setSelectedProxyServerId(e.target.value)}
                  className="w-full px-4 py-3 rounded-xl text-base"
                  style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                  required
                >
                  <option value="">{language === 'zh' ? '请选择已测试通过的代理服务器' : 'Select a tested proxy server'}</option>
                  {availableProxyServers.map((proxy) => (
                    <option key={proxy.id} value={proxy.id}>
                      {proxy.name}{proxy.last_exit_ip ? ` (${proxy.last_exit_ip})` : ''}
                    </option>
                  ))}
                </select>
                <div className="rounded-xl px-4 py-3 text-xs leading-6" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-secondary)' }}>
                  {selectedProxyServer ? (
                    <>
                      <div style={{ color: 'var(--text-primary)' }}>
                        {language === 'zh' ? '当前选择：' : 'Selected: '}
                        {selectedProxyServer.name}
                      </div>
                      <div>
                        {language === 'zh' ? '代理出口 IP：' : 'Proxy exit IP: '}
                        <span className="font-mono" style={{ color: 'var(--text-primary)' }}>{selectedProxyServer.last_exit_ip || '-'}</span>
                      </div>
                      <div>
                        {language === 'zh' ? '交易所 API 白名单应填写该出口 IP。' : 'Use this exit IP in the exchange API whitelist.'}
                      </div>
                    </>
                  ) : (
                    <div>
                      {language === 'zh'
                        ? '请先到“代理服务器”页面新增代理；保存时仅绑定代理 ID，不再在交易所配置里保存新的裸代理地址。'
                        : 'Create a proxy in the Proxies tab first. Exchange config now binds a proxy ID instead of storing a new raw proxy URL.'}
                    </div>
                  )}
                </div>
              </div>

              {/* CEX Fields */}
              {(currentExchangeType === 'binance' || currentExchangeType === 'bybit' || currentExchangeType === 'okx' || currentExchangeType === 'bitget' || currentExchangeType === 'gate' || currentExchangeType === 'kucoin' || currentExchangeType === 'indodax') && (
                <>
                  {currentExchangeType === 'binance' && (
                    <div
                      className="p-4 rounded-xl cursor-pointer transition-colors"
                      style={coolCardStyle}
                      onClick={() => setShowBinanceGuide(!showBinanceGuide)}
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <span style={{ color: '#5B8DEF' }}>ℹ️</span>
                          <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                            {t('exchangeConfig.useBinanceFuturesApi', language)}
                          </span>
                        </div>
                        <span style={{ color: 'var(--text-secondary)' }}>{showBinanceGuide ? '▲' : '▼'}</span>
                      </div>
                      {showBinanceGuide && (
                        <div className="mt-3 pt-3 text-sm" style={{ borderTop: '1px solid var(--panel-border)', color: 'var(--text-secondary)' }}>
                          <a
                            href="https://www.binance.com/zh-CN/support/faq/how-to-create-api-keys-on-binance-360002502072"
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-1 hover:underline"
                            style={{ color: '#5B8DEF' }}
                            onClick={(e) => e.stopPropagation()}
                          >
                            {t('exchangeConfig.viewTutorial', language)} <ExternalLink className="w-3 h-3" />
                          </a>
                        </div>
                      )}
                    </div>
                  )}

                  <div className="space-y-2">
                    <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                      <Key className="w-4 h-4" style={{ color: '#F0B90B' }} />
                      {t('apiKey', language)}
                    </label>
                    <input
                      type="password"
                      value={apiKey}
                      onChange={(e) => setApiKey(e.target.value)}
                      placeholder={t('enterAPIKey', language)}
                      className="w-full px-4 py-3 rounded-xl"
                      style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                      required
                    />
                  </div>

                  <div className="space-y-2">
                    <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                      <Shield className="w-4 h-4" style={{ color: '#F0B90B' }} />
                      {t('secretKey', language)}
                    </label>
                    <input
                      type="password"
                      value={secretKey}
                      onChange={(e) => setSecretKey(e.target.value)}
                      placeholder={t('enterSecretKey', language)}
                      className="w-full px-4 py-3 rounded-xl"
                      style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                      required
                    />
                  </div>

                  {(currentExchangeType === 'okx' || currentExchangeType === 'bitget' || currentExchangeType === 'kucoin') && (
                    <div className="space-y-2">
                      <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                        <Key className="w-4 h-4" style={{ color: '#F0B90B' }} />
                        {t('passphrase', language)}
                      </label>
                      <input
                        type="password"
                        value={passphrase}
                        onChange={(e) => setPassphrase(e.target.value)}
                        placeholder={t('enterPassphrase', language)}
                        className="w-full px-4 py-3 rounded-xl"
                        style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                        required
                      />
                    </div>
                  )}

                </>
              )}

              {/* Aster Fields */}
              {currentExchangeType === 'aster' && (
                <>
                  <div className="p-4 rounded-xl" style={violetCardStyle}>
                    <div className="flex items-start gap-2">
                      <span style={{ fontSize: '16px' }}>🔐</span>
                      <div>
                        <div className="text-sm font-semibold mb-1" style={{ color: '#A78BFA' }}>{t('asterApiProTitle', language)}</div>
                        <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>{t('asterApiProDesc', language)}</div>
                      </div>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                      {t('asterUserLabel', language)}
                      <Tooltip content={t('asterUserDesc', language)}>
                        <HelpCircle className="w-4 h-4 cursor-help" style={{ color: '#A78BFA' }} />
                      </Tooltip>
                    </label>
                    <input type="text" value={asterUser} onChange={(e) => setAsterUser(e.target.value)} placeholder={t('enterAsterUser', language)} className="w-full px-4 py-3 rounded-xl" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }} required />
                  </div>
                  <div className="space-y-2">
                    <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                      {t('asterSignerLabel', language)}
                      <Tooltip content={t('asterSignerDesc', language)}>
                        <HelpCircle className="w-4 h-4 cursor-help" style={{ color: '#A78BFA' }} />
                      </Tooltip>
                    </label>
                    <input type="text" value={asterSigner} onChange={(e) => setAsterSigner(e.target.value)} placeholder={t('enterAsterSigner', language)} className="w-full px-4 py-3 rounded-xl" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }} required />
                  </div>
                  <div className="space-y-2">
                    <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                      {t('asterPrivateKeyLabel', language)}
                      <Tooltip content={t('asterPrivateKeyDesc', language)}>
                        <HelpCircle className="w-4 h-4 cursor-help" style={{ color: '#A78BFA' }} />
                      </Tooltip>
                    </label>
                    <input type="password" value={asterPrivateKey} onChange={(e) => setAsterPrivateKey(e.target.value)} placeholder={t('enterAsterPrivateKey', language)} className="w-full px-4 py-3 rounded-xl" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }} required />
                  </div>
                </>
              )}

              {/* Hyperliquid Fields */}
              {currentExchangeType === 'hyperliquid' && (
                <>
                  <div className="p-4 rounded-xl" style={tealCardStyle}>
                    <div className="flex items-start gap-2">
                      <span style={{ fontSize: '16px' }}>🔐</span>
                      <div>
                        <div className="text-sm font-semibold mb-1" style={{ color: '#7FE7CC' }}>{t('hyperliquidAgentWalletTitle', language)}</div>
                        <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>{t('hyperliquidAgentWalletDesc', language)}</div>
                      </div>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>{t('hyperliquidAgentPrivateKey', language)}</label>
                    <div className="flex gap-2">
                      <input type="text" value={maskSecret(apiKey)} readOnly placeholder={t('enterHyperliquidAgentPrivateKey', language)} className="flex-1 px-4 py-3 rounded-xl" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }} />
                      <button type="button" onClick={() => setSecureInputTarget('hyperliquid')} className="px-4 py-3 rounded-xl text-sm font-semibold transition-all hover:scale-105" style={{ background: '#7FE7CC', color: '#000' }}>
                        {apiKey ? t('secureInputReenter', language) : t('secureInputButton', language)}
                      </button>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>{t('hyperliquidMainWalletAddress', language)}</label>
                    <input type="text" value={hyperliquidWalletAddr} onChange={(e) => setHyperliquidWalletAddr(e.target.value)} placeholder={t('enterHyperliquidMainWalletAddress', language)} className="w-full px-4 py-3 rounded-xl" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }} required />
                  </div>
                </>
              )}

              {/* Lighter Fields */}
              {currentExchangeType === 'lighter' && (
                <>
                  <div className="p-4 rounded-xl" style={blueCardStyle}>
                    <div className="flex items-start gap-2">
                      <span style={{ fontSize: '16px' }}>🔐</span>
                      <div>
                        <div className="text-sm font-semibold mb-1" style={{ color: '#3B82F6' }}>
                          {t('exchangeConfig.lighterApiKeySetup', language)}
                        </div>
                        <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                          {t('exchangeConfig.lighterApiKeyDesc', language)}
                        </div>
                      </div>
                    </div>
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>{t('lighterWalletAddress', language)} *</label>
                    <input type="text" value={lighterWalletAddr} onChange={(e) => setLighterWalletAddr(e.target.value)} placeholder={t('enterLighterWalletAddress', language)} className="w-full px-4 py-3 rounded-xl" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }} required />
                  </div>
                  <div className="space-y-2">
                    <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                      {t('lighterApiKeyPrivateKey', language)} *
                      <button type="button" onClick={() => setSecureInputTarget('lighter')} className="text-xs underline" style={{ color: '#3B82F6' }}>{t('secureInputButton', language)}</button>
                    </label>
                    <input type="password" value={lighterApiKeyPrivateKey} onChange={(e) => setLighterApiKeyPrivateKey(e.target.value)} placeholder={t('enterLighterApiKeyPrivateKey', language)} className="w-full px-4 py-3 rounded-xl font-mono" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }} required />
                  </div>
                  <div className="space-y-2">
                    <label className="flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                      {t('exchangeConfig.apiKeyIndex', language)}
                      <Tooltip content={t('exchangeConfig.apiKeyIndexTooltip', language)}>
                        <HelpCircle className="w-4 h-4 cursor-help" style={{ color: '#3B82F6' }} />
                      </Tooltip>
                    </label>
                    <input type="number" min={0} max={255} value={lighterApiKeyIndex} onChange={(e) => setLighterApiKeyIndex(parseInt(e.target.value) || 0)} className="w-full px-4 py-3 rounded-xl" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }} />
                  </div>
                </>
              )}

              {/* Buttons */}
              <div className="flex gap-3 pt-4">
                <button type="button" onClick={handleBack} className="flex-1 px-4 py-3 rounded-xl text-sm font-semibold transition-all" style={{ background: 'var(--panel-bg-solid)', color: 'var(--text-secondary)', border: '1px solid var(--panel-border)' }}>
                  {editingExchangeId ? t('cancel', language) : t('exchangeConfig.back', language)}
                </button>
                <button
                  type="submit"
                  disabled={isSaving || !accountName.trim()}
                  className="flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl text-sm font-bold transition-all hover:scale-[1.02] disabled:opacity-50 disabled:cursor-not-allowed"
                  style={{ background: '#F0B90B', color: '#000' }}
                >
                  {isSaving ? t('saving', language) : (
                    <>{t('saveConfig', language)} <ArrowRight className="w-4 h-4" /></>
                  )}
                </button>
              </div>
            </form>
          )}
        </div>
      </div>

      {/* Binance Guide Modal */}
      {showGuide && (
        <div className="fixed inset-0 bg-black/75 flex items-center justify-center z-50 p-4" onClick={() => setShowGuide(false)}>
          <div className="rounded-2xl p-6 w-full max-w-4xl" style={{ background: 'linear-gradient(180deg, var(--panel-bg-solid) 0%, var(--panel-bg) 100%)', border: '1px solid var(--panel-border)' }} onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-xl font-bold flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
                <BookOpen className="w-6 h-6" style={{ color: '#F0B90B' }} />
                {t('binanceSetupGuide', language)}
              </h3>
              <button onClick={() => setShowGuide(false)} className="px-4 py-2 rounded-lg text-sm font-semibold" style={{ background: 'var(--panel-bg-solid)', color: 'var(--text-secondary)', border: '1px solid var(--panel-border)' }}>
                {t('closeGuide', language)}
              </button>
            </div>
            <div className="overflow-y-auto max-h-[80vh]">
              <img src="/images/guide.png" alt={t('binanceSetupGuide', language)} className="w-full h-auto rounded-lg" />
            </div>
          </div>
        </div>
      )}

      {/* Secure Input Modal */}
      <TwoStageKeyModal
        isOpen={secureInputTarget !== null}
        language={language}
        contextLabel={secureInputContextLabel}
        expectedLength={64}
        onCancel={() => setSecureInputTarget(null)}
        onComplete={handleSecureInputComplete}
      />
    </div>
  )
}
