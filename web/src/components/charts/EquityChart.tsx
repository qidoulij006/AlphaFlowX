import { useState } from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
} from 'recharts'
import useSWR from 'swr'
import { api } from '../../lib/api'
import { useLanguage } from '../../contexts/LanguageContext'
import { useAuth } from '../../contexts/AuthContext'
import { BRAND_INFO } from '../../constants/branding'
import { t } from '../../i18n/translations'
import {
  AlertTriangle,
  BarChart3,
  DollarSign,
  Percent,
  TrendingUp as ArrowUp,
  TrendingDown as ArrowDown,
} from 'lucide-react'

interface EquityPoint {
  timestamp: string
  total_equity: number
  pnl: number
  pnl_pct: number
  cycle_number: number
}

interface EquityChartProps {
  traderId?: string
  embedded?: boolean // 嵌入模式（不显示外层卡片）
  shareToken?: string
}

type RangePreset = '1day' | '7day' | 'all'

export function EquityChart({ traderId, embedded = false, shareToken }: EquityChartProps) {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  const [displayMode, setDisplayMode] = useState<'dollar' | 'percent'>('dollar')
  const [rangePreset, setRangePreset] = useState<RangePreset>('all')

  const { data: history, error, isLoading } = useSWR<EquityPoint[]>(
    traderId && (shareToken || (user && token)) ? `equity-history-${traderId}-${shareToken || 'auth'}` : null,
    () => shareToken ? api.getSharedEquityHistory(shareToken) : api.getEquityHistory(traderId),
    {
      refreshInterval: 30000, // 30秒刷新（历史数据更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  const { data: account } = useSWR(
    traderId && (shareToken || (user && token)) ? `account-${traderId}-${shareToken || 'auth'}` : null,
    () => shareToken ? api.getSharedAccount(shareToken) : api.getAccount(traderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端缓存）
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    }
  )

  const historyWithLivePoint = (() => {
    const historical = history && history.length > 0 ? [...history] : []
    if (!account) return historical

    const livePoint: EquityPoint = {
      timestamp: new Date().toISOString(),
      total_equity: account.total_equity,
      pnl: account.total_pnl,
      pnl_pct: account.total_pnl_pct,
      cycle_number: historical.length > 0
        ? (historical[historical.length - 1].cycle_number ?? historical.length) + 1
        : 1,
    }

    if (historical.length === 0) {
      return [livePoint]
    }

    const lastPoint = historical[historical.length - 1]
    const lastTimestamp = new Date(lastPoint.timestamp).getTime()
    const liveTimestamp = new Date(livePoint.timestamp).getTime()
    const sameEquity = Math.abs(lastPoint.total_equity - livePoint.total_equity) < 0.0001

    // Keep the full historical curve, only append a live point when it is meaningfully newer
    // or the latest equity has changed.
    if (liveTimestamp - lastTimestamp > 60_000 || !sameEquity) {
      historical.push(livePoint)
    }

    return historical
  })()

  // Loading state - show skeleton
  if (isLoading) {
    return (
      <div className={embedded ? 'p-6' : 'binance-card p-6'}>
        {!embedded && (
          <h3 className="text-lg font-semibold mb-6" style={{ color: 'var(--text-primary)' }}>
            {t('accountEquityCurve', language)}
          </h3>
        )}
        <div className="animate-pulse">
          <div className="skeleton h-64 w-full rounded"></div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className={embedded ? 'p-6' : 'binance-card p-6'}>
        <div
          className="flex items-center gap-3 p-4 rounded"
          style={{
            background: 'rgba(246, 70, 93, 0.1)',
            border: '1px solid rgba(246, 70, 93, 0.2)',
          }}
        >
          <AlertTriangle className="w-6 h-6" style={{ color: '#F6465D' }} />
          <div>
            <div className="font-semibold" style={{ color: '#F6465D' }}>
              {t('loadingError', language)}
            </div>
            <div className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              {error.message}
            </div>
          </div>
        </div>
      </div>
    )
  }

  // 过滤掉无效数据：total_equity为0或小于1的数据点（API失败导致）
  const validHistory = historyWithLivePoint.filter((point) => point.total_equity > 1) || []

  if (!validHistory || validHistory.length === 0) {
    return (
      <div className={embedded ? 'p-6' : 'binance-card p-6'}>
        {!embedded && (
          <h3 className="text-lg font-semibold mb-6" style={{ color: 'var(--text-primary)' }}>
            {t('accountEquityCurve', language)}
          </h3>
        )}
        <div className="text-center py-16" style={{ color: 'var(--text-secondary)' }}>
          <div className="mb-4 flex justify-center opacity-50">
            <BarChart3 className="w-16 h-16" />
          </div>
          <div className="text-lg font-semibold mb-2">
            {t('noHistoricalData', language)}
          </div>
          <div className="text-sm">{account ? (language === 'zh' ? '当前已使用实时权益兜底，历史曲线会在快照生成后补齐' : 'Showing live equity fallback until snapshots are available') : t('dataWillAppear', language)}</div>
        </div>
      </div>
    )
  }

  const now = Date.now()
  const rangeStart =
    rangePreset === '1day'
      ? now - 24 * 60 * 60 * 1000
      : rangePreset === '7day'
        ? now - 7 * 24 * 60 * 60 * 1000
        : null

  const filteredHistory =
    rangeStart == null
      ? validHistory
      : validHistory.filter((point) => new Date(point.timestamp).getTime() >= rangeStart)

  const displayHistory = filteredHistory.length > 0 ? filteredHistory : validHistory

  // 计算初始余额（优先从 account 获取配置的初始余额，备选从历史数据反推）
  const initialBalance =
    account?.initial_balance || // 从交易员配置读取真实初始余额
    (validHistory[0]
      ? validHistory[0].total_equity - validHistory[0].pnl
      : undefined) || // 备选：淨值 - 盈亏
    1000 // 默认值（与创建交易员时的默认配置一致）

  // 转换数据格式
  const chartData = displayHistory.map((point, index) => {
    const pnl = point.total_equity - initialBalance
    const pnlPct = ((pnl / initialBalance) * 100).toFixed(2)
    const pointDate = new Date(point.timestamp)
    return {
      time: pointDate.toLocaleTimeString(language === 'zh' ? 'zh-CN' : 'en-US', {
        hour: '2-digit',
        minute: '2-digit',
      }),
      fullTime: pointDate.toLocaleString(language === 'zh' ? 'zh-CN' : 'en-US', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      }),
      value: displayMode === 'dollar' ? point.total_equity : parseFloat(pnlPct),
      cycle: point.cycle_number ?? index + 1,
      raw_equity: point.total_equity,
      raw_pnl: pnl,
      raw_pnl_pct: parseFloat(pnlPct),
    }
  })

  const currentValue = chartData[chartData.length - 1]
  const isProfit = currentValue.raw_pnl >= 0
  const hideXAxis = chartData.length > 36

  // 计算Y轴范围
  const calculateYDomain = () => {
    if (displayMode === 'percent') {
      // 百分比模式：找到最大最小值，留20%余量
      const values = chartData.map((d) => d.value)
      const minVal = Math.min(...values)
      const maxVal = Math.max(...values)
      const range = Math.max(Math.abs(maxVal), Math.abs(minVal))
      const padding = Math.max(range * 0.2, 1) // 至少留1%余量
      return [Math.floor(minVal - padding), Math.ceil(maxVal + padding)]
    } else {
      // 美元模式：以初始余额为基准，上下留10%余量
      const values = chartData.map((d) => d.value)
      const minVal = Math.min(...values, initialBalance)
      const maxVal = Math.max(...values, initialBalance)
      const range = maxVal - minVal
      const padding = Math.max(range * 0.15, initialBalance * 0.01) // 至少留1%余量
      return [Math.floor(minVal - padding), Math.ceil(maxVal + padding)]
    }
  }

  // 自定义Tooltip - Binance Style
  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      return (
        <div
          className="rounded p-3 shadow-xl"
          style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}
        >
          <div className="text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>
            Cycle #{data.cycle != null ? data.cycle : '—'}
          </div>
          <div className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
            {data.fullTime}
          </div>
          <div className="font-bold mono" style={{ color: 'var(--text-primary)' }}>
            {data.raw_equity.toFixed(2)} USDT
          </div>
          <div
            className="text-sm mono font-bold"
            style={{ color: data.raw_pnl >= 0 ? '#0ECB81' : '#F6465D' }}
          >
            {data.raw_pnl >= 0 ? '+' : ''}
            {data.raw_pnl.toFixed(2)} USDT ({data.raw_pnl_pct >= 0 ? '+' : ''}
            {data.raw_pnl_pct}%)
          </div>
        </div>
      )
    }
    return null
  }

  return (
    <div className={embedded ? 'p-3 sm:p-5' : 'binance-card p-3 sm:p-5 animate-fade-in'}>
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-4">
        <div className="flex-1">
          {!embedded && (
            <h3
              className="text-base sm:text-lg font-bold mb-2"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('accountEquityCurve', language)}
            </h3>
          )}
          <div className="flex flex-col sm:flex-row sm:items-baseline gap-2 sm:gap-4">
            <span
              className="text-2xl sm:text-3xl font-bold mono"
              style={{ color: 'var(--text-primary)' }}
            >
              {account?.total_equity.toFixed(2) || '0.00'}
              <span
                className="text-base sm:text-lg ml-1"
                style={{ color: 'var(--text-secondary)' }}
              >
                USDT
              </span>
            </span>
            <div className="flex items-center gap-2 flex-wrap">
              <span
                className="text-sm sm:text-lg font-bold mono px-2 sm:px-3 py-1 rounded flex items-center gap-1"
                style={{
                  color: isProfit ? '#0ECB81' : '#F6465D',
                  background: isProfit
                    ? 'rgba(14, 203, 129, 0.1)'
                    : 'rgba(246, 70, 93, 0.1)',
                  border: `1px solid ${
                    isProfit
                      ? 'rgba(14, 203, 129, 0.2)'
                      : 'rgba(246, 70, 93, 0.2)'
                  }`,
                }}
              >
                {isProfit ? (
                  <ArrowUp className="w-4 h-4" />
                ) : (
                  <ArrowDown className="w-4 h-4" />
                )}
                {isProfit ? '+' : ''}
                {currentValue.raw_pnl_pct}%
              </span>
              <span
                className="text-xs sm:text-sm mono"
                style={{ color: 'var(--text-secondary)' }}
              >
                ({isProfit ? '+' : ''}
                {currentValue.raw_pnl.toFixed(2)} USDT)
              </span>
            </div>
          </div>
        </div>

        <div className="flex flex-col gap-2 self-start sm:self-auto">
          <div
            className="flex gap-0.5 sm:gap-1 rounded p-0.5 sm:p-1"
            style={{
              background: 'var(--panel-bg-solid)',
              border: '1px solid var(--panel-border)',
            }}
          >
            <button
              onClick={() => setRangePreset('1day')}
              className="px-3 py-1.5 rounded text-xs font-bold transition-all"
              style={
                rangePreset === '1day'
                  ? {
                      background: '#F0B90B',
                      color: '#000',
                      boxShadow: '0 2px 8px rgba(240, 185, 11, 0.4)',
                    }
                  : { background: 'transparent', color: 'var(--text-secondary)' }
              }
            >
              1day
            </button>
            <button
              onClick={() => setRangePreset('7day')}
              className="px-3 py-1.5 rounded text-xs font-bold transition-all"
              style={
                rangePreset === '7day'
                  ? {
                      background: '#F0B90B',
                      color: '#000',
                      boxShadow: '0 2px 8px rgba(240, 185, 11, 0.4)',
                    }
                  : { background: 'transparent', color: 'var(--text-secondary)' }
              }
            >
              7day
            </button>
            <button
              onClick={() => setRangePreset('all')}
              className="px-3 py-1.5 rounded text-xs font-bold transition-all"
              style={
                rangePreset === 'all'
                  ? {
                      background: '#F0B90B',
                      color: '#000',
                      boxShadow: '0 2px 8px rgba(240, 185, 11, 0.4)',
                    }
                  : { background: 'transparent', color: 'var(--text-secondary)' }
              }
            >
              All
            </button>
          </div>

          <div
            className="flex gap-0.5 sm:gap-1 rounded p-0.5 sm:p-1"
            style={{
              background: 'var(--panel-bg-solid)',
              border: '1px solid var(--panel-border)',
            }}
          >
            <button
              onClick={() => setDisplayMode('dollar')}
              className="px-3 sm:px-4 py-1.5 sm:py-2 rounded text-xs sm:text-sm font-bold transition-all flex items-center gap-1"
              style={
                displayMode === 'dollar'
                  ? {
                      background: '#F0B90B',
                      color: '#000',
                      boxShadow: '0 2px 8px rgba(240, 185, 11, 0.4)',
                    }
                  : { background: 'transparent', color: 'var(--text-secondary)' }
              }
            >
              <DollarSign className="w-4 h-4" /> USDT
            </button>
            <button
              onClick={() => setDisplayMode('percent')}
              className="px-3 sm:px-4 py-1.5 sm:py-2 rounded text-xs sm:text-sm font-bold transition-all flex items-center gap-1"
              style={
                displayMode === 'percent'
                  ? {
                      background: '#F0B90B',
                      color: '#000',
                      boxShadow: '0 2px 8px rgba(240, 185, 11, 0.4)',
                    }
                  : { background: 'transparent', color: 'var(--text-secondary)' }
              }
            >
              <Percent className="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      {/* Chart */}
      <div
        className="my-2"
        style={{
          borderRadius: '8px',
          overflow: 'hidden',
          position: 'relative',
        }}
      >
        {/* Brand watermark */}
        <div
          style={{
            position: 'absolute',
            top: '15px',
            right: '15px',
            fontSize: '20px',
            fontWeight: 'bold',
            color: 'rgba(240, 185, 11, 0.15)',
            zIndex: 10,
            pointerEvents: 'none',
            fontFamily: 'monospace',
          }}
        >
          {BRAND_INFO.shortName}
        </div>
        <ResponsiveContainer width="100%" height={280}>
          <LineChart
            data={chartData}
            margin={{ top: 10, right: 20, left: 5, bottom: 30 }}
          >
            <defs>
              <linearGradient id="colorGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#F0B90B" stopOpacity={0.8} />
                <stop offset="95%" stopColor="#FCD535" stopOpacity={0.2} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--panel-border)" />
            <XAxis
              hide={hideXAxis}
              dataKey="time"
              stroke="var(--text-tertiary)"
              tick={{ fill: 'var(--text-secondary)', fontSize: 11 }}
              tickLine={{ stroke: 'var(--panel-border)' }}
              axisLine={{ stroke: 'var(--panel-border)' }}
              interval={Math.max(0, Math.floor(chartData.length / 10))}
              angle={-15}
              textAnchor="end"
              height={60}
            />
            <YAxis
              stroke="var(--text-tertiary)"
              tick={{ fill: 'var(--text-secondary)', fontSize: 12 }}
              tickLine={{ stroke: 'var(--panel-border)' }}
              domain={calculateYDomain()}
              tickFormatter={(value) =>
                displayMode === 'dollar' ? `$${value.toFixed(0)}` : `${value}%`
              }
            />
            <Tooltip content={<CustomTooltip />} />
            <ReferenceLine
              y={displayMode === 'dollar' ? initialBalance : 0}
              stroke="var(--text-disabled)"
              strokeDasharray="3 3"
              label={{
                value:
                  displayMode === 'dollar'
                    ? t('initialBalance', language).split(' ')[0]
                    : '0%',
                fill: 'var(--text-secondary)',
                fontSize: 12,
              }}
            />
            <Line
              type="natural"
              dataKey="value"
              stroke="url(#colorGradient)"
              strokeWidth={3}
              dot={chartData.length > 50 ? false : { fill: '#F0B90B', r: 3 }}
              activeDot={{
                r: 6,
                fill: '#FCD535',
                stroke: '#F0B90B',
                strokeWidth: 2,
              }}
              connectNulls={true}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/* Footer Stats */}
      <div
        className="mt-3 grid grid-cols-2 sm:grid-cols-4 gap-2 sm:gap-3 pt-3"
        style={{ borderTop: '1px solid var(--panel-border)' }}
      >
        <div
          className="p-2 rounded transition-all hover:bg-opacity-50"
          style={{ background: 'rgba(240, 185, 11, 0.05)' }}
        >
          <div
            className="text-xs mb-1 uppercase tracking-wider"
            style={{ color: 'var(--text-secondary)' }}
          >
            {t('initialBalance', language)}
          </div>
          <div
            className="text-xs sm:text-sm font-bold mono"
            style={{ color: 'var(--text-primary)' }}
          >
            {initialBalance.toFixed(2)} USDT
          </div>
        </div>
        <div
          className="p-2 rounded transition-all hover:bg-opacity-50"
          style={{ background: 'rgba(240, 185, 11, 0.05)' }}
        >
          <div
            className="text-xs mb-1 uppercase tracking-wider"
            style={{ color: 'var(--text-secondary)' }}
          >
            {t('currentEquity', language)}
          </div>
          <div
            className="text-xs sm:text-sm font-bold mono"
            style={{ color: 'var(--text-primary)' }}
          >
            {currentValue.raw_equity.toFixed(2)} USDT
          </div>
        </div>
        <div
          className="p-2 rounded transition-all hover:bg-opacity-50"
          style={{ background: 'rgba(240, 185, 11, 0.05)' }}
        >
          <div
            className="text-xs mb-1 uppercase tracking-wider"
            style={{ color: 'var(--text-secondary)' }}
          >
            {t('historicalCycles', language)}
          </div>
          <div
            className="text-xs sm:text-sm font-bold mono"
            style={{ color: 'var(--text-primary)' }}
          >
            {displayHistory.length} {t('cycles', language)}
          </div>
        </div>
        <div
          className="p-2 rounded transition-all hover:bg-opacity-50"
          style={{ background: 'rgba(240, 185, 11, 0.05)' }}
        >
          <div
            className="text-xs mb-1 uppercase tracking-wider"
            style={{ color: 'var(--text-secondary)' }}
          >
            {t('displayRange', language)}
          </div>
          <div
            className="text-xs sm:text-sm font-bold mono"
            style={{ color: 'var(--text-primary)' }}
          >
            {rangePreset === '1day' ? '1day' : rangePreset === '7day' ? '7day' : 'All'}
          </div>
        </div>
      </div>
    </div>
  )
}
