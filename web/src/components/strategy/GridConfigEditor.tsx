import { useEffect, useState } from 'react'
import { Grid, DollarSign, TrendingUp, Shield, Compass } from 'lucide-react'
import type { GridStrategyConfig } from '../../types'
import { gridConfig, ts } from '../../i18n/strategy-translations'

interface GridConfigEditorProps {
  config: GridStrategyConfig
  onChange: (config: GridStrategyConfig) => void
  disabled?: boolean
  language: string
}

// Default grid configuration
export const defaultGridConfig: GridStrategyConfig = {
  symbol: 'BTCUSDT',
  grid_count: 10,
  total_investment: 1000,
  leverage: 5,
  upper_price: 0,
  lower_price: 0,
  use_atr_bounds: true,
  atr_multiplier: 2.0,
  distribution: 'gaussian',
  max_drawdown_pct: 15,
  stop_loss_pct: 5,
  daily_loss_limit_pct: 10,
  use_maker_only: true,
  enable_direction_adjust: false,
  direction_bias_ratio: 0.7,
}

export function GridConfigEditor({
  config,
  onChange,
  disabled,
  language,
}: GridConfigEditorProps) {
  const presetSymbols = ['BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'BNBUSDT', 'XRPUSDT', 'DOGEUSDT']
  const isCustomSymbol = !presetSymbols.includes((config.symbol || '').toUpperCase())
  const [symbolMode, setSymbolMode] = useState<'preset' | 'custom'>(isCustomSymbol ? 'custom' : 'preset')
  const [customSymbol, setCustomSymbol] = useState(config.symbol || '')
  const [customSymbolError, setCustomSymbolError] = useState('')

  const formatGridSymbolForDisplay = (raw: string): string => {
    const normalized = raw.trim().toUpperCase().replace(/\s+/g, '')
    if (/^[A-Z0-9]{2,20}USDT$/.test(normalized)) {
      return `${normalized.slice(0, -4)}/USDT`
    }
    return normalized
  }

  useEffect(() => {
    const nextIsCustom = !presetSymbols.includes((config.symbol || '').toUpperCase())
    setSymbolMode(nextIsCustom ? 'custom' : 'preset')
    if (nextIsCustom) {
      setCustomSymbol(formatGridSymbolForDisplay(config.symbol || ''))
    }
  }, [config.symbol])

  const updateField = <K extends keyof GridStrategyConfig>(
    key: K,
    value: GridStrategyConfig[K]
  ) => {
    if (!disabled) {
      onChange({ ...config, [key]: value })
    }
  }

  const normalizeGridSymbol = (raw: string): string => raw.trim().toUpperCase().replace(/\s+/g, '')

  const validateGridSymbol = (raw: string): string => {
    const symbol = normalizeGridSymbol(raw)
    if (!symbol) {
      return language === 'zh' ? '请输入交易对' : 'Please enter a trading pair'
    }
    if (/^[A-Z0-9]{2,20}USDT$/.test(symbol)) {
      return ''
    }
    if (/^[A-Z0-9]{2,20}\/USDT$/.test(symbol)) {
      return ''
    }
    return language === 'zh'
      ? '格式必须为 BTC/USDT，或 XYZ/USDT 这类格式'
      : 'Format must be like BTC/USDT, or XYZ/USDT'
  }

  const applyCustomSymbol = () => {
    const normalized = normalizeGridSymbol(customSymbol)
    const error = validateGridSymbol(normalized)
    setCustomSymbolError(error)
    if (error || disabled) return
    updateField('symbol', normalized.replace('/', ''))
  }

  const inputStyle = {
    background: 'var(--panel-bg)',
    border: '1px solid var(--panel-border)',
    color: 'var(--text-primary)',
  }

  const sectionStyle = {
    background: 'var(--panel-bg-solid)',
    border: '1px solid var(--panel-border)',
  }

  return (
    <div className="space-y-6">
      {/* Trading Setup */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <DollarSign className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: 'var(--text-primary)' }}>
            {ts(gridConfig.tradingPair, language)}
          </h3>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {/* Symbol */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
              {ts(gridConfig.symbol, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
              {ts(gridConfig.symbolDesc, language)}
            </p>
            <select
              value={symbolMode === 'custom' ? '__custom__' : config.symbol}
              onChange={(e) => {
                const value = e.target.value
                if (value === '__custom__') {
                  setSymbolMode('custom')
                  setCustomSymbol(formatGridSymbolForDisplay(config.symbol || ''))
                  setCustomSymbolError(validateGridSymbol(config.symbol || ''))
                  return
                }
                setSymbolMode('preset')
                setCustomSymbolError('')
                updateField('symbol', value)
              }}
              disabled={disabled}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            >
              <option value="BTCUSDT">BTC/USDT</option>
              <option value="ETHUSDT">ETH/USDT</option>
              <option value="SOLUSDT">SOL/USDT</option>
              <option value="BNBUSDT">BNB/USDT</option>
              <option value="XRPUSDT">XRP/USDT</option>
              <option value="DOGEUSDT">DOGE/USDT</option>
              <option value="__custom__">{language === 'zh' ? '自定义交易对' : 'Custom Symbol'}</option>
            </select>
            {symbolMode === 'custom' && (
              <div className="mt-3 space-y-2">
                <input
                  type="text"
                  value={customSymbol}
                  onChange={(e) => {
                    setCustomSymbol(e.target.value)
                    if (customSymbolError) {
                      setCustomSymbolError(validateGridSymbol(e.target.value))
                    }
                  }}
                  onBlur={applyCustomSymbol}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      applyCustomSymbol()
                    }
                  }}
                  disabled={disabled}
                  placeholder={language === 'zh' ? '例如 BTC/USDT 或 XYZ/USDT' : 'e.g. BTC/USDT or XYZ/USDT'}
                  className="w-full px-3 py-2 rounded font-mono"
                  style={inputStyle}
                />
                <div className="text-xs leading-5" style={{ color: customSymbolError ? '#F6465D' : 'var(--text-secondary)' }}>
                  {customSymbolError || (language === 'zh'
                    ? '仅允许标准永续格式：BTC/USDT；若为 xyz 资产，请填写 XYZ/USDT 这类格式。'
                    : 'Only standard perpetual format is allowed: BTC/USDT. For xyz assets, use a format like XYZ/USDT.')}
                </div>
              </div>
            )}
          </div>

          {/* Investment */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
              {ts(gridConfig.totalInvestment, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
              {ts(gridConfig.totalInvestmentDesc, language)}
            </p>
            <input
              type="number"
              value={config.total_investment}
              onChange={(e) => updateField('total_investment', parseFloat(e.target.value) || 1000)}
              disabled={disabled}
              min={100}
              step={100}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
            <div className="mt-2 text-xs leading-5" style={{ color: 'var(--text-secondary)' }}>
              {language === 'zh'
                ? '实际运行中，每笔网格挂单会按实时账户权益动态调整；这里主要决定初始启动金额和余额异常时的回退值。'
                : 'During live trading, each grid order scales from real-time account equity. This field mainly defines the startup base and the fallback value when balance data is unavailable.'}
            </div>
          </div>

          {/* Leverage */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
              {ts(gridConfig.leverage, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
              {ts(gridConfig.leverageDesc, language)}
            </p>
            <input
              type="number"
              value={config.leverage}
              onChange={(e) => updateField('leverage', parseInt(e.target.value) || 5)}
              disabled={disabled}
              min={1}
              max={5}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>
        </div>
      </div>

      {/* Grid Parameters */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Grid className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: 'var(--text-primary)' }}>
            {ts(gridConfig.gridParameters, language)}
          </h3>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Grid Count */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
              {ts(gridConfig.gridCount, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
              {ts(gridConfig.gridCountDesc, language)}
            </p>
            <input
              type="number"
              value={config.grid_count}
              onChange={(e) => updateField('grid_count', parseInt(e.target.value) || 10)}
              disabled={disabled}
              min={5}
              max={50}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>

          {/* Distribution */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
              {ts(gridConfig.distribution, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
              {ts(gridConfig.distributionDesc, language)}
            </p>
            <select
              value={config.distribution}
              onChange={(e) => updateField('distribution', e.target.value as 'uniform' | 'gaussian' | 'pyramid')}
              disabled={disabled}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            >
              <option value="uniform">{ts(gridConfig.uniform, language)}</option>
              <option value="gaussian">{ts(gridConfig.gaussian, language)}</option>
              <option value="pyramid">{ts(gridConfig.pyramid, language)}</option>
            </select>
          </div>
        </div>
      </div>

      {/* Price Bounds */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <TrendingUp className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: 'var(--text-primary)' }}>
            {ts(gridConfig.priceBounds, language)}
          </h3>
        </div>

        {/* ATR Toggle */}
        <div className="p-4 rounded-lg mb-4" style={sectionStyle}>
          <div className="flex items-center justify-between">
            <div>
              <label className="block text-sm" style={{ color: 'var(--text-primary)' }}>
                {ts(gridConfig.useAtrBounds, language)}
              </label>
              <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                {ts(gridConfig.useAtrBoundsDesc, language)}
              </p>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={config.use_atr_bounds}
                onChange={(e) => updateField('use_atr_bounds', e.target.checked)}
                disabled={disabled}
                className="sr-only peer"
              />
              <div className="w-11 h-6 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-[#F0B90B]" style={{ background: 'var(--panel-border)' }}></div>
            </label>
          </div>
        </div>

        {config.use_atr_bounds ? (
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
              {ts(gridConfig.atrMultiplier, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
              {ts(gridConfig.atrMultiplierDesc, language)}
            </p>
            <input
              type="number"
              value={config.atr_multiplier}
              onChange={(e) => updateField('atr_multiplier', parseFloat(e.target.value) || 2.0)}
              disabled={disabled}
              min={1}
              max={5}
              step={0.5}
              className="w-32 px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
                {ts(gridConfig.upperPrice, language)}
              </label>
              <p className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
                {ts(gridConfig.upperPriceDesc, language)}
              </p>
              <input
                type="number"
                value={config.upper_price}
                onChange={(e) => updateField('upper_price', parseFloat(e.target.value) || 0)}
                disabled={disabled}
                min={0}
                step={0.01}
                className="w-full px-3 py-2 rounded"
                style={inputStyle}
              />
            </div>
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
                {ts(gridConfig.lowerPrice, language)}
              </label>
              <p className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
                {ts(gridConfig.lowerPriceDesc, language)}
              </p>
              <input
                type="number"
                value={config.lower_price}
                onChange={(e) => updateField('lower_price', parseFloat(e.target.value) || 0)}
                disabled={disabled}
                min={0}
                step={0.01}
                className="w-full px-3 py-2 rounded"
                style={inputStyle}
              />
            </div>
          </div>
        )}
      </div>

      {/* Risk Control */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: 'var(--text-primary)' }}>
            {ts(gridConfig.riskControl, language)}
          </h3>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
              {ts(gridConfig.maxDrawdown, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
              {ts(gridConfig.maxDrawdownDesc, language)}
            </p>
            <input
              type="number"
              value={config.max_drawdown_pct}
              onChange={(e) => updateField('max_drawdown_pct', parseFloat(e.target.value) || 15)}
              disabled={disabled}
              min={5}
              max={50}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>

          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
              {ts(gridConfig.stopLoss, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
              {ts(gridConfig.stopLossDesc, language)}
            </p>
            <input
              type="number"
              value={config.stop_loss_pct}
              onChange={(e) => updateField('stop_loss_pct', parseFloat(e.target.value) || 5)}
              disabled={disabled}
              min={1}
              max={20}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>

          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
              {ts(gridConfig.dailyLossLimit, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
              {ts(gridConfig.dailyLossLimitDesc, language)}
            </p>
            <input
              type="number"
              value={config.daily_loss_limit_pct}
              onChange={(e) => updateField('daily_loss_limit_pct', parseFloat(e.target.value) || 10)}
              disabled={disabled}
              min={1}
              max={30}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>
        </div>

        {/* Maker Only Toggle */}
        <div className="p-4 rounded-lg" style={sectionStyle}>
          <div className="flex items-center justify-between">
            <div>
            <label className="block text-sm" style={{ color: 'var(--text-primary)' }}>
                {ts(gridConfig.useMakerOnly, language)}
              </label>
              <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                {ts(gridConfig.useMakerOnlyDesc, language)}
              </p>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={config.use_maker_only}
                onChange={(e) => updateField('use_maker_only', e.target.checked)}
                disabled={disabled}
                className="sr-only peer"
              />
              <div className="w-11 h-6 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-[#F0B90B]" style={{ background: 'var(--panel-border)' }}></div>
            </label>
          </div>
        </div>
      </div>

      {/* Direction Auto-Adjust */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Compass className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: 'var(--text-primary)' }}>
            {ts(gridConfig.directionAdjust, language)}
          </h3>
        </div>

        {/* Enable Toggle */}
        <div className="p-4 rounded-lg mb-4" style={sectionStyle}>
          <div className="flex items-center justify-between">
            <div>
              <label className="block text-sm" style={{ color: 'var(--text-primary)' }}>
                {ts(gridConfig.enableDirectionAdjust, language)}
              </label>
              <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                {ts(gridConfig.enableDirectionAdjustDesc, language)}
              </p>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={config.enable_direction_adjust ?? false}
                onChange={(e) => updateField('enable_direction_adjust', e.target.checked)}
                disabled={disabled}
                className="sr-only peer"
              />
              <div className="w-11 h-6 bg-gray-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-[#F0B90B]"></div>
            </label>
          </div>
        </div>

        {config.enable_direction_adjust && (
          <>
            {/* Direction Modes Explanation */}
            <div className="p-4 rounded-lg mb-4" style={{ background: 'var(--panel-bg)', border: '1px solid #F0B90B33' }}>
              <p className="text-xs font-medium mb-2" style={{ color: '#F0B90B' }}>
                📊 {ts(gridConfig.directionModes, language)}
              </p>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-2 text-xs" style={{ color: 'var(--text-secondary)' }}>
                <div>• {ts(gridConfig.modeNeutral, language)}</div>
                <div>• <span style={{ color: '#0ECB81' }}>{ts(gridConfig.modeLongBias, language)}</span></div>
                <div>• <span style={{ color: '#0ECB81' }}>{ts(gridConfig.modeLong, language)}</span></div>
                <div>• <span style={{ color: '#F6465D' }}>{ts(gridConfig.modeShortBias, language)}</span></div>
                <div>• <span style={{ color: '#F6465D' }}>{ts(gridConfig.modeShort, language)}</span></div>
              </div>
              <p className="text-xs mt-3 pt-2" style={{ color: 'var(--text-secondary)', borderTop: '1px solid var(--panel-border)' }}>
                💡 {ts(gridConfig.directionExplain, language)}
              </p>
            </div>

            {/* Bias Strength */}
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-1" style={{ color: 'var(--text-primary)' }}>
                {ts(gridConfig.directionBiasRatio, language)} (X)
              </label>
              <p className="text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>
                {ts(gridConfig.directionBiasRatioDesc, language)}
              </p>
              <p className="text-xs mb-3" style={{ color: '#F0B90B' }}>
                {ts(gridConfig.directionBiasExplain, language)}
              </p>
              <div className="flex items-center gap-3">
                <input
                  type="range"
                  value={(config.direction_bias_ratio ?? 0.7) * 100}
                  onChange={(e) => updateField('direction_bias_ratio', parseInt(e.target.value) / 100)}
                  disabled={disabled}
                  min={55}
                  max={90}
                  step={5}
                  className="flex-1 h-2 rounded-lg appearance-none cursor-pointer"
                  style={{ background: 'var(--panel-border)' }}
                />
                <span className="text-sm font-mono w-20 text-right" style={{ color: '#F0B90B' }}>
                  X = {Math.round((config.direction_bias_ratio ?? 0.7) * 100)}%
                </span>
              </div>
              <div className="mt-2 grid grid-cols-2 gap-2 text-xs">
                <div className="p-2 rounded" style={{ background: '#0ECB8115', border: '1px solid #0ECB8130' }}>
                  <span style={{ color: '#0ECB81' }}>Long Bias: </span>
                  <span style={{ color: 'var(--text-primary)' }}>{Math.round((config.direction_bias_ratio ?? 0.7) * 100)}% {ts(gridConfig.buy, language)} + {Math.round((1 - (config.direction_bias_ratio ?? 0.7)) * 100)}% {ts(gridConfig.sell, language)}</span>
                </div>
                <div className="p-2 rounded" style={{ background: '#F6465D15', border: '1px solid #F6465D30' }}>
                  <span style={{ color: '#F6465D' }}>Short Bias: </span>
                  <span style={{ color: 'var(--text-primary)' }}>{Math.round((1 - (config.direction_bias_ratio ?? 0.7)) * 100)}% {ts(gridConfig.buy, language)} + {Math.round((config.direction_bias_ratio ?? 0.7) * 100)}% {ts(gridConfig.sell, language)}</span>
                </div>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
