import {
  Brain,
  Landmark,
  Server,
  Eye,
  EyeOff,
  Copy,
  Check,
} from 'lucide-react'
import type { AIModel, Exchange, ProxyServer } from '../../types'
import type { Language } from '../../i18n/translations'
import { t } from '../../i18n/translations'
import { getModelIcon } from '../common/ModelIcons'
import { getExchangeIcon } from '../common/ExchangeIcons'
import {
  AI_PROVIDER_CONFIG,
  getShortName,
  truncateAddress,
} from './model-constants'

interface UsageInfo {
  runningCount: number
  totalCount: number
}

interface ConfigStatusGridProps {
  configuredModels: AIModel[]
  configuredExchanges: Exchange[]
  configuredProxyServers: ProxyServer[]
  visibleExchangeAddresses: Set<string>
  copiedId: string | null
  language: Language
  isModelInUse: (modelId: string) => boolean | undefined
  getModelUsageInfo: (modelId: string) => UsageInfo
  isExchangeInUse: (exchangeId: string) => boolean | undefined
  getExchangeUsageInfo: (exchangeId: string) => UsageInfo
  onModelClick: (modelId: string) => void
  onExchangeClick: (exchangeId: string) => void
  onProxyClick: () => void
  onToggleExchangeAddress: (exchangeId: string) => void
  onCopyAddress: (id: string, address: string) => void
}

