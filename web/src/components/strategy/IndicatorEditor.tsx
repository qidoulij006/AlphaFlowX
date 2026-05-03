import { Clock, Activity, TrendingUp, BarChart2, Info, Lock, Zap, Check, AlertCircle, Key } from 'lucide-react'
import type { IndicatorConfig } from '../../types'
import { indicator, ts } from '../../i18n/strategy-translations'

interface IndicatorEditorProps {
  config: IndicatorConfig
  onChange: (config: IndicatorConfig) => void
  disabled?: boolean
  language: string
}

// All available timeframes
const allTimeframes = [
  { value: '1m', label: '1m', category: 'scalp' },
  { value: '3m', label: '3m', category: 'scalp' },
  { value: '5m', label: '5m', category: 'scalp' },
  { value: '15m', label: '15m', category: 'intraday' },
  { value: '30m', label: '30m', category: 'intraday' },
  { value: '1h', label: '1h', category: 'intraday' },
  { value: '2h', label: '2h', category: 'swing' },
  { value: '4h', label: '4h', category: 'swing' },
  { value: '6h', label: '6h', category: 'swing' },
  { value: '8h', label: '8h', category: 'swing' },
  { value: '12h', label: '12h', category: 'swing' },
  { value: '1d', label: '1D', category: 'position' },
  { value: '3d', label: '3D', category: 'position' },
  { value: '1w', label: '1W', category: 'position' },
]

