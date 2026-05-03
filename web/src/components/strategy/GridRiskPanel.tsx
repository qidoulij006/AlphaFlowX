import { useState, useEffect, useCallback } from 'react'
import { Shield, TrendingUp, AlertTriangle, Activity, Box, ChevronDown, ChevronUp } from 'lucide-react'
import type { GridRiskInfo } from '../../types'
import { gridRisk, ts } from '../../i18n/strategy-translations'

interface GridRiskPanelProps {
  traderId: string
  language?: string
  refreshInterval?: number // ms, default 5000
  embedded?: boolean
  shareToken?: string
}

export function GridRiskPanel({
  traderId,
  language = 'en',
  refreshInterval = 5000,
  embedded = false,
  shareToken,
}: GridRiskPanelProps) {
  const [riskInfo, setRiskInfo] = useState<GridRiskInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState(true)
  const [ordersExpanded, setOrdersExpanded] = useState(true)
  const [riskExpanded, setRiskExpanded] = useState(true)

  const fetchRiskInfo = useCallback(async () => {
    try {
      const headers: Record<string, string> = {}
      const token = localStorage.getItem('auth_token')
      if (!shareToken && token) {
        headers.Authorization = `Bearer ${token}`
      }

      const response = await fetch(
        shareToken
          ? `/api/shared/grid-risk?token=${encodeURIComponent(shareToken)}`
          : `/api/traders/${traderId}/grid-risk`,
        { headers }
      )

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }

      const data = await response.json()
      setRiskInfo(data)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }, [traderId])

  useEffect(() => {
    fetchRiskInfo()
    const interval = setInterval(fetchRiskInfo, refreshInterval)
    return () => clearInterval(interval)
  }, [fetchRiskInfo, refreshInterval])

  const getRegimeColor = (regime: string) => {
    switch (regime) {
      case 'narrow': return '#0ECB81'
      case 'standard': return '#F0B90B'
      case 'wide': return '#F7931A'
      case 'volatile': return '#F6465D'
      case 'trending': return '#8B5CF6'
      default: return 'var(--text-secondary)'
    }
  }

  const getBreakoutColor = (level: string) => {
    switch (level) {
      case 'none': return '#0ECB81'
      case 'short': return '#F0B90B'
      case 'mid': return '#F7931A'
      case 'long': return '#F6465D'
      default: return 'var(--text-secondary)'
    }
  }

  const getPositionColor = (percent: number) => {
    if (percent < 50) return '#0ECB81'
    if (percent < 80) return '#F0B90B'
    return '#F6465D'
  }

  const formatPrice = (price: number | undefined | null) => {
    if (typeof price !== 'number' || Number.isNaN(price) || price === 0) return '-'
    if (price >= 1000) return price.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
    if (price >= 1) return price.toFixed(4)
    return price.toFixed(6)
  }

  const formatUSD = (value: number | undefined) => {
    if (typeof value !== 'number' || Number.isNaN(value)) return '$0'

    const absValue = Math.abs(value)
    if (absValue === 0) return '$0'
    if (absValue < 0.01) return `$${value.toFixed(4)}`
    if (absValue < 1) return `$${value.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 4 })}`
    if (absValue < 1000) return `$${value.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
    return `$${value.toLocaleString('en-US', { minimumFractionDigits: 0, maximumFractionDigits: 0 })}`
  }

  const formatPct = (value: number | undefined | null, digits = 2) => {
    if (typeof value !== 'number' || Number.isNaN(value)) return '-'
    return `${value.toFixed(digits)}%`
  }

  const formatOrderPrice = (price: number | undefined | null) => {
    if (typeof price !== 'number' || Number.isNaN(price) || price === 0) return '-'
    if (price >= 1000) return price.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
    if (price >= 1) return price.toFixed(4)
    return price.toFixed(6)
  }

  const bucketLabel = (bucket: string) => {
    switch (bucket) {
      case 'long_entry': return ts(gridRisk.longEntryOrders, language)
      case 'short_entry': return ts(gridRisk.shortEntryOrders, language)
      case 'long_exit': return ts(gridRisk.longExitOrders, language)
      case 'short_exit': return ts(gridRisk.shortExitOrders, language)
      default: return bucket
    }
  }

  const bucketColor = (bucket: string) => {
    switch (bucket) {
      case 'long_entry': return '#0ECB81'
      case 'short_entry': return '#F6465D'
      case 'long_exit': return '#22C55E'
      case 'short_exit': return '#FB7185'
      default: return '#F0B90B'
    }
  }

  const isExitBucket = (bucket: string) => bucket === 'long_exit' || bucket === 'short_exit'

  const formatLevelLabel = (levelIndex: number) => {
    if (levelIndex < 0) return '-'
    return `L${levelIndex + 1}`
  }

  const formatLevelLabels = (levelIndexes: number[]) => {
    const uniqueLevelIndexes = Array.from(new Set(levelIndexes.filter((levelIndex) => levelIndex >= 0))).sort((a, b) => a - b)
    if (uniqueLevelIndexes.length === 0) return '-'
    return uniqueLevelIndexes.map((levelIndex) => formatLevelLabel(levelIndex)).join('、')
  }

  const formatSourceLevelLabels = (levelIndexes: number[]) => {
    const uniqueLevelIndexes = Array.from(new Set(levelIndexes.filter((levelIndex) => levelIndex >= 0))).sort((a, b) => a - b)
    if (uniqueLevelIndexes.length === 0) {
      return language === 'zh' ? '未知开仓层' : 'Unknown entry level'
    }
    return uniqueLevelIndexes.map((levelIndex) => formatLevelLabel(levelIndex)).join('、')
  }

  const groupedOrders = (riskInfo?.grid_orders || []).reduce<Record<string, Array<{
    price: number
    priceLabel: string
    quantity: number
    notional: number
    orderCount: number
    levelIndexes: number[]
    linkedLevelIndexes: number[]
    sourceEntryPrices: number[]
  }>>>((acc, order) => {
    if (!acc[order.bucket]) acc[order.bucket] = []

    const priceLabel = formatOrderPrice(order.price)
    const existingGroup = acc[order.bucket].find((group) => group.priceLabel === priceLabel)
    if (existingGroup) {
      existingGroup.quantity += order.quantity
      existingGroup.notional += order.notional
      existingGroup.orderCount += 1
      if (order.level_index >= 0) existingGroup.levelIndexes.push(order.level_index)
      if (order.linked_level_index >= 0) existingGroup.linkedLevelIndexes.push(order.linked_level_index)
      if (order.source_entry_price > 0) existingGroup.sourceEntryPrices.push(order.source_entry_price)
      return acc
    }

    acc[order.bucket].push({
      price: order.price,
      priceLabel,
      quantity: order.quantity,
      notional: order.notional,
      orderCount: 1,
      levelIndexes: order.level_index >= 0 ? [order.level_index] : [],
      linkedLevelIndexes: order.linked_level_index >= 0 ? [order.linked_level_index] : [],
      sourceEntryPrices: order.source_entry_price > 0 ? [order.source_entry_price] : [],
    })
    return acc
  }, {})

  Object.values(groupedOrders).forEach((groups) => {
    groups.sort((a, b) => b.price - a.price)
  })

  const formatEntryPriceLabel = (prices: number[]) => {
    const uniquePrices = Array.from(new Set(prices.filter((price) => price > 0).map((price) => formatOrderPrice(price))))
    if (uniquePrices.length === 0) return '-'
    return uniquePrices.join('、')
  }

  const formatBoundarySource = (source: string) => {
    switch (source) {
      case 'manual':
        return ts(gridRisk.manualBoundary, language)
      case 'mid_box':
        return ts(gridRisk.midBoxBoundary, language)
      case 'atr':
        return ts(gridRisk.atrBoundary, language)
      default:
        return ts(gridRisk.defaultBoundary, language)
    }
  }

  const formatGuardSource = (source: string) => {
    switch (source) {
      case 'atr_10x':
        return language === 'zh' ? '10x ATR14' : '10x ATR14'
      case 'fixed_22pct':
        return language === 'zh' ? '固定 22%' : 'Fixed 22%'
      default:
        return source || '-'
    }
  }

  const formatRiskStateLabel = (state: string) => {
    switch (state) {
      case 'warning':
        return language === 'zh' ? '预警' : 'Warning'
      case 'soft_reduce':
        return language === 'zh' ? '软减仓' : 'Soft Reduce'
      case 'hard_reduce':
        return language === 'zh' ? '强减仓' : 'Hard Reduce'
      case 'emergency':
        return language === 'zh' ? '紧急退出' : 'Emergency'
      default:
        return language === 'zh' ? '正常' : 'Normal'
    }
  }

  const riskStateColor = (state: string) => {
    switch (state) {
      case 'warning':
        return '#F59E0B'
      case 'soft_reduce':
        return '#FB923C'
      case 'hard_reduce':
        return '#EF4444'
      case 'emergency':
        return '#DC2626'
      default:
        return '#0ECB81'
    }
  }

  const collapsedGridLevels = Object.values(
    (riskInfo?.grid_levels || []).reduce<Record<string, { price: number; levelIndexes: number[] }>>((acc, level) => {
      const priceLabel = formatOrderPrice(level.price)
      if (!acc[priceLabel]) {
        acc[priceLabel] = { price: level.price, levelIndexes: [] }
      }
      acc[priceLabel].levelIndexes.push(level.index)
      return acc
    }, {})
  )
    .map((group) => ({
      price: group.price,
      levelIndexes: Array.from(new Set(group.levelIndexes)).sort((a, b) => a - b),
    }))
    .filter((group) => group.levelIndexes.length > 1)
    .sort((a, b) => b.price - a.price)

  const cardStyle = {
    background: 'var(--panel-bg)',
    border: '1px solid var(--panel-border)',
  }

  const sectionStyle = {
    background: 'linear-gradient(180deg, color-mix(in srgb, var(--panel-bg-solid) 90%, rgba(255,255,255,0.05) 10%), var(--panel-bg-solid))',
    border: '1px solid color-mix(in srgb, var(--panel-border) 82%, rgba(240,185,11,0.18) 18%)',
    boxShadow: '0 14px 36px rgba(15, 23, 42, 0.06)',
  }
  const subModuleHeader = (
    title: string,
    expandedState: boolean,
    onToggle: () => void,
    accent = '#F0B90B'
  ) => (
    <button
      type="button"
      onClick={onToggle}
      className="mb-4 flex w-full items-center justify-between gap-3 rounded-xl px-0 py-1 text-left"
    >
      <div className="flex items-center gap-3">
        <div className="h-5 w-0.5 rounded-full" style={{ background: accent }} />
        <div className="text-sm font-semibold uppercase tracking-[0.08em]" style={{ color: 'var(--text-primary)' }}>
          {title}
        </div>
      </div>
      <div className="rounded-full border px-2 py-2" style={{ borderColor: 'var(--panel-border)', background: 'var(--panel-bg)' }}>
        {expandedState ? (
          <ChevronUp className="w-3.5 h-3.5" style={{ color: 'var(--text-secondary)' }} />
        ) : (
          <ChevronDown className="w-3.5 h-3.5" style={{ color: 'var(--text-secondary)' }} />
        )}
      </div>
    </button>
  )

  if (loading) {
    return (
      <div className="p-3 text-center text-xs" style={{ color: 'var(--text-secondary)' }}>
        {ts(gridRisk.loading, language)}
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-3 text-center text-xs" style={{ color: '#F6465D' }}>
        {ts(gridRisk.error, language)}: {error}
      </div>
    )
  }

  if (!riskInfo) {
    return (
      <div className="p-3 text-center text-xs" style={{ color: 'var(--text-secondary)' }}>
        {ts(gridRisk.noData, language)}
      </div>
    )
  }

  const riskContent = (
    <>
      {!embedded && (
        <div
          className="flex flex-wrap items-center justify-between gap-4 cursor-pointer px-5 py-4 transition-colors"
          style={{ background: 'transparent' }}
          onClick={() => setExpanded(!expanded)}
        >
          <div className="flex items-start gap-3 min-w-0">
            <div
              className="mt-1 h-10 w-1 shrink-0 rounded-full"
              style={{ background: 'linear-gradient(180deg, rgba(240,185,11,0.88), rgba(240,185,11,0.24))' }}
            />
            <div className="min-w-0 pt-0.5">
              <div className="flex items-center gap-2">
                <Shield className="h-3.5 w-3.5 opacity-70" style={{ color: 'var(--text-secondary)' }} />
                <div className="text-[1.02rem] font-semibold tracking-[0.01em]" style={{ color: 'var(--text-primary)' }}>
                  {ts(gridRisk.gridRisk, language)}
                </div>
              </div>
              <div className="mt-1.5 max-w-2xl text-[13px] leading-6" style={{ color: 'var(--text-secondary)' }}>
                {language === 'zh' ? '网格仓位、杠杆、区间与清算风险总览' : 'Overview of grid exposure, leverage, box range, and liquidation risk'}
              </div>
              <div className="mt-2 max-w-3xl text-[12px] leading-6" style={{ color: 'var(--text-secondary)' }}>
                {ts(gridRisk.compoundingNotice, language)}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2.5 text-sm flex-wrap justify-end">
              <span
                className="rounded-full px-3 py-1 text-xs font-semibold tracking-wide"
                style={{ background: getRegimeColor(riskInfo.regime_level) + '20', color: getRegimeColor(riskInfo.regime_level) }}
              >
                {ts(gridRisk[(riskInfo.regime_level || 'standard') as keyof typeof gridRisk], language)}
              </span>
              <span className="rounded-full px-3 py-1 font-mono text-xs" style={{ background: 'var(--panel-bg-solid)', color: 'var(--text-primary)' }}>
                {riskInfo.effective_leverage.toFixed(1)}x
              </span>
              <span
                className="rounded-full px-3 py-1 font-mono text-xs"
                style={{ background: getPositionColor(riskInfo.position_percent) + '18', color: getPositionColor(riskInfo.position_percent) }}
              >
                {riskInfo.position_percent.toFixed(0)}%
              </span>
              <span className="rounded-full px-3 py-1 font-mono text-xs" style={{ background: 'rgba(14, 203, 129, 0.12)', color: '#0ECB81' }}>
                L {riskInfo.current_long_position.toFixed(1)}
              </span>
              <span className="rounded-full px-3 py-1 font-mono text-xs" style={{ background: 'rgba(246, 70, 93, 0.12)', color: '#F6465D' }}>
                S {riskInfo.current_short_position.toFixed(1)}
              </span>
              <span className="rounded-full px-3 py-1 font-mono text-xs" style={{ background: 'rgba(240, 185, 11, 0.12)', color: '#F0B90B' }}>
                RO {riskInfo.reduce_only_order_count}
              </span>
            </div>
            {expanded ? (
              <div className="rounded-full border px-2.5 py-2" style={{ borderColor: 'var(--panel-border)', background: 'var(--panel-bg)' }}>
                <ChevronUp className="w-4 h-4" style={{ color: 'var(--text-secondary)' }} />
              </div>
            ) : (
              <div className="rounded-full border px-2.5 py-2" style={{ borderColor: 'var(--panel-border)', background: 'var(--panel-bg)' }}>
                <ChevronDown className="w-4 h-4" style={{ color: 'var(--text-secondary)' }} />
              </div>
            )}
          </div>
        </div>
      )}

      {/* Expanded Content */}
      {(embedded || expanded) && (
        <div className={embedded ? 'space-y-4 pb-0' : 'space-y-4 px-5 pb-5'}>
          {collapsedGridLevels.length > 0 && (
            <div
              className="rounded-2xl border px-4 py-3"
              style={{
                borderColor: 'rgba(240, 185, 11, 0.28)',
                background: 'color-mix(in srgb, var(--panel-bg-solid) 92%, rgba(240,185,11,0.08) 8%)',
              }}
            >
              <div className="flex items-start gap-3">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" style={{ color: '#F0B90B' }} />
                <div className="min-w-0">
                  <div className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                    {language === 'zh' ? '检测到层级价格收敛' : 'Grid level price convergence detected'}
                  </div>
                  <div className="mt-1 text-xs leading-6" style={{ color: 'var(--text-secondary)' }}>
                    {language === 'zh'
                      ? '理论上不同层级不应落在同一实际价格。若出现该情况，通常说明当前网格间距偏小，或交易所价格精度过粗。'
                      : 'Different grid levels should not normally collapse into the same actual price. If they do, the grid spacing is likely too tight or exchange price precision is too coarse.'}
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {collapsedGridLevels.map((group) => (
                      <span
                        key={`${group.price}-${group.levelIndexes.join('-')}`}
                        className="rounded-full border px-3 py-1 text-xs font-medium"
                        style={{
                          borderColor: 'rgba(240, 185, 11, 0.24)',
                          background: 'rgba(240, 185, 11, 0.08)',
                          color: 'var(--text-primary)',
                        }}
                      >
                        {formatLevelLabels(group.levelIndexes)} = {formatOrderPrice(group.price)}
                      </span>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          )}

          {embedded && subModuleHeader(language === 'zh' ? '挂单' : 'Orders', ordersExpanded, () => setOrdersExpanded((v) => !v))}
          {ordersExpanded && (
          <div className="rounded-2xl p-4 md:p-5" style={sectionStyle}>
            <div className="mb-4 flex items-center gap-2">
              <Activity className="w-4 h-4" style={{ color: '#F0B90B' }} />
              <span className="text-sm font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>{ts(gridRisk.orderBook, language)}</span>
            </div>
            {riskInfo.grid_orders && riskInfo.grid_orders.length > 0 ? (
              <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                {['short_entry', 'long_entry', 'long_exit', 'short_exit'].map((bucket) => (
                  <div key={bucket} className="rounded-xl p-4" style={{ background: 'var(--panel-bg)' }}>
                    <div className="mb-3 flex items-center justify-between gap-3">
                      <div className="text-sm font-semibold" style={{ color: bucketColor(bucket) }}>
                        {bucketLabel(bucket)}
                      </div>
                      <div className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                        {(groupedOrders[bucket] || []).length}
                      </div>
                    </div>
                    <div className="space-y-2">
                      {(groupedOrders[bucket] || []).length > 0 ? (
                        (groupedOrders[bucket] || []).map((orderGroup) => (
                          <div key={`${bucket}-${orderGroup.priceLabel}`} className="rounded-lg px-3 py-2.5" style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}>
                            <div className="flex items-center justify-between gap-3">
                              <div className="font-mono text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                                {formatOrderPrice(orderGroup.price)}
                              </div>
                              <div className="text-xs font-medium" style={{ color: bucketColor(bucket) }}>
                                {formatUSD(orderGroup.notional)}
                              </div>
                            </div>
                            <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs" style={{ color: 'var(--text-secondary)' }}>
                              <span>{language === 'zh' ? '数量' : 'Qty'} {orderGroup.quantity.toFixed(4)}</span>
                              <span>{language === 'zh' ? '挂单层' : 'Order level'}: {formatLevelLabels(orderGroup.levelIndexes)}</span>
                              {isExitBucket(bucket) && (
                                <>
                                  <span>{language === 'zh' ? '来源开仓层' : 'Entry level'}: {formatSourceLevelLabels(orderGroup.linkedLevelIndexes)}</span>
                                  <span>{language === 'zh' ? '持仓价' : 'Entry'}: {formatEntryPriceLabel(orderGroup.sourceEntryPrices)}</span>
                                </>
                              )}
                              {orderGroup.orderCount > 1 && (
                                <span>{language === 'zh' ? '同价位订单' : 'Same-price orders'}: {orderGroup.orderCount}</span>
                              )}
                            </div>
                          </div>
                        ))
                      ) : (
                        <div className="rounded-lg px-3 py-3 text-sm" style={{ background: 'var(--panel-bg-solid)', color: 'var(--text-secondary)' }}>
                          {ts(gridRisk.noOpenOrders, language)}
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-xl px-4 py-4 text-sm" style={{ background: 'var(--panel-bg)', color: 'var(--text-secondary)' }}>
                {ts(gridRisk.noOpenOrders, language)}
              </div>
            )}
          </div>
          )}

          {embedded && subModuleHeader(language === 'zh' ? '风控' : 'Risk Control', riskExpanded, () => setRiskExpanded((v) => !v))}
          {riskExpanded && (
          <>
          <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
            {/* Leverage */}
            <div className="rounded-2xl p-4 md:p-5" style={sectionStyle}>
              <div className="mb-4 flex items-center gap-2">
                <TrendingUp className="w-4 h-4" style={{ color: '#F0B90B' }} />
                <span className="text-sm font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>{ts(gridRisk.leverageInfo, language)}</span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.currentLeverage, language)}</div>
                  <div className="font-mono text-base md:text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>{riskInfo.current_leverage}x</div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.effectiveLeverage, language)}</div>
                  <div className="font-mono text-base md:text-lg font-semibold" style={{ color: '#F0B90B' }}>{riskInfo.effective_leverage.toFixed(2)}x</div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.recommendedLeverage, language)}</div>
                  <div
                    className="font-mono text-base md:text-lg font-semibold"
                    style={{ color: riskInfo.current_leverage > riskInfo.recommended_leverage ? '#F6465D' : '#0ECB81' }}
                  >
                    {riskInfo.recommended_leverage}x
                  </div>
                </div>
                <div className="space-y-1 md:col-span-3">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.compoundingBase, language)}</div>
                  <div className="font-mono text-base md:text-lg font-semibold" style={{ color: '#F0B90B' }}>{formatUSD(riskInfo.compounding_base)}</div>
                </div>
              </div>
            </div>

            {/* Position */}
            <div className="rounded-2xl p-4 md:p-5" style={sectionStyle}>
              <div className="mb-4 flex items-center gap-2">
                <Activity className="w-4 h-4" style={{ color: '#F0B90B' }} />
                <span className="text-sm font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>{ts(gridRisk.positionInfo, language)}</span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.currentPosition, language)}</div>
                  <div className="font-mono text-base md:text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>{formatUSD(riskInfo.current_position)}</div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.maxPosition, language)}</div>
                  <div className="font-mono text-base md:text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>{formatUSD(riskInfo.max_position)}</div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.positionPercent, language)}</div>
                  <div className="font-mono text-base md:text-lg font-semibold" style={{ color: getPositionColor(riskInfo.position_percent) }}>
                    {riskInfo.position_percent.toFixed(1)}%
                  </div>
                </div>
              </div>
              <div className="mt-4 h-2 rounded-full overflow-hidden" style={{ background: 'var(--panel-border)' }}>
                <div
                  className="h-full rounded-full"
                  style={{ width: `${Math.min(riskInfo.position_percent, 100)}%`, background: getPositionColor(riskInfo.position_percent) }}
                />
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
            {/* Market State */}
            <div className="rounded-2xl p-4 md:p-5" style={sectionStyle}>
              <div className="mb-4 flex items-center gap-2">
                <Shield className="w-4 h-4" style={{ color: '#F0B90B' }} />
                <span className="text-sm font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>{ts(gridRisk.marketState, language)}</span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.regimeLevel, language)}</div>
                  <div className="text-base font-semibold" style={{ color: getRegimeColor(riskInfo.regime_level) }}>
                    {ts(gridRisk[(riskInfo.regime_level || 'standard') as keyof typeof gridRisk], language)}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.currentPrice, language)}</div>
                  <div className="font-mono text-base font-semibold" style={{ color: 'var(--text-primary)' }}>{formatPrice(riskInfo.current_price)}</div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.breakoutLevel, language)}</div>
                  <div className="text-base font-semibold" style={{ color: getBreakoutColor(riskInfo.breakout_level) }}>
                    {ts(gridRisk[(riskInfo.breakout_level || 'none') as keyof typeof gridRisk], language)}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.breakoutDirection, language)}</div>
                  <div
                    className="text-base font-semibold"
                    style={{ color: riskInfo.breakout_direction === 'up' ? '#0ECB81' : riskInfo.breakout_direction === 'down' ? '#F6465D' : 'var(--text-secondary)' }}
                  >
                    {riskInfo.breakout_direction ? ts(gridRisk[riskInfo.breakout_direction as keyof typeof gridRisk], language) : '-'}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                    {language === 'zh' ? '风控状态' : 'Risk State'}
                  </div>
                  <div className="text-base font-semibold" style={{ color: riskStateColor(riskInfo.current_risk_state) }}>
                    {formatRiskStateLabel(riskInfo.current_risk_state)}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                    {language === 'zh' ? '风险方向' : 'Risk Side'}
                  </div>
                  <div className="font-mono text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
                    {riskInfo.current_risk_side || '-'}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                    {language === 'zh' ? '参考止损线' : 'Reference Stop'}
                  </div>
                  <div className="font-mono text-base font-semibold" style={{ color: '#EF4444' }}>
                    {riskInfo.current_short_position > 0
                      ? formatPrice(riskInfo.short_reference_stop_price)
                      : riskInfo.current_long_position > 0
                        ? formatPrice(riskInfo.long_reference_stop_price)
                        : '-'}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                    {language === 'zh' ? '当前浮亏' : 'Open Loss'}
                  </div>
                  <div className="font-mono text-base font-semibold" style={{ color: '#EF4444' }}>
                    {formatUSD(riskInfo.stop_loss_unrealized_loss)} / {formatPct(riskInfo.stop_loss_unrealized_loss_pct)}
                  </div>
                </div>
                <div className="space-y-1 md:col-span-2">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                    {language === 'zh' ? '触发原因' : 'Reason'}
                  </div>
                  <div className="text-sm leading-6" style={{ color: 'var(--text-primary)' }}>
                    {riskInfo.current_risk_reason || '-'}
                  </div>
                </div>
              </div>
            </div>

            {/* Liquidation */}
            <div className="rounded-2xl p-4 md:p-5" style={sectionStyle}>
              <div className="mb-4 flex items-center gap-2">
                <AlertTriangle className="w-4 h-4" style={{ color: '#F6465D' }} />
                <span className="text-sm font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>{ts(gridRisk.liquidationInfo, language)}</span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.liquidationPrice, language)}</div>
                  <div className="font-mono text-base md:text-lg font-semibold" style={{ color: '#F6465D' }}>
                    {riskInfo.liquidation_price > 0 ? formatPrice(riskInfo.liquidation_price) : '-'}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.liquidationDistance, language)}</div>
                  <div className="font-mono text-base md:text-lg font-semibold" style={{ color: '#F6465D' }}>
                    {riskInfo.liquidation_distance > 0 ? `${riskInfo.liquidation_distance.toFixed(1)}%` : '-'}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
            <div className="rounded-2xl p-4 md:p-5" style={sectionStyle}>
              <div className="mb-4 flex items-center gap-2">
                <Activity className="w-4 h-4" style={{ color: '#0ECB81' }} />
                <span className="text-sm font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>{ts(gridRisk.inventoryInfo, language)}</span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.longPosition, language)}</div>
                  <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: '#0ECB81' }}>
                    {formatUSD(riskInfo.current_long_value)} / {riskInfo.current_long_position.toFixed(2)}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.shortPosition, language)}</div>
                  <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: '#F6465D' }}>
                    {formatUSD(riskInfo.current_short_value)} / {riskInfo.current_short_position.toFixed(2)}
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.longLevels, language)}</div>
                  <div className="font-mono text-base font-semibold" style={{ color: '#0ECB81' }}>{riskInfo.long_filled_levels}</div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.shortLevels, language)}</div>
                  <div className="font-mono text-base font-semibold" style={{ color: '#F6465D' }}>{riskInfo.short_filled_levels}</div>
                </div>
              </div>
            </div>

            <div className="rounded-2xl p-4 md:p-5" style={sectionStyle}>
              <div className="mb-4 flex items-center gap-2">
                <Activity className="w-4 h-4" style={{ color: '#F0B90B' }} />
                <span className="text-sm font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>{ts(gridRisk.orderInventory, language)}</span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.entryOrders, language)}</div>
                  <div className="font-mono text-base font-semibold" style={{ color: 'var(--text-primary)' }}>{riskInfo.entry_order_count}</div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.reduceOnlyOrders, language)}</div>
                  <div className="font-mono text-base font-semibold" style={{ color: '#F0B90B' }}>{riskInfo.reduce_only_order_count}</div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.longOrders, language)}</div>
                  <div className="font-mono text-base font-semibold" style={{ color: '#0ECB81' }}>{riskInfo.long_order_count}</div>
                </div>
                <div className="space-y-1">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.shortOrders, language)}</div>
                  <div className="font-mono text-base font-semibold" style={{ color: '#F6465D' }}>{riskInfo.short_order_count}</div>
                </div>
                <div className="space-y-1 md:col-span-2">
                  <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.avgOrderNotional, language)}</div>
                  <div className="font-mono text-base font-semibold" style={{ color: 'var(--text-primary)' }}>{formatUSD(riskInfo.average_order_notional)}</div>
                </div>
              </div>
            </div>
          </div>

          <div className="rounded-2xl p-4 md:p-5" style={sectionStyle}>
            <div className="mb-4 flex items-center gap-2">
              <Box className="w-4 h-4" style={{ color: '#F0B90B' }} />
              <span className="text-sm font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>{ts(gridRisk.boundaryState, language)}</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4 text-sm">
              <div className="space-y-1">
                <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.gridBounds, language)}</div>
                <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: 'var(--text-primary)' }}>
                  {formatPrice(riskInfo.grid_lower_price)} - {formatPrice(riskInfo.grid_upper_price)}
                </div>
              </div>
              <div className="space-y-1">
                <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.boundarySource, language)}</div>
                <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: '#F0B90B' }}>
                  {formatBoundarySource(riskInfo.grid_boundary_source)}
                </div>
              </div>
              <div className="space-y-1">
                <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.currentSpacing, language)}</div>
                <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: 'var(--text-primary)' }}>
                  {formatPrice(riskInfo.grid_spacing)}
                </div>
              </div>
              <div className="space-y-1">
                <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.minSpacing, language)}</div>
                <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: '#0ECB81' }}>
                  {formatPrice(riskInfo.min_grid_spacing)} / {(0.50).toFixed(2)}%
                </div>
              </div>
              <div className="space-y-1">
                <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.maxSpacing, language)}</div>
                <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: '#F6465D' }}>
                  {formatPrice(riskInfo.max_grid_spacing)} / {(1.20).toFixed(2)}%
                </div>
              </div>
              <div className="space-y-1">
                <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                  {language === 'zh' ? '首仓保护源' : 'First Entry Guard Source'}
                </div>
                <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: '#F97316' }}>
                  {formatGuardSource(riskInfo.first_entry_guard_active_source)}
                </div>
              </div>
              <div className="space-y-1">
                <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                  {language === 'zh' ? '固定带 / ATR带' : 'Fixed / ATR Band'}
                </div>
                <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: 'var(--text-primary)' }}>
                  {formatPrice(riskInfo.first_entry_guard_fixed_band)} / {formatPrice(riskInfo.first_entry_guard_atr_band)}
                </div>
              </div>
              <div className="space-y-1">
                <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                  {language === 'zh' ? '空头首仓保护线' : 'Short First Entry Guard'}
                </div>
                <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: '#F97316' }}>
                  {formatPrice(riskInfo.short_first_entry_guard_price)}
                </div>
              </div>
              <div className="space-y-1">
                <div className="text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                  {language === 'zh' ? '多头首仓保护线' : 'Long First Entry Guard'}
                </div>
                <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: '#22C55E' }}>
                  {formatPrice(riskInfo.long_first_entry_guard_price)}
                </div>
              </div>
            </div>
            <div className="mt-4 rounded-xl px-4 py-3 text-sm leading-6" style={{ background: 'var(--panel-bg)' }}>
              <div className="mb-1 text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                {ts(gridRisk.boundaryLogic, language)}
              </div>
              <div style={{ color: 'var(--text-primary)' }}>
                {ts(gridRisk.boundaryLogicDesc, language)}
              </div>
            </div>
          </div>

          <div className="rounded-2xl p-4 md:p-5" style={sectionStyle}>
            <div className="mb-4 flex items-center gap-2">
              <Box className="w-4 h-4" style={{ color: '#F0B90B' }} />
              <span className="text-sm font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>{ts(gridRisk.boxState, language)}</span>
            </div>
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 text-sm">
              <div className="rounded-xl px-4 py-3" style={{ background: 'var(--panel-bg)' }}>
                <div className="mb-1 text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.shortBox, language)}</div>
                <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: 'var(--text-primary)' }}>
                  {formatPrice(riskInfo.short_box_lower)} - {formatPrice(riskInfo.short_box_upper)}
                </div>
              </div>
              <div className="rounded-xl px-4 py-3" style={{ background: 'var(--panel-bg)' }}>
                <div className="mb-1 text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.midBox, language)}</div>
                <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: 'var(--text-primary)' }}>
                  {formatPrice(riskInfo.mid_box_lower)} - {formatPrice(riskInfo.mid_box_upper)}
                </div>
              </div>
              <div className="rounded-xl px-4 py-3" style={{ background: 'var(--panel-bg)' }}>
                <div className="mb-1 text-xs md:text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>{ts(gridRisk.longBox, language)}</div>
                <div className="font-mono text-sm md:text-base font-semibold leading-relaxed" style={{ color: 'var(--text-primary)' }}>
                  {formatPrice(riskInfo.long_box_lower)} - {formatPrice(riskInfo.long_box_upper)}
                </div>
              </div>
            </div>
          </div>
          </>
          )}

        </div>
      )}
    </>
  )

  if (embedded) {
    return <div className="space-y-0">{riskContent}</div>
  }

  return (
    <div className="rounded-2xl overflow-hidden" style={cardStyle}>
      {riskContent}
    </div>
  )
}