export function ConfigStatusGrid({
  configuredModels,
  configuredExchanges,
  configuredProxyServers,
  visibleExchangeAddresses,
  copiedId,
  language,
  isModelInUse,
  getModelUsageInfo,
  isExchangeInUse,
  getExchangeUsageInfo,
  onModelClick,
  onExchangeClick,
  onProxyClick,
  onToggleExchangeAddress,
  onCopyAddress,
}: ConfigStatusGridProps) {
  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
      {/* AI Models Card */}
      <div className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--panel-border)', background: 'var(--panel-bg)' }}>
        <div className="px-4 py-3 flex items-center gap-2 backdrop-blur-sm" style={{ borderBottom: '1px solid var(--panel-border)', background: 'color-mix(in srgb, var(--panel-bg-solid) 86%, transparent)' }}>
          <Brain className="w-4 h-4 text-nofx-gold" />
          <h3 className="text-sm font-mono font-semibold tracking-widest uppercase" style={{ color: 'var(--text-primary)' }}>
            {t('aiModels', language)}
          </h3>
        </div>

        <div className="p-4 space-y-3">
          {configuredModels.map((model) => {
            const inUse = isModelInUse(model.id)
            const usageInfo = getModelUsageInfo(model.id)
            return (
              <div
                key={model.id}
                className={`group relative flex items-center justify-between p-3 rounded-md transition-all border ${inUse ? 'opacity-80' : 'cursor-pointer'}`}
                style={{ background: 'var(--panel-bg-solid)', borderColor: 'var(--panel-border)' }}
                onClick={() => onModelClick(model.id)}
              >
                <div className="flex items-center gap-4">
                  <div className="relative">
                    <div className="absolute inset-0 bg-indigo-500/20 rounded-full blur-sm group-hover:bg-indigo-500/30 transition-all"></div>
                    <div className="w-10 h-10 rounded-full flex items-center justify-center relative z-10" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}>
                      {getModelIcon(model.provider || model.id, { width: 20, height: 20 }) || (
                        <span className="text-xs font-bold text-indigo-400">{model.name[0]}</span>
                      )}
                    </div>
                  </div>

                  <div className="min-w-0">
                    <div className="font-mono text-[15px] font-semibold transition-colors" style={{ color: 'var(--text-primary)' }}>
                      {model.name}
                    </div>
                    <div className="text-xs font-mono font-medium flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
                      {model.customModelName || AI_PROVIDER_CONFIG[model.provider]?.defaultModel || ''}
                    </div>
                  </div>
                </div>

                <div className="text-right">
                  {usageInfo.totalCount > 0 ? (
                    <span className={`text-[10px] font-mono font-semibold px-2 py-1 rounded border ${usageInfo.runningCount > 0
                      ? 'bg-green-500/10 border-green-500/30 text-green-400'
                      : 'bg-yellow-500/10 border-yellow-500/30 text-yellow-400'
                      }`}>
                      {usageInfo.runningCount}/{usageInfo.totalCount} ACTIVE
                    </span>
                  ) : (
                    <span className="text-[10px] font-mono font-semibold uppercase tracking-wider" style={{ color: 'var(--text-secondary)' }}>
                      {language === 'zh' ? '就绪' : 'STANDBY'}
                    </span>
                  )}
                </div>
              </div>
            )
          })}

          {configuredModels.length === 0 && (
            <div className="text-center py-10 border border-dashed rounded-lg" style={{ borderColor: 'var(--panel-border)', background: 'var(--panel-bg-solid)' }}>
              <Brain className="w-8 h-8 mx-auto mb-3" style={{ color: 'var(--text-secondary)' }} />
              <div className="text-xs font-mono uppercase tracking-widest" style={{ color: 'var(--text-secondary)' }}>{t('noModelsConfigured', language)}</div>
            </div>
          )}
        </div>
      </div>

      {/* Exchanges Card */}
      <div className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--panel-border)', background: 'var(--panel-bg)' }}>
        <div className="px-4 py-3 flex items-center gap-2 backdrop-blur-sm" style={{ borderBottom: '1px solid var(--panel-border)', background: 'color-mix(in srgb, var(--panel-bg-solid) 86%, transparent)' }}>
          <Landmark className="w-4 h-4 text-nofx-gold" />
          <h3 className="text-sm font-mono font-semibold tracking-widest uppercase" style={{ color: 'var(--text-primary)' }}>
            {t('exchanges', language)}
          </h3>
        </div>

        <div className="p-4 space-y-3">
          {configuredExchanges.map((exchange) => {
            const inUse = isExchangeInUse(exchange.id)
            const usageInfo = getExchangeUsageInfo(exchange.id)
            return (
              <div
                key={exchange.id}
                className={`group relative flex items-center justify-between p-3 rounded-md transition-all border ${inUse ? 'opacity-80' : 'cursor-pointer'}`}
                style={{ background: 'var(--panel-bg-solid)', borderColor: 'var(--panel-border)' }}
                onClick={() => onExchangeClick(exchange.id)}
              >
                <div className="flex items-center gap-4 min-w-0">
                  <div className="relative">
                    <div className="absolute inset-0 bg-yellow-500/20 rounded-full blur-sm group-hover:bg-yellow-500/30 transition-all"></div>
                    <div className="w-10 h-10 rounded-full flex items-center justify-center relative z-10" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}>
                      {getExchangeIcon(exchange.exchange_type || exchange.id, { width: 20, height: 20 })}
                    </div>
                  </div>

                  <div className="min-w-0">
                    <div className="font-mono text-[15px] font-semibold transition-colors truncate" style={{ color: 'var(--text-primary)' }}>
                      {exchange.exchange_type?.toUpperCase() || getShortName(exchange.name)}
                      <span className="text-[10px] ml-2 border px-1 rounded font-medium" style={{ color: 'var(--text-secondary)', borderColor: 'var(--panel-border)' }}>
                        {exchange.account_name || 'DEFAULT'}
                      </span>
                    </div>
                    <div className="text-xs font-mono font-medium flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
                      {exchange.type?.toUpperCase() || 'CEX'}
                    </div>
                  </div>
                </div>

                <div className="flex flex-col items-end gap-1">
                  {/* Wallet Address Display Logic */}
                  {(() => {
                    const walletAddr = exchange.hyperliquidWalletAddr || exchange.asterUser || exchange.lighterWalletAddr
                    if (exchange.type !== 'dex' || !walletAddr) return null
                    const isVisible = visibleExchangeAddresses.has(exchange.id)
                    const isCopied = copiedId === `exchange-${exchange.id}`

                    return (
                      <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
                        <span className="text-[10px] font-mono font-medium px-1.5 py-0.5 rounded border" style={{ color: 'var(--text-secondary)', background: 'var(--panel-bg)', borderColor: 'var(--panel-border)' }}>
                          {isVisible ? walletAddr : truncateAddress(walletAddr)}
                        </span>
                        <button
                          onClick={(e) => { e.stopPropagation(); onToggleExchangeAddress(exchange.id) }}
                          className="transition-colors"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          {isVisible ? <EyeOff size={10} /> : <Eye size={10} />}
                        </button>
                        <button
                          onClick={(e) => { e.stopPropagation(); onCopyAddress(`exchange-${exchange.id}`, walletAddr) }}
                          className="transition-colors"
                          style={{ color: 'var(--text-secondary)' }}
                          onMouseEnter={(e) => {
                            e.currentTarget.style.color = '#D4A017'
                          }}
                          onMouseLeave={(e) => {
                            e.currentTarget.style.color = 'var(--text-secondary)'
                          }}
                        >
                          {isCopied ? <Check size={10} className="text-green-500" /> : <Copy size={10} />}
                        </button>
                      </div>
                    )
                  })()}

                  {usageInfo.totalCount > 0 ? (
                    <span className={`text-[10px] font-mono font-semibold px-2 py-1 rounded border ${usageInfo.runningCount > 0
                      ? 'bg-green-500/10 border-green-500/30 text-green-400'
                      : 'bg-yellow-500/10 border-yellow-500/30 text-yellow-400'
                      }`}>
                      {usageInfo.runningCount}/{usageInfo.totalCount} ACTIVE
                    </span>
                  ) : (
                    <span className="text-[10px] font-mono font-semibold uppercase tracking-wider" style={{ color: 'var(--text-secondary)' }}>
                      {language === 'zh' ? '就绪' : 'STANDBY'}
                    </span>
                  )}
                </div>
              </div>
            )
          })}
          {configuredExchanges.length === 0 && (
            <div className="text-center py-10 border border-dashed rounded-lg" style={{ borderColor: 'var(--panel-border)', background: 'var(--panel-bg-solid)' }}>
              <Landmark className="w-8 h-8 mx-auto mb-3" style={{ color: 'var(--text-secondary)' }} />
              <div className="text-xs font-mono uppercase tracking-widest" style={{ color: 'var(--text-secondary)' }}>{t('noExchangesConfigured', language)}</div>
            </div>
          )}
        </div>
      </div>

      {/* Proxy Servers Card */}
      <div className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--panel-border)', background: 'var(--panel-bg)' }}>
        <div className="px-4 py-3 flex items-center gap-2 backdrop-blur-sm" style={{ borderBottom: '1px solid var(--panel-border)', background: 'color-mix(in srgb, var(--panel-bg-solid) 86%, transparent)' }}>
          <Server className="w-4 h-4 text-nofx-gold" />
          <h3 className="text-sm font-mono font-semibold tracking-widest uppercase" style={{ color: 'var(--text-primary)' }}>
            {language === 'zh' ? '代理服务器' : 'Proxy Servers'}
          </h3>
        </div>

        <div className="p-4 space-y-3">
          {configuredProxyServers.map((proxy) => (
            <div
              key={proxy.id}
              className="group relative flex items-center justify-between p-3 rounded-md transition-all border cursor-pointer"
              style={{ background: 'var(--panel-bg-solid)', borderColor: 'var(--panel-border)' }}
              onClick={onProxyClick}
            >
              <div className="flex items-center gap-4 min-w-0">
                <div className="relative">
                  <div className="absolute inset-0 bg-emerald-500/20 rounded-full blur-sm group-hover:bg-emerald-500/30 transition-all"></div>
                  <div className="w-10 h-10 rounded-full flex items-center justify-center relative z-10" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}>
                    <Server size={18} style={{ color: 'var(--text-secondary)' }} />
                  </div>
                </div>

                <div className="min-w-0">
                  <div className="font-mono text-[15px] font-semibold transition-colors truncate" style={{ color: 'var(--text-primary)' }}>
                    {proxy.name}
                  </div>
                  <div className="text-xs font-mono font-medium truncate" style={{ color: 'var(--text-secondary)' }}>
                    {language === 'zh' ? '出口 IP：' : 'Exit IP: '}
                    {proxy.last_exit_ip || '-'}
                  </div>
                </div>
              </div>

              <span className={`text-[10px] font-mono font-semibold px-2 py-1 rounded border ${
                proxy.bound_exchange_id
                  ? 'bg-green-500/10 border-green-500/30 text-green-400'
                  : 'bg-yellow-500/10 border-yellow-500/30 text-yellow-400'
              }`}>
                {proxy.bound_exchange_id ? (language === 'zh' ? '已绑定' : 'BOUND') : (language === 'zh' ? '未绑定' : 'UNBOUND')}
              </span>
            </div>
          ))}

          {configuredProxyServers.length === 0 && (
            <div className="text-center py-10 border border-dashed rounded-lg" style={{ borderColor: 'var(--panel-border)', background: 'var(--panel-bg-solid)' }}>
              <Server className="w-8 h-8 mx-auto mb-3" style={{ color: 'var(--text-secondary)' }} />
              <div className="text-xs font-mono uppercase tracking-widest" style={{ color: 'var(--text-secondary)' }}>
                {language === 'zh' ? '尚未配置代理服务器' : 'No proxy servers configured'}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
