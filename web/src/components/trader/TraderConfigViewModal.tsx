import type { TraderConfigData } from '../../types'
import { t } from '../../i18n/translations'
import { useLanguage } from '../../contexts/LanguageContext'
import { PunkAvatar, getTraderAvatar } from '../common/PunkAvatar'

// Extract the name part after the last underscore
function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}

interface TraderConfigViewModalProps {
  isOpen: boolean
  onClose: () => void
  traderData?: TraderConfigData | null
}

export function TraderConfigViewModal({
  isOpen,
  onClose,
  traderData,
}: TraderConfigViewModalProps) {
  const { language } = useLanguage()
  if (!isOpen || !traderData) return null

  const InfoRow = ({
    label,
    value,
  }: {
    label: string
    value: string | number | boolean
  }) => (
    <div className="flex justify-between items-start py-2 last:border-b-0" style={{ borderBottom: '1px solid var(--panel-border)' }}>
      <span className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{label}</span>
      <span className="text-sm font-mono text-right" style={{ color: 'var(--text-primary)' }}>
        {typeof value === 'boolean' ? (value ? t('traderConfigView.yes', language) : t('traderConfigView.no', language)) : value}
      </span>
    </div>
  )

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div
        className="rounded-xl shadow-2xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto"
        style={{ background: 'linear-gradient(180deg, var(--panel-bg-solid) 0%, var(--panel-bg) 100%)', border: '1px solid var(--panel-border)' }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6" style={{ borderBottom: '1px solid var(--panel-border)', background: 'linear-gradient(to right, var(--panel-bg-solid), color-mix(in srgb, var(--panel-bg) 88%, black))' }}>
          <div className="flex items-center gap-3">
            <PunkAvatar
              seed={getTraderAvatar(traderData.trader_id || '', traderData.trader_name)}
              size={48}
              className="rounded-lg"
            />
            <div>
              <h2 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>{t('traderConfigView.traderConfig', language)}</h2>
              <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
                {t('traderConfigView.configInfo', language, { name: traderData.trader_name })}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {/* Running Status */}
            <div
              className="px-3 py-1 rounded-full text-xs font-bold flex items-center gap-1"
              style={
                traderData.is_running
                  ? { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
                  : { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
              }
            >
              <span>{traderData.is_running ? '●' : '○'}</span>
              {traderData.is_running ? t('traderConfigView.running', language) : t('traderConfigView.stopped', language)}
            </div>
            <button
              onClick={onClose}
              className="w-8 h-8 rounded-lg transition-colors flex items-center justify-center"
              style={{ color: 'var(--text-secondary)' }}
            >
              ✕
            </button>
          </div>
        </div>

        {/* Content */}
        <div className="p-6 space-y-6">
          {/* Basic Info */}
          <div className="rounded-lg p-5" style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}>
            <h3 className="text-lg font-semibold mb-4 flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
              {'🤖 ' + t('traderConfigView.basicInfo', language)}
            </h3>
            <div className="space-y-3">
              <InfoRow
                label={t('traderConfigView.traderName', language)}
                value={traderData.trader_name}
              />
              <InfoRow
                label={t('traderConfigView.aiModel', language)}
                value={getShortName(traderData.ai_model).toUpperCase()}
              />
              <InfoRow
                label={t('traderConfigView.exchange', language)}
                value={getShortName(traderData.exchange_id).toUpperCase()}
              />
              <InfoRow
                label={t('traderConfigView.initialBalance', language)}
                value={`$${traderData.initial_balance.toLocaleString()}`}
              />
              <InfoRow
                label={t('traderConfigView.marginMode', language)}
                value={traderData.is_cross_margin ? t('traderConfigView.crossMargin', language) : t('traderConfigView.isolatedMargin', language)}
              />
              <InfoRow
                label={t('traderConfigView.scanIntervalLabel', language)}
                value={t('traderConfigView.scanInterval', language, { minutes: traderData.scan_interval_minutes || 3 })}
              />
            </div>
          </div>

          {/* Strategy Info - only show if strategy is bound */}
          {traderData.strategy_id && (
            <div className="rounded-lg p-5" style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}>
              <h3 className="text-lg font-semibold mb-4 flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
                {'📋 ' + t('traderConfigView.strategyUsed', language)}
              </h3>
              <div className="space-y-3">
                <InfoRow
                  label={t('traderConfigView.strategyName', language)}
                  value={traderData.strategy_name || traderData.strategy_id}
                />
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex justify-end p-6" style={{ borderTop: '1px solid var(--panel-border)', background: 'linear-gradient(to right, var(--panel-bg-solid), color-mix(in srgb, var(--panel-bg) 88%, black))' }}>
          <button
            onClick={onClose}
            className="px-6 py-3 rounded-lg transition-all duration-200"
            style={{ background: 'var(--panel-bg-solid)', color: 'var(--text-primary)', border: '1px solid var(--panel-border)' }}
          >
            {t('traderConfigView.close', language)}
          </button>
        </div>
      </div>
    </div>
  )
}