export function IndicatorEditor({
  config,
  onChange,
  disabled,
  language,
}: IndicatorEditorProps) {
  const externalDataKey = config.external_data_key || config.nofxos_api_key || ''

  const updateExternalDataKey = (value: string) => {
    onChange({
      ...config,
      external_data_key: value,
      nofxos_api_key: value,
    })
  }

  // Get currently selected timeframes
  const selectedTimeframes = config.klines.selected_timeframes || [config.klines.primary_timeframe]

  // Toggle timeframe selection
  const toggleTimeframe = (tf: string) => {
    if (disabled) return
    const current = [...selectedTimeframes]
    const index = current.indexOf(tf)

    if (index >= 0) {
      if (current.length > 1) {
        current.splice(index, 1)
        const newPrimary = tf === config.klines.primary_timeframe ? current[0] : config.klines.primary_timeframe
        onChange({
          ...config,
          klines: {
            ...config.klines,
            selected_timeframes: current,
            primary_timeframe: newPrimary,
            enable_multi_timeframe: current.length > 1,
          },
        })
      }
    } else {
      current.push(tf)
      onChange({
        ...config,
        klines: {
          ...config.klines,
          selected_timeframes: current,
          enable_multi_timeframe: current.length > 1,
        },
      })
    }
  }

  // Set primary timeframe
  const setPrimaryTimeframe = (tf: string) => {
    if (disabled) return
    onChange({
      ...config,
      klines: {
        ...config.klines,
        primary_timeframe: tf,
      },
    })
  }

  const categoryColors: Record<string, string> = {
    scalp: '#F6465D',
    intraday: '#F0B90B',
    swing: '#0ECB81',
    position: '#60a5fa',
  }

  // Ensure enable_raw_klines is always true
  const ensureRawKlines = () => {
    if (!config.enable_raw_klines) {
      onChange({ ...config, enable_raw_klines: true })
    }
  }

  // Call on mount if needed
  if (config.enable_raw_klines === undefined || config.enable_raw_klines === false) {
    ensureRawKlines()
  }

  // Legacy external quant data key
  const hasApiKey = !!externalDataKey

  return (
    <div className="space-y-5">
      {/* ============================================ */}
      {/* External Quant Data - Top Configuration    */}
      {/* ============================================ */}
      <div
        className="rounded-lg overflow-hidden relative"
        style={{
          background: 'linear-gradient(135deg, rgba(99, 102, 241, 0.08) 0%, rgba(168, 85, 247, 0.08) 50%, rgba(236, 72, 153, 0.08) 100%)',
          border: '1px solid rgba(139, 92, 246, 0.3)',
        }}
      >
        {/* Decorative gradient line at top */}
        <div
          className="absolute top-0 left-0 right-0 h-[2px]"
          style={{ background: 'linear-gradient(90deg, #6366f1, #a855f7, #ec4899)' }}
        />

        <div className="p-4">
          {/* Header Row */}
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <div
                className="w-8 h-8 rounded-lg flex items-center justify-center"
                style={{ background: 'linear-gradient(135deg, #6366f1, #a855f7)' }}
              >
                <Zap className="w-4 h-4" style={{ color: '#FFFFFF' }} />
              </div>
              <div>
                <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                  {ts(indicator.nofxosTitle, language)}
                </h3>
                <span className="text-[10px]" style={{ color: 'var(--text-secondary)' }}>
                  {ts(indicator.nofxosFeatures, language)}
                </span>
              </div>
            </div>

            {/* Status */}
            <div className="flex items-center gap-2">
              {hasApiKey ? (
                <span className="flex items-center gap-1 text-[10px] px-2 py-1 rounded-full" style={{ background: 'rgba(14, 203, 129, 0.15)', color: '#0ECB81' }}>
                  <Check className="w-3 h-3" />
                  {ts(indicator.connected, language)}
                </span>
              ) : (
                <span className="flex items-center gap-1 text-[10px] px-2 py-1 rounded-full" style={{ background: 'rgba(246, 70, 93, 0.15)', color: '#F6465D' }}>
                  <AlertCircle className="w-3 h-3" />
                  {ts(indicator.notConfigured, language)}
                </span>
              )}
              <span
                className="flex items-center gap-1 text-[10px] px-2 py-1 rounded-full"
                style={{
                  background: 'rgba(246, 70, 93, 0.12)',
                  color: '#F6465D',
                }}
              >
                <Lock className="w-3 h-3" />
                {language === 'zh' ? '已禁用' : 'Disabled'}
              </span>
            </div>
          </div>

          {/* API Key Input */}
          <div className="flex items-center gap-2">
            <div className="flex-1 relative">
              <Key className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4" style={{ color: 'var(--text-secondary)' }} />
              <input
                type="text"
                value={externalDataKey}
                onChange={(e) => !disabled && updateExternalDataKey(e.target.value)}
                disabled={disabled}
                placeholder={ts(indicator.apiKeyPlaceholder, language)}
                className="w-full pl-9 pr-3 py-2 rounded-lg text-sm font-mono"
                style={{
                  background: 'var(--panel-bg)',
                  border: hasApiKey ? '1px solid rgba(14, 203, 129, 0.3)' : '1px solid rgba(139, 92, 246, 0.3)',
                  color: 'var(--text-primary)',
                }}
              />
            </div>
          </div>
          <p className="mt-2 text-[10px]" style={{ color: 'var(--text-secondary)' }}>
            {language === 'zh'
              ? '当前部署已切断外部量化数据调用，相关配置仅保留为兼容旧策略。'
              : 'External quant data calls are disabled in this deployment. These fields remain only for backward compatibility.'}
          </p>

          {/* External Quant Data Sources Grid */}
          <div className="mt-4">
            <div className="text-[10px] font-medium mb-2" style={{ color: 'var(--text-secondary)' }}>
              {ts(indicator.nofxosDataSources, language)}
            </div>
            <div className="grid grid-cols-2 gap-2">
              {/* Quant Data */}
              <div
                className="p-2.5 rounded-lg transition-all cursor-pointer"
                style={{
                  background: config.enable_quant_data ? 'rgba(96, 165, 250, 0.1)' : 'var(--panel-bg)',
                  border: config.enable_quant_data ? '1px solid rgba(96, 165, 250, 0.3)' : '1px solid var(--panel-border)',
                  opacity: disabled ? 0.5 : 1,
                }}
                onClick={() => !disabled && onChange({ ...config, enable_quant_data: !config.enable_quant_data })}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full" style={{ background: '#60a5fa' }} />
                    <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>{ts(indicator.quantData, language)}</span>
                  </div>
                  <input
                    type="checkbox"
                    checked={config.enable_quant_data || false}
                    onChange={(e) => { e.stopPropagation(); !disabled && onChange({ ...config, enable_quant_data: e.target.checked }) }}
                    disabled={disabled}
                    className="w-3.5 h-3.5 rounded accent-blue-500"
                  />
                </div>
                <p className="text-[10px] mt-1" style={{ color: 'var(--text-secondary)' }}>{ts(indicator.quantDataDesc, language)}</p>
                {config.enable_quant_data && (
                  <div className="flex gap-3 mt-2">
                    <label className="flex items-center gap-1.5 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={config.enable_quant_oi !== false}
                        onChange={(e) => { e.stopPropagation(); !disabled && onChange({ ...config, enable_quant_oi: e.target.checked }) }}
                        disabled={disabled}
                        className="w-3 h-3 rounded accent-blue-500"
                      />
                      <span className="text-[10px]" style={{ color: 'var(--text-primary)' }}>OI</span>
                    </label>
                    <label className="flex items-center gap-1.5 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={config.enable_quant_netflow !== false}
                        onChange={(e) => { e.stopPropagation(); !disabled && onChange({ ...config, enable_quant_netflow: e.target.checked }) }}
                        disabled={disabled}
                        className="w-3 h-3 rounded accent-blue-500"
                      />
                      <span className="text-[10px]" style={{ color: 'var(--text-primary)' }}>Netflow</span>
                    </label>
                  </div>
                )}
              </div>

              {/* OI Ranking */}
              <div
                className="p-2.5 rounded-lg transition-all cursor-pointer"
                style={{
                  background: config.enable_oi_ranking ? 'rgba(34, 197, 94, 0.1)' : 'var(--panel-bg)',
                  border: config.enable_oi_ranking ? '1px solid rgba(34, 197, 94, 0.3)' : '1px solid var(--panel-border)',
                  opacity: disabled ? 0.5 : 1,
                }}
                onClick={() => !disabled && onChange({
                  ...config,
                  enable_oi_ranking: !config.enable_oi_ranking,
                  ...(!config.enable_oi_ranking && !config.oi_ranking_duration ? { oi_ranking_duration: '1h' } : {}),
                  ...(!config.enable_oi_ranking && !config.oi_ranking_limit ? { oi_ranking_limit: 10 } : {}),
                })}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full" style={{ background: '#22c55e' }} />
                    <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>{ts(indicator.oiRanking, language)}</span>
                  </div>
                  <input
                    type="checkbox"
                    checked={config.enable_oi_ranking || false}
                    onChange={(e) => { e.stopPropagation(); !disabled && onChange({
                      ...config,
                      enable_oi_ranking: e.target.checked,
                      ...(e.target.checked && !config.oi_ranking_duration ? { oi_ranking_duration: '1h' } : {}),
                      ...(e.target.checked && !config.oi_ranking_limit ? { oi_ranking_limit: 10 } : {}),
                    }) }}
                    disabled={disabled}
                    className="w-3.5 h-3.5 rounded accent-green-500"
                  />
                </div>
                <p className="text-[10px] mt-1" style={{ color: 'var(--text-secondary)' }}>{ts(indicator.oiRankingDesc, language)}</p>
                {config.enable_oi_ranking && (
                  <div className="flex gap-2 mt-2" onClick={(e) => e.stopPropagation()}>
                    <select
                      value={config.oi_ranking_duration || '1h'}
                      onChange={(e) => !disabled && onChange({ ...config, oi_ranking_duration: e.target.value })}
                      disabled={disabled}
                      className="flex-1 px-2 py-1 rounded text-[10px]"
                      style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                    >
                      <option value="1h">1h</option>
                      <option value="4h">4h</option>
                      <option value="24h">24h</option>
                    </select>
                    <select
                      value={config.oi_ranking_limit || 10}
                      onChange={(e) => !disabled && onChange({ ...config, oi_ranking_limit: parseInt(e.target.value) })}
                      disabled={disabled}
                      className="w-14 px-2 py-1 rounded text-[10px]"
                      style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                    >
                      {[5, 10, 15, 20].map(n => <option key={n} value={n}>{n}</option>)}
                    </select>
                  </div>
                )}
              </div>

              {/* NetFlow Ranking */}
              <div
                className="p-2.5 rounded-lg transition-all cursor-pointer"
                style={{
                  background: config.enable_netflow_ranking ? 'rgba(245, 158, 11, 0.1)' : 'rgba(30, 35, 41, 0.5)',
                  border: config.enable_netflow_ranking ? '1px solid rgba(245, 158, 11, 0.3)' : '1px solid rgba(43, 49, 57, 0.5)',
                  opacity: disabled ? 0.5 : 1,
                }}
                onClick={() => !disabled && onChange({
                  ...config,
                  enable_netflow_ranking: !config.enable_netflow_ranking,
                  ...(!config.enable_netflow_ranking && !config.netflow_ranking_duration ? { netflow_ranking_duration: '1h' } : {}),
                  ...(!config.enable_netflow_ranking && !config.netflow_ranking_limit ? { netflow_ranking_limit: 10 } : {}),
                })}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full" style={{ background: '#f59e0b' }} />
                    <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>{ts(indicator.netflowRanking, language)}</span>
                  </div>
                  <input
                    type="checkbox"
                    checked={config.enable_netflow_ranking || false}
                    onChange={(e) => { e.stopPropagation(); !disabled && onChange({
                      ...config,
                      enable_netflow_ranking: e.target.checked,
                      ...(e.target.checked && !config.netflow_ranking_duration ? { netflow_ranking_duration: '1h' } : {}),
                      ...(e.target.checked && !config.netflow_ranking_limit ? { netflow_ranking_limit: 10 } : {}),
                    }) }}
                    disabled={disabled}
                    className="w-3.5 h-3.5 rounded accent-amber-500"
                  />
                </div>
                <p className="text-[10px] mt-1" style={{ color: '#5E6673' }}>{ts(indicator.netflowRankingDesc, language)}</p>
                {config.enable_netflow_ranking && (
                  <div className="flex gap-2 mt-2" onClick={(e) => e.stopPropagation()}>
                    <select
                      value={config.netflow_ranking_duration || '1h'}
                      onChange={(e) => !disabled && onChange({ ...config, netflow_ranking_duration: e.target.value })}
                      disabled={disabled}
                      className="flex-1 px-2 py-1 rounded text-[10px]"
                      style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                    >
                      <option value="1h">1h</option>
                      <option value="4h">4h</option>
                      <option value="24h">24h</option>
                    </select>
                    <select
                      value={config.netflow_ranking_limit || 10}
                      onChange={(e) => !disabled && onChange({ ...config, netflow_ranking_limit: parseInt(e.target.value) })}
                      disabled={disabled}
                      className="w-14 px-2 py-1 rounded text-[10px]"
                      style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                    >
                      {[5, 10, 15, 20].map(n => <option key={n} value={n}>{n}</option>)}
                    </select>
                  </div>
                )}
              </div>

              {/* Price Ranking */}
              <div
                className="p-2.5 rounded-lg transition-all cursor-pointer"
                style={{
                  background: config.enable_price_ranking ? 'rgba(236, 72, 153, 0.1)' : 'rgba(30, 35, 41, 0.5)',
                  border: config.enable_price_ranking ? '1px solid rgba(236, 72, 153, 0.3)' : '1px solid rgba(43, 49, 57, 0.5)',
                  opacity: disabled ? 0.5 : 1,
                }}
                onClick={() => !disabled && onChange({
                  ...config,
                  enable_price_ranking: !config.enable_price_ranking,
                  ...(!config.enable_price_ranking && !config.price_ranking_duration ? { price_ranking_duration: '1h,4h,24h' } : {}),
                  ...(!config.enable_price_ranking && !config.price_ranking_limit ? { price_ranking_limit: 10 } : {}),
                })}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full" style={{ background: '#ec4899' }} />
                    <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>{ts(indicator.priceRanking, language)}</span>
                  </div>
                  <input
                    type="checkbox"
                    checked={config.enable_price_ranking || false}
                    onChange={(e) => { e.stopPropagation(); !disabled && onChange({
                      ...config,
                      enable_price_ranking: e.target.checked,
                      ...(e.target.checked && !config.price_ranking_duration ? { price_ranking_duration: '1h,4h,24h' } : {}),
                      ...(e.target.checked && !config.price_ranking_limit ? { price_ranking_limit: 10 } : {}),
                    }) }}
                    disabled={disabled}
                    className="w-3.5 h-3.5 rounded accent-pink-500"
                  />
                </div>
                <p className="text-[10px] mt-1" style={{ color: '#5E6673' }}>{ts(indicator.priceRankingDesc, language)}</p>
                {config.enable_price_ranking && (
                  <div className="flex gap-2 mt-2" onClick={(e) => e.stopPropagation()}>
                    <select
                      value={config.price_ranking_duration || '1h,4h,24h'}
                      onChange={(e) => !disabled && onChange({ ...config, price_ranking_duration: e.target.value })}
                      disabled={disabled}
                      className="flex-1 px-2 py-1 rounded text-[10px]"
                      style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                    >
                      <option value="1h">1h</option>
                      <option value="4h">4h</option>
                      <option value="24h">24h</option>
                      <option value="1h,4h,24h">{ts(indicator.priceRankingMulti, language)}</option>
                    </select>
                    <select
                      value={config.price_ranking_limit || 10}
                      onChange={(e) => !disabled && onChange({ ...config, price_ranking_limit: parseInt(e.target.value) })}
                      disabled={disabled}
                      className="w-14 px-2 py-1 rounded text-[10px]"
                      style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                    >
                      {[5, 10, 15, 20].map(n => <option key={n} value={n}>{n}</option>)}
                    </select>
                  </div>
                )}
              </div>
            </div>

            <div className="flex items-center gap-2 mt-3 p-2 rounded-lg" style={{ background: 'rgba(246, 70, 93, 0.1)', border: '1px solid rgba(246, 70, 93, 0.2)' }}>
              <AlertCircle className="w-4 h-4 flex-shrink-0" style={{ color: '#F6465D' }} />
              <span className="text-[10px]" style={{ color: '#F6465D' }}>
                {language === 'zh' ? '这些选项当前不会发起任何外部请求。' : 'These options do not make outbound requests in this deployment.'}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* ============================================ */}
      {/* Section 1: Market Data (Required)           */}
      {/* ============================================ */}
      <div className="rounded-lg overflow-hidden" style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}>
        <div className="px-3 py-2 flex items-center gap-2" style={{ background: 'var(--panel-bg)', borderBottom: '1px solid var(--panel-border)' }}>
          <BarChart2 className="w-4 h-4" style={{ color: '#F0B90B' }} />
          <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{ts(indicator.marketData, language)}</span>
          <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>- {ts(indicator.marketDataDesc, language)}</span>
        </div>

        <div className="p-3 space-y-4">
          {/* Raw Klines - Required, Always On */}
          <div className="flex items-center justify-between p-3 rounded-lg" style={{ background: 'rgba(240, 185, 11, 0.08)', border: '1px solid rgba(240, 185, 11, 0.2)' }}>
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: 'rgba(240, 185, 11, 0.15)' }}>
                <TrendingUp className="w-4 h-4" style={{ color: '#F0B90B' }} />
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{ts(indicator.rawKlines, language)}</span>
                  <span className="px-1.5 py-0.5 rounded text-[10px] font-medium flex items-center gap-1" style={{ background: 'rgba(240, 185, 11, 0.2)', color: '#F0B90B' }}>
                    <Lock className="w-2.5 h-2.5" />
                    {ts(indicator.required, language)}
                  </span>
                </div>
                <p className="text-xs mt-0.5" style={{ color: 'var(--text-secondary)' }}>{ts(indicator.rawKlinesDesc, language)}</p>
              </div>
            </div>
            <input
              type="checkbox"
              checked={true}
              disabled={true}
              className="w-5 h-5 rounded accent-yellow-500 cursor-not-allowed"
            />
          </div>

          {/* Timeframe Selection */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-2">
                <Clock className="w-3.5 h-3.5" style={{ color: 'var(--text-secondary)' }} />
                <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>{ts(indicator.timeframes, language)}</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-[10px]" style={{ color: 'var(--text-secondary)' }}>{ts(indicator.klineCount, language)}:</span>
                <input
                  type="number"
                  value={config.klines.primary_count}
                  onChange={(e) =>
                    !disabled &&
                    onChange({
                      ...config,
                      klines: { ...config.klines, primary_count: parseInt(e.target.value) || 30 },
                    })
                  }
                  disabled={disabled}
                  min={10}
                  max={200}
                  className="w-16 px-2 py-1 rounded text-xs text-center"
                  style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                />
              </div>
            </div>
            <p className="text-[10px] mb-2" style={{ color: 'var(--text-secondary)' }}>{ts(indicator.timeframesDesc, language)}</p>

            {/* Timeframe Grid */}
            <div className="space-y-1.5">
              {(['scalp', 'intraday', 'swing', 'position'] as const).map((category) => {
                const categoryTfs = allTimeframes.filter((tf) => tf.category === category)
                return (
                  <div key={category} className="flex items-center gap-2">
                    <span className="text-[10px] w-10 flex-shrink-0" style={{ color: categoryColors[category] }}>
                      {ts(indicator[category], language)}
                    </span>
                    <div className="flex flex-wrap gap-1">
                      {categoryTfs.map((tf) => {
                        const isSelected = selectedTimeframes.includes(tf.value)
                        const isPrimary = config.klines.primary_timeframe === tf.value
                        return (
                          <button
                            key={tf.value}
                            onClick={() => toggleTimeframe(tf.value)}
                            onDoubleClick={() => setPrimaryTimeframe(tf.value)}
                            disabled={disabled}
                            className={`px-2 py-1 rounded text-xs font-medium transition-all ${
                              isSelected ? '' : 'opacity-40 hover:opacity-70'
                            }`}
                            style={{
                              background: isSelected ? `${categoryColors[category]}15` : 'transparent',
                              border: `1px solid ${isSelected ? categoryColors[category] : 'var(--panel-border)'}`,
                              color: isSelected ? categoryColors[category] : 'var(--text-secondary)',
                              boxShadow: isPrimary ? `0 0 0 2px ${categoryColors[category]}` : undefined,
                            }}
                            title={isPrimary ? `${tf.label} (Primary)` : tf.label}
                          >
                            {tf.label}
                            {isPrimary && <span className="ml-0.5 text-[8px]">★</span>}
                          </button>
                        )
                      })}
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      </div>

      {/* ============================================ */}
      {/* Section 2: Technical Indicators (Optional)  */}
      {/* ============================================ */}
      <div className="rounded-lg overflow-hidden" style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}>
        <div className="px-3 py-2 flex items-center gap-2" style={{ background: 'var(--panel-bg)', borderBottom: '1px solid var(--panel-border)' }}>
          <Activity className="w-4 h-4" style={{ color: '#0ECB81' }} />
          <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{ts(indicator.technicalIndicators, language)}</span>
          <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>- {ts(indicator.technicalIndicatorsDesc, language)}</span>
        </div>

        <div className="p-3">
          {/* Tip */}
          <div className="flex items-start gap-2 mb-3 p-2 rounded" style={{ background: 'rgba(14, 203, 129, 0.05)' }}>
            <Info className="w-3.5 h-3.5 mt-0.5 flex-shrink-0" style={{ color: '#0ECB81' }} />
            <p className="text-[10px]" style={{ color: 'var(--text-secondary)' }}>{ts(indicator.aiCanCalculate, language)}</p>
          </div>

          {/* Indicator Grid */}
          <div className="grid grid-cols-2 gap-2">
            {[
              { key: 'enable_ema', label: 'ema', desc: 'emaDesc', color: '#F0B90B', periodKey: 'ema_periods', defaultPeriods: '20,50' },
              { key: 'enable_macd', label: 'macd', desc: 'macdDesc', color: '#a855f7' },
              { key: 'enable_rsi', label: 'rsi', desc: 'rsiDesc', color: '#F6465D', periodKey: 'rsi_periods', defaultPeriods: '7,14' },
              { key: 'enable_atr', label: 'atr', desc: 'atrDesc', color: '#60a5fa', periodKey: 'atr_periods', defaultPeriods: '14' },
              { key: 'enable_boll', label: 'boll', desc: 'bollDesc', color: '#ec4899', periodKey: 'boll_periods', defaultPeriods: '20' },
            ].map(({ key, label, desc, color, periodKey, defaultPeriods }) => (
              <div
                key={key}
                className="p-2.5 rounded-lg transition-all"
                style={{
                  background: config[key as keyof IndicatorConfig] ? `${color}08` : 'transparent',
                  border: `1px solid ${config[key as keyof IndicatorConfig] ? `${color}30` : 'var(--panel-border)'}`,
                }}
              >
                <div className="flex items-center justify-between mb-1">
                  <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full" style={{ background: color }} />
                    <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>{ts(indicator[label as keyof typeof indicator], language)}</span>
                  </div>
                  <input
                    type="checkbox"
                    checked={config[key as keyof IndicatorConfig] as boolean || false}
                    onChange={(e) => !disabled && onChange({ ...config, [key]: e.target.checked })}
                    disabled={disabled}
                    className="w-4 h-4 rounded accent-yellow-500"
                  />
                </div>
                <p className="text-[10px] mb-1.5" style={{ color: '#5E6673' }}>{ts(indicator[desc as keyof typeof indicator], language)}</p>
                {periodKey && config[key as keyof IndicatorConfig] && (
                  <input
                    type="text"
                    value={(config[periodKey as keyof IndicatorConfig] as number[])?.join(',') || defaultPeriods}
                    onChange={(e) => {
                      if (disabled) return
                      const periods = e.target.value
                        .split(',')
                        .map((s) => parseInt(s.trim()))
                        .filter((n) => !isNaN(n) && n > 0)
                      onChange({ ...config, [periodKey]: periods })
                    }}
                    disabled={disabled}
                    placeholder={defaultPeriods}
                    className="w-full px-2 py-1 rounded text-[10px] text-center"
                    style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                  />
                )}
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* ============================================ */}
      {/* Section 3: Market Sentiment                 */}
      {/* ============================================ */}
      <div className="rounded-lg overflow-hidden" style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}>
        <div className="px-3 py-2 flex items-center gap-2" style={{ background: 'var(--panel-bg)', borderBottom: '1px solid var(--panel-border)' }}>
          <TrendingUp className="w-4 h-4" style={{ color: '#22c55e' }} />
          <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{ts(indicator.marketSentiment, language)}</span>
          <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>- {ts(indicator.marketSentimentDesc, language)}</span>
        </div>

        <div className="p-3">
          <div className="grid grid-cols-3 gap-2">
            {[
              { key: 'enable_volume', label: 'volume', desc: 'volumeDesc', color: '#c084fc' },
              { key: 'enable_oi', label: 'oi', desc: 'oiDesc', color: '#34d399' },
              { key: 'enable_funding_rate', label: 'fundingRate', desc: 'fundingRateDesc', color: '#fbbf24' },
            ].map(({ key, label, desc, color }) => (
              <div
                key={key}
                className="p-2.5 rounded-lg transition-all"
                style={{
                  background: config[key as keyof IndicatorConfig] ? `${color}08` : 'transparent',
                  border: `1px solid ${config[key as keyof IndicatorConfig] ? `${color}30` : 'var(--panel-border)'}`,
                }}
              >
                <div className="flex items-center justify-between mb-1">
                  <div className="flex items-center gap-2">
                    <div className="w-2 h-2 rounded-full" style={{ background: color }} />
                    <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>{ts(indicator[label as keyof typeof indicator], language)}</span>
                  </div>
                  <input
                    type="checkbox"
                    checked={config[key as keyof IndicatorConfig] as boolean || false}
                    onChange={(e) => !disabled && onChange({ ...config, [key]: e.target.checked })}
                    disabled={disabled}
                    className="w-4 h-4 rounded accent-yellow-500"
                  />
                </div>
                <p className="text-[10px]" style={{ color: '#5E6673' }}>{ts(indicator[desc as keyof typeof indicator], language)}</p>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
