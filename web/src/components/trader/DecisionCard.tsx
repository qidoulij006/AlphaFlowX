import { useState } from 'react'
import type { DecisionRecord, DecisionAction } from '../../types'
import { t, type Language } from '../../i18n/translations'

interface DecisionCardProps {
  decision: DecisionRecord
  language: Language
  onSymbolClick?: (symbol: string) => void
}

const ACTION_STYLE: Record<string, { color: string; bg: string; icon: string }> = {
  open_long: { color: '#0ECB81', bg: 'rgba(14, 203, 129, 0.15)', icon: '📈' },
  open_short: { color: '#F6465D', bg: 'rgba(246, 70, 93, 0.15)', icon: '📉' },
  close_long: { color: '#F0B90B', bg: 'rgba(240, 185, 11, 0.15)', icon: '💰' },
  close_short: { color: '#F0B90B', bg: 'rgba(240, 185, 11, 0.15)', icon: '💰' },
  place_buy_limit: { color: '#0ECB81', bg: 'rgba(14, 203, 129, 0.15)', icon: '🟢' },
  place_sell_limit: { color: '#F6465D', bg: 'rgba(246, 70, 93, 0.15)', icon: '🔴' },
  cancel_order: { color: '#F0B90B', bg: 'rgba(240, 185, 11, 0.15)', icon: '🧹' },
  cancel_all_orders: { color: '#F0B90B', bg: 'rgba(240, 185, 11, 0.15)', icon: '🧹' },
  pause_grid: { color: '#F6465D', bg: 'rgba(246, 70, 93, 0.15)', icon: '⏸️' },
  resume_grid: { color: '#0ECB81', bg: 'rgba(14, 203, 129, 0.15)', icon: '▶️' },
  adjust_grid: { color: '#60A5FA', bg: 'rgba(96, 165, 250, 0.15)', icon: '🛠️' },
  hold: { color: '#848E9C', bg: 'rgba(132, 142, 156, 0.15)', icon: '⏸️' },
  wait: { color: '#848E9C', bg: 'rgba(132, 142, 156, 0.15)', icon: '⏳' },
}

// Format price with proper decimals
function formatPrice(price: number | undefined): string {
  if (!price || price === 0) return '-'
  if (price >= 1000) return price.toFixed(2)
  if (price >= 1) return price.toFixed(4)
  return price.toFixed(6)
}

// Calculate percentage change
function calcPctChange(entry: number | undefined, target: number | undefined, isLong: boolean): string {
  if (!entry || !target || entry === 0) return '-'
  const pct = ((target - entry) / entry) * 100
  const adjustedPct = isLong ? pct : -pct
  return `${adjustedPct >= 0 ? '+' : ''}${adjustedPct.toFixed(2)}%`
}

// Get confidence color
function getConfidenceColor(confidence: number | undefined): string {
  if (!confidence) return '#848E9C'
  if (confidence >= 80) return '#0ECB81'
  if (confidence >= 60) return '#F0B90B'
  return '#F6465D'
}

function extractNumberPair(message: string, currentKey: string, limitKey: string): { current?: string; limit?: string } {
  const currentMatch = message.match(new RegExp(`${currentKey}=\\$?([0-9.]+)`))
  const limitMatch = message.match(new RegExp(`${limitKey}=\\$?([0-9.]+)`))
  return {
    current: currentMatch?.[1],
    limit: limitMatch?.[1],
  }
}

function buildChineseExecutionSummary(message: string): string | null {
  if (message.includes('TOTAL POSITION LIMIT EXCEEDED')) {
    const { current, limit } = extractNumberPair(message, 'current', 'max')
    if (current && limit) {
      return `因总仓位超限被拒单，当前总仓位 $${current}，上限 $${limit}。`
    }
    return '因总仓位超限被拒单。'
  }

  if (message.includes('total position value') && message.includes('would exceed limit')) {
    const currentMatch = message.match(/total position value \$([0-9.]+) would exceed limit \$([0-9.]+)/)
    if (currentMatch) {
      return `因总仓位超限被拒单，下单后总仓位将达 $${currentMatch[1]}，超过上限 $${currentMatch[2]}。`
    }
    return '因总仓位超限被拒单。'
  }

  if (message.includes('failed to place limit order')) {
    if (message.includes('Margin is insufficient') || message.includes('code=-2019')) {
      return '交易所拒绝下单：可用保证金不足。通常是当前仓位、挂单占用过高，或账户可用余额不够。'
    }
    if (message.includes('ReduceOnly Order is rejected') || message.includes('code=-2022')) {
      return '交易所拒绝平仓单：这张 reduce-only 单当前不能成立。通常是可平仓位不足、方向不匹配，或已有挂单已经占用了可平数量。'
    }
    if (message.includes('ReduceOnly Order Failed') || message.includes('code=-4118')) {
      return '交易所拒绝下单：该平仓单未能通过校验。通常表示当前可平仓位不足，或已有挂单/仓位状态与这张 reduce-only 单冲突。'
    }
    if (message.includes('Invalid API-key, IP, or permissions for action') || message.includes('code=-2015')) {
      return '交易所拒绝访问：请检查 API Key、API Secret、权限开通情况，以及白名单 IP 是否正确。'
    }
    if (message.includes('Timestamp for this request') || message.includes('code=-1021')) {
      return '交易所拒绝请求：服务器时间与交易所时间偏差过大，请检查系统时钟同步。'
    }
    if (message.includes('Filter failure: PRICE_FILTER') || message.includes('PRICE_FILTER')) {
      return '交易所拒绝下单：价格不符合最小变动单位或允许范围，请调整挂单价格。'
    }
    if (message.includes('Filter failure: LOT_SIZE') || message.includes('LOT_SIZE')) {
      return '交易所拒绝下单：下单数量不符合最小数量或步进要求，请调整数量。'
    }
    if (message.includes('Filter failure: MIN_NOTIONAL') || message.includes('MIN_NOTIONAL')) {
      return '交易所拒绝下单：单笔订单金额太小，没有达到交易所的最小成交额要求。'
    }
    if (message.includes('Order would immediately trigger') || message.includes('code=-2021')) {
      return '交易所拒绝下单：这张条件单如果提交会立刻触发，参数设置不合理。'
    }
    if (message.includes('position side does not match') || message.includes('Position side does not match')) {
      return '交易所拒绝下单：仓位方向设置不匹配，请检查单向持仓或双向持仓模式。'
    }
    return '限价单下单失败。'
  }

  if (message.includes('position value') && message.includes('exceeds safety limit')) {
    const matched = message.match(/position value \$([0-9.]+) exceeds safety limit \$([0-9.]+)/)
    if (matched) {
      return `因仓位价值超过安全限制被拒单，仓位价值 $${matched[1]}，安全上限 $${matched[2]}。`
    }
    return '因仓位价值超过安全限制被拒单。'
  }

  if (message.includes('Trader stopped')) {
    return '交易器已停止，剩余动作未执行。'
  }

  if (message.includes('failed to cancel order')) {
    return '撤单失败，交易所未接受这次撤单请求。'
  }

  if (message.includes('failed to cancel all orders')) {
    return '全部撤单失败，交易所未接受这次批量撤单请求。'
  }

  if (message.includes('failed to get market price')) {
    return '获取最新市场价格失败，这次动作已跳过。'
  }

  if (message.includes('HTTP 401') || message.includes('unauthorized') || message.includes('Unauthorized')) {
    return '请求失败：当前登录状态已失效，请重新登录后再试。'
  }

  if (message.includes('HTTP 403') || message.includes('forbidden') || message.includes('Forbidden')) {
    return '请求失败：当前账号没有执行该操作的权限。'
  }

  if (message.includes('HTTP 404') || message.includes('not found') || message.includes('Not Found')) {
    return '请求失败：目标资源不存在，或当前数据状态已经变化。'
  }

  if (message.includes('HTTP 429') || message.includes('Too Many Requests')) {
    return '请求过于频繁，被系统或交易所限流，请稍后再试。'
  }

  if (message.includes('timeout') || message.includes('context deadline exceeded')) {
    return '请求超时，后端或交易所在限定时间内没有返回结果。'
  }

  if (message.includes('network') || message.includes('connection refused') || message.includes('EOF')) {
    return '请求失败：后端或交易所连接中断。'
  }

  return null
}

function buildEnglishExecutionSummary(message: string): string | null {
  if (message.includes('TOTAL POSITION LIMIT EXCEEDED')) {
    const { current, limit } = extractNumberPair(message, 'current', 'max')
    if (current && limit) {
      return `Order rejected because total exposure is already too high. Current exposure is $${current} and the limit is $${limit}.`
    }
    return 'Order rejected because total exposure is already above the allowed limit.'
  }

  if (message.includes('total position value') && message.includes('would exceed limit')) {
    const currentMatch = message.match(/total position value \$([0-9.]+) would exceed limit \$([0-9.]+)/)
    if (currentMatch) {
      return `Order rejected because total exposure would rise to $${currentMatch[1]}, above the limit of $${currentMatch[2]}.`
    }
    return 'Order rejected because total exposure would exceed the configured limit.'
  }

  if (message.includes('failed to place limit order')) {
    if (message.includes('Margin is insufficient') || message.includes('code=-2019')) {
      return 'Exchange rejected the order because available margin is insufficient. Existing positions or working orders are likely using too much margin.'
    }
    if (message.includes('ReduceOnly Order is rejected') || message.includes('code=-2022')) {
      return 'Exchange rejected the reduce-only exit order. This usually means there is not enough closable position left, the order side is wrong for the current position, or other exit orders already reserved that closable size.'
    }
    if (message.includes('ReduceOnly Order Failed') || message.includes('code=-4118')) {
      return 'Exchange rejected the reduce-only order. This usually means there is not enough closable position left, or an existing order/position conflicts with this exit order.'
    }
    if (message.includes('Invalid API-key, IP, or permissions for action') || message.includes('code=-2015')) {
      return 'Exchange rejected the request. Check the API key, API secret, enabled permissions, and the whitelisted IP address.'
    }
    if (message.includes('Timestamp for this request') || message.includes('code=-1021')) {
      return 'Exchange rejected the request because the server time is too far off. Check time synchronization on the host.'
    }
    if (message.includes('Filter failure: PRICE_FILTER') || message.includes('PRICE_FILTER')) {
      return 'Exchange rejected the order because the price does not fit the allowed tick size or price range.'
    }
    if (message.includes('Filter failure: LOT_SIZE') || message.includes('LOT_SIZE')) {
      return 'Exchange rejected the order because the quantity does not match the minimum size or step size rules.'
    }
    if (message.includes('Filter failure: MIN_NOTIONAL') || message.includes('MIN_NOTIONAL')) {
      return 'Exchange rejected the order because the order value is below the minimum notional requirement.'
    }
    if (message.includes('Order would immediately trigger') || message.includes('code=-2021')) {
      return 'Exchange rejected the conditional order because it would trigger immediately with the current parameters.'
    }
    if (message.includes('position side does not match') || message.includes('Position side does not match')) {
      return 'Exchange rejected the order because the position side does not match the current hedge mode or position direction.'
    }
    return 'The exchange rejected the limit order.'
  }

  if (message.includes('position value') && message.includes('exceeds safety limit')) {
    const matched = message.match(/position value \$([0-9.]+) exceeds safety limit \$([0-9.]+)/)
    if (matched) {
      return `Order rejected because the position value would reach $${matched[1]}, above the safety limit of $${matched[2]}.`
    }
    return 'Order rejected because the position value exceeds the configured safety limit.'
  }

  if (message.includes('Trader stopped')) {
    return 'Trader stopped before the remaining actions could be executed.'
  }

  if (message.includes('failed to cancel order')) {
    return 'The exchange did not accept the cancel request for this order.'
  }

  if (message.includes('failed to cancel all orders')) {
    return 'The exchange did not accept the request to cancel all working orders.'
  }

  if (message.includes('failed to get market price')) {
    return 'The system could not fetch the latest market price in time, so this action was skipped.'
  }

  if (message.includes('HTTP 401') || message.includes('unauthorized') || message.includes('Unauthorized')) {
    return 'Request failed because the current session is no longer authorized. Sign in again and retry.'
  }

  if (message.includes('HTTP 403') || message.includes('forbidden') || message.includes('Forbidden')) {
    return 'Request failed because this account does not have permission to perform that action.'
  }

  if (message.includes('HTTP 404') || message.includes('not found') || message.includes('Not Found')) {
    return 'Requested resource was not found. It may have been removed, or the current state is already outdated.'
  }

  if (message.includes('HTTP 429') || message.includes('Too Many Requests')) {
    return 'Request was rate-limited. Too many actions were sent in a short time. Please wait and retry.'
  }

  if (message.includes('timeout') || message.includes('context deadline exceeded')) {
    return 'The request timed out before the exchange or backend returned a result.'
  }

  if (message.includes('network') || message.includes('connection refused') || message.includes('EOF')) {
    return 'The request failed because the backend or exchange connection was interrupted.'
  }

  return null
}

function formatExecutionMessage(message: string | undefined, language: Language): string {
  if (!message) return ''

  if (language === 'zh') {
    const summary = buildChineseExecutionSummary(message)
    if (summary) {
      return `${summary}\n原始信息: ${message}`
    }
  } else {
    const summary = buildEnglishExecutionSummary(message)
    if (summary) {
      return `${summary}\nRaw message: ${message}`
    }
  }

  return message
}

function getActionLabel(action: string, language: Language): string {
  const zhLabels: Record<string, string> = {
    open_long: '开多',
    open_short: '开空',
    close_long: '平多',
    close_short: '平空',
    place_buy_limit: '限价买入',
    place_sell_limit: '限价卖出',
    cancel_order: '撤单',
    cancel_all_orders: '全部撤单',
    pause_grid: '暂停网格',
    resume_grid: '恢复网格',
    adjust_grid: '调整网格',
    hold: '保持',
    wait: '等待',
  }
  const enLabels: Record<string, string> = {
    open_long: 'OPEN LONG',
    open_short: 'OPEN SHORT',
    close_long: 'CLOSE LONG',
    close_short: 'CLOSE SHORT',
    place_buy_limit: 'BUY LIMIT',
    place_sell_limit: 'SELL LIMIT',
    cancel_order: 'CANCEL',
    cancel_all_orders: 'CANCEL ALL',
    pause_grid: 'PAUSE GRID',
    resume_grid: 'RESUME GRID',
    adjust_grid: 'ADJUST GRID',
    hold: 'HOLD',
    wait: 'WAIT',
  }

  return language === 'zh' ? (zhLabels[action] || action) : (enLabels[action] || action)
}

function getActionHint(action: DecisionAction, language: Language): string | null {
  if (action.action === 'place_buy_limit') {
    return language === 'zh'
      ? '执行层会优先用于平空；若没有可平空仓，则开多。'
      : 'Execution prefers closing an existing short first; otherwise it opens a long.'
  }
  if (action.action === 'place_sell_limit') {
    return language === 'zh'
      ? '执行层会优先用于平多；若没有可平多仓，则开空。'
      : 'Execution prefers closing an existing long first; otherwise it opens a short.'
  }
  return null
}

function hasReduceOnlyPlacementFailure(message: string | undefined): boolean {
  if (!message) return false
  return message.includes('ReduceOnly Order is rejected') ||
    message.includes('ReduceOnly Order Failed') ||
    message.includes('code=-2022') ||
    message.includes('code=-4118') ||
    message.includes('failed to seed paired exit')
}

function hasLiveOrderWaitingState(reasoning: string | undefined): boolean {
  if (!reasoning) return false
  const normalized = reasoning.toLowerCase()
  return normalized.includes('等待价格') ||
    normalized.includes('等待价格波动') ||
    normalized.includes('等待触发') ||
    normalized.includes('已有挂单') ||
    normalized.includes('挂单已布置') ||
    normalized.includes('waiting for price') ||
    normalized.includes('waiting for price movement') ||
    normalized.includes('waiting for trigger') ||
    normalized.includes('orders are already placed') ||
    normalized.includes('orders already in place')
}

function getActionPurpose(action: DecisionAction, language: Language): string {
  const reduceOnlyFailed = hasReduceOnlyPlacementFailure(action.error)
  const hasWaitingReasoning = hasLiveOrderWaitingState(action.reasoning)

  if ((action.action === 'hold' || action.action === 'wait') && reduceOnlyFailed) {
    return language === 'zh'
      ? '目标：当前应有平仓挂单，但交易所补挂失败；等待下一轮重试，不要误判为已有挂单在场。'
      : 'Goal: an exit order should exist, but the exchange rejected the re-seed. Wait for the next retry instead of assuming live orders are already working.'
  }

  if ((action.action === 'hold' || action.action === 'wait') && hasWaitingReasoning) {
    return language === 'zh'
      ? '目标：当前已有真实挂单在场，保持现有仓位与挂单结构，等待价格触达后自动成交。'
      : 'Goal: real working orders are already live. Keep the current position and order structure unchanged and wait for price to reach those orders.'
  }

  const zhPurposes: Record<string, string> = {
    open_long: '目标：建立多头仓位，捕捉上涨行情。',
    open_short: '目标：建立空头仓位，捕捉下跌行情。',
    close_long: '目标：平掉现有多头仓位，落袋收益或收缩风险。',
    close_short: '目标：平掉现有空头仓位，落袋收益或收缩风险。',
    place_buy_limit: '目标：在目标价挂买单，优先平空；若无空仓则作为开多准备。',
    place_sell_limit: '目标：在目标价挂卖单，优先平多；若无多仓则作为开空准备。',
    cancel_order: '目标：撤销当前无效或过期挂单，清理执行路径。',
    cancel_all_orders: '目标：清空当前挂单，重置执行环境。',
    pause_grid: '目标：暂停网格新开仓，先控制风险暴露。',
    resume_grid: '目标：恢复网格挂单与自动执行。',
    adjust_grid: '目标：按新价格区间重排网格层级与挂单结构。',
    hold: '目标：保持现有仓位与挂单结构，等待新信号。',
    wait: '目标：暂不动作，继续观察市场与条件变化。',
  }

  const enPurposes: Record<string, string> = {
    open_long: 'Goal: build long exposure for an upside move.',
    open_short: 'Goal: build short exposure for a downside move.',
    close_long: 'Goal: close existing longs to lock gains or reduce risk.',
    close_short: 'Goal: close existing shorts to lock gains or reduce risk.',
    place_buy_limit: 'Goal: queue a buy limit order, preferring short exit before opening a long.',
    place_sell_limit: 'Goal: queue a sell limit order, preferring long exit before opening a short.',
    cancel_order: 'Goal: remove an outdated or invalid working order.',
    cancel_all_orders: 'Goal: clear all working orders and reset execution.',
    pause_grid: 'Goal: pause new grid entries and reduce risk exposure.',
    resume_grid: 'Goal: resume grid order placement and execution.',
    adjust_grid: 'Goal: rebuild the grid around a new price regime.',
    hold: 'Goal: keep current positions and orders unchanged.',
    wait: 'Goal: stand by and observe until conditions improve.',
  }

  return language === 'zh'
    ? (zhPurposes[action.action] || '目标：执行当前策略动作。')
    : (enPurposes[action.action] || 'Goal: execute the current strategy action.')
}

// Single Action Card Component
function ActionCard({ action, language, onSymbolClick }: { action: DecisionAction; language: Language; onSymbolClick?: (symbol: string) => void }) {
  const config = ACTION_STYLE[action.action] || ACTION_STYLE.wait
  const isLong = action.action.includes('long')
  const isOpen = action.action.includes('open') || action.action === 'place_buy_limit' || action.action === 'place_sell_limit'
  const actionHint = getActionHint(action, language)
  const actionPurpose = getActionPurpose(action, language)

  return (
    <div
      className="rounded-lg p-4 transition-all duration-200 hover:scale-[1.01]"
      style={{
        background: 'linear-gradient(135deg, var(--panel-bg) 0%, color-mix(in srgb, var(--panel-bg-solid) 92%, black) 100%)',
        border: `1px solid ${config.color}33`,
        boxShadow: `var(--shadow-md), inset 0 1px 0 rgba(255, 255, 255, 0.03)`,
      }}
    >
      {/* Header Row */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-3">
          <span className="text-xl">{config.icon}</span>
          <div className="min-w-0">
            <div className="flex items-center gap-3">
              <span
                className="font-mono font-bold text-lg cursor-pointer transition-all duration-200 hover:scale-110"
                style={{ color: 'var(--text-primary)' }}
                onClick={() => onSymbolClick?.(action.symbol)}
                title="Click to view chart"
              >
                {action.symbol.replace('USDT', '')}
              </span>
              <span
                className="px-3 py-1 rounded-full text-xs font-bold uppercase tracking-wider"
                style={{ background: config.bg, color: config.color, border: `1px solid ${config.color}55` }}
              >
                {getActionLabel(action.action, language)}
              </span>
            </div>
            <div className="mt-1 text-xs leading-5" style={{ color: 'var(--text-secondary)' }}>
              {actionPurpose}
            </div>
          </div>
        </div>

        {/* Status Badge */}
        <div className="flex items-center gap-2">
          {action.confidence !== undefined && action.confidence > 0 && (
            <div
              className="px-2 py-1 rounded text-xs font-semibold"
              style={{
                background: `${getConfidenceColor(action.confidence)}22`,
                color: getConfidenceColor(action.confidence)
              }}
            >
              {action.confidence.toFixed(0)}%
            </div>
          )}
          <div
            className="w-2 h-2 rounded-full"
            style={{ background: action.success ? '#0ECB81' : '#F6465D' }}
          />
        </div>
      </div>

      {/* Trading Details Grid */}
      {isOpen && (
        <div className="grid grid-cols-4 gap-3 mt-3 pt-3" style={{ borderTop: '1px solid var(--panel-border)' }}>
          {/* Entry Price */}
          <div className="text-center">
            <div className="text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>
              {t('entryPrice', language)}
            </div>
            <div className="font-mono font-semibold" style={{ color: 'var(--text-primary)' }}>
              {formatPrice(action.price)}
            </div>
          </div>

          {/* Stop Loss */}
          <div className="text-center">
            <div className="text-xs mb-1" style={{ color: '#F6465D' }}>
              {t('stopLoss', language)}
            </div>
            <div className="font-mono font-semibold" style={{ color: '#F6465D' }}>
              {formatPrice(action.stop_loss)}
            </div>
            {action.stop_loss && action.price && (
              <div className="text-xs mt-0.5" style={{ color: 'var(--text-secondary)' }}>
                {calcPctChange(action.price, action.stop_loss, isLong)}
              </div>
            )}
          </div>

          {/* Take Profit */}
          <div className="text-center">
            <div className="text-xs mb-1" style={{ color: '#0ECB81' }}>
              {t('takeProfit', language)}
            </div>
            <div className="font-mono font-semibold" style={{ color: '#0ECB81' }}>
              {formatPrice(action.take_profit)}
            </div>
            {action.take_profit && action.price && (
              <div className="text-xs mt-0.5" style={{ color: 'var(--text-secondary)' }}>
                {calcPctChange(action.price, action.take_profit, isLong)}
              </div>
            )}
          </div>

          {/* Leverage */}
          <div className="text-center">
            <div className="text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>
              {t('leverage', language)}
            </div>
            <div className="font-mono font-semibold" style={{ color: '#F0B90B' }}>
              {action.leverage}x
            </div>
          </div>
        </div>
      )}

      {/* Risk/Reward Ratio for open positions */}
      {isOpen && action.stop_loss && action.take_profit && action.price && (
        <div className="mt-3 pt-3 flex items-center justify-between" style={{ borderTop: '1px solid var(--panel-border)' }}>
          <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>{t('riskReward', language)}</span>
          <div className="flex items-center gap-2">
            {(() => {
              const slDist = Math.abs(action.price - action.stop_loss)
              const tpDist = Math.abs(action.take_profit - action.price)
              const ratio = slDist > 0 ? (tpDist / slDist) : 0
              const ratioColor = ratio >= 3 ? '#0ECB81' : ratio >= 2 ? '#F0B90B' : '#F6465D'
              return (
                <>
                  <div className="flex gap-1">
                    <span style={{ color: '#F6465D' }}>1</span>
                    <span style={{ color: 'var(--text-secondary)' }}>:</span>
                    <span style={{ color: '#0ECB81' }}>{ratio.toFixed(1)}</span>
                  </div>
                  <div
                    className="h-1.5 rounded-full"
                    style={{
                      width: '60px',
                      background: 'var(--panel-border)',
                    }}
                  >
                    <div
                      className="h-full rounded-full transition-all duration-300"
                      style={{
                        width: `${Math.min(ratio / 5 * 100, 100)}%`,
                        background: ratioColor
                      }}
                    />
                  </div>
                </>
              )
            })()}
          </div>
        </div>
      )}

      {/* Reasoning */}
      {action.reasoning && (
        <div className="mt-3 pt-3" style={{ borderTop: '1px solid var(--panel-border)' }}>
          <div
            className="text-xs whitespace-pre-wrap break-words"
            style={{ color: 'var(--text-secondary)' }}
          >
            💡 {action.reasoning}
          </div>
        </div>
      )}

      {actionHint && (
        <div className="mt-3 rounded p-2 text-xs" style={{ background: 'rgba(240, 185, 11, 0.08)', border: '1px solid rgba(240, 185, 11, 0.2)', color: '#F5D67B' }}>
          {actionHint}
        </div>
      )}

      {/* Error Message */}
      {action.error && (
        <div
          className="mt-3 rounded p-2 text-xs"
          style={{
            background: 'rgba(246, 70, 93, 0.1)',
            border: '1px solid rgba(246, 70, 93, 0.3)',
            color: '#F6465D',
          }}
        >
          <div className="whitespace-pre-wrap break-all">
            ❌ {formatExecutionMessage(action.error, language)}
          </div>
        </div>
      )}
    </div>
  )
}

export function DecisionCard({ decision, language, onSymbolClick }: DecisionCardProps) {
  const [showSystemPrompt, setShowSystemPrompt] = useState(false)
  const [showInputPrompt, setShowInputPrompt] = useState(false)
  const [showCoT, setShowCoT] = useState(false)
  const hasActionFailure = Array.isArray(decision.decisions) && decision.decisions.some((action) => !action.success)

  // Copy text to clipboard
  const copyToClipboard = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text)
      alert(`${label} copied!`)
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }

  // Download text as file
  const downloadAsFile = (text: string, filename: string) => {
    const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  }

  return (
    <div
      className="rounded-xl p-5 transition-all duration-300 hover:translate-y-[-2px]"
      style={{
        border: '1px solid var(--panel-border)',
        background: 'linear-gradient(180deg, var(--panel-bg) 0%, color-mix(in srgb, var(--panel-bg-solid) 92%, black) 100%)',
        boxShadow: 'var(--shadow-md)',
      }}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <div
            className="w-10 h-10 rounded-lg flex items-center justify-center"
            style={{ background: 'rgba(240, 185, 11, 0.15)' }}
          >
            <span className="text-xl">🤖</span>
          </div>
          <div>
            <div className="font-bold" style={{ color: 'var(--text-primary)' }}>
              {t('cycle', language)} #{decision.cycle_number}
            </div>
            <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
              {new Date(decision.timestamp).toLocaleString()}
            </div>
          </div>
        </div>
        <div
          className="px-4 py-1.5 rounded-full text-xs font-bold tracking-wider"
          style={
            decision.success
              ? { background: 'rgba(14, 203, 129, 0.15)', color: '#0ECB81', border: '1px solid rgba(14, 203, 129, 0.3)' }
              : { background: 'rgba(246, 70, 93, 0.15)', color: '#F6465D', border: '1px solid rgba(246, 70, 93, 0.3)' }
          }
        >
          {t(decision.success ? 'success' : 'failed', language)}
        </div>
      </div>

      {(decision.error_message || hasActionFailure) && (
        <div
          className="mb-4 rounded-lg p-3 text-sm"
          style={{
            background: 'rgba(246, 70, 93, 0.08)',
            border: '1px solid rgba(246, 70, 93, 0.25)',
            color: '#FCA5A5',
          }}
        >
          <div className="whitespace-pre-wrap break-all">
            {formatExecutionMessage(
              decision.error_message || 'One or more actions failed during execution.',
              language
            )}
          </div>
        </div>
      )}

      {/* Decision Actions - Beautiful Grid */}
      {decision.decisions && decision.decisions.length > 0 && (
        <div className="space-y-3 mb-4">
          {decision.decisions.map((action, index) => (
            <ActionCard key={`${action.symbol}-${index}`} action={action} language={language} onSymbolClick={onSymbolClick} />
          ))}
        </div>
      )}

      {decision.execution_log && decision.execution_log.length > 0 && (
        <div
          className="mb-4 rounded-lg p-3 text-xs font-mono whitespace-pre-wrap"
          style={{
            background: 'var(--panel-bg-solid)',
            border: '1px solid var(--panel-border)',
            color: 'var(--text-disabled)',
          }}
        >
          {decision.execution_log
            .map((line) => formatExecutionMessage(line, language))
            .join('\n')}
        </div>
      )}

      {/* Collapsible Sections */}
      <div className="space-y-2">
        {/* System Prompt */}
        {decision.system_prompt && (
          <div>
            <button
              onClick={() => setShowSystemPrompt(!showSystemPrompt)}
              className="flex items-center gap-2 text-sm transition-colors w-full justify-between p-2 rounded"
              style={{ color: 'var(--text-primary)' }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = 'var(--panel-bg)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = 'transparent'
              }}
            >
              <div className="flex items-center gap-2">
                <span className="text-base">⚙️</span>
                <span className="font-semibold" style={{ color: '#a78bfa' }}>
                  System Prompt
                </span>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    copyToClipboard(decision.system_prompt, 'System Prompt')
                  }}
                className="text-xs px-2.5 py-1 rounded hover:opacity-80 transition-opacity flex items-center gap-1"
                  style={{ background: 'rgba(167, 139, 250, 0.2)', color: '#a78bfa', border: '1px solid rgba(167, 139, 250, 0.3)' }}
                  title="Copy to clipboard"
                >
                  <span>📋</span>
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    downloadAsFile(decision.system_prompt, `system-prompt-cycle-${decision.cycle_number}.txt`)
                  }}
                  className="text-xs px-2.5 py-1 rounded hover:opacity-80 transition-opacity flex items-center gap-1"
                  style={{ background: 'rgba(167, 139, 250, 0.2)', color: '#a78bfa', border: '1px solid rgba(167, 139, 250, 0.3)' }}
                  title="Download as file"
                >
                  <span>💾</span>
                </button>
                <span
                  className="text-xs px-2 py-0.5 rounded"
                  style={{ background: 'rgba(167, 139, 250, 0.15)', color: '#a78bfa' }}
                >
                  {showSystemPrompt ? t('collapse', language) : t('expand', language)}
                </span>
              </div>
            </button>
            {showSystemPrompt && (
              <div
                className="mt-2 rounded-lg p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
                style={{
                  background: 'var(--panel-bg-solid)',
                  border: '1px solid var(--panel-border)',
                  color: 'var(--text-primary)',
                }}
              >
                {decision.system_prompt}
              </div>
            )}
          </div>
        )}

        {/* User/Input Prompt */}
        {decision.input_prompt && (
          <div>
            <button
              onClick={() => setShowInputPrompt(!showInputPrompt)}
              className="flex items-center gap-2 text-sm transition-colors w-full justify-between p-2 rounded"
              style={{ color: 'var(--text-primary)' }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = 'var(--panel-bg)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = 'transparent'
              }}
            >
              <div className="flex items-center gap-2">
                <span className="text-base">📥</span>
                <span className="font-semibold" style={{ color: '#60a5fa' }}>
                  User Prompt
                </span>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    copyToClipboard(decision.input_prompt, 'User Prompt')
                  }}
                  className="text-xs px-2.5 py-1 rounded hover:opacity-80 transition-opacity flex items-center gap-1"
                  style={{ background: 'rgba(96, 165, 250, 0.2)', color: '#60a5fa', border: '1px solid rgba(96, 165, 250, 0.3)' }}
                  title="Copy to clipboard"
                >
                  <span>📋</span>
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    downloadAsFile(decision.input_prompt, `user-prompt-cycle-${decision.cycle_number}.txt`)
                  }}
                  className="text-xs px-2.5 py-1 rounded hover:opacity-80 transition-opacity flex items-center gap-1"
                  style={{ background: 'rgba(96, 165, 250, 0.2)', color: '#60a5fa', border: '1px solid rgba(96, 165, 250, 0.3)' }}
                  title="Download as file"
                >
                  <span>💾</span>
                </button>
                <span
                  className="text-xs px-2 py-0.5 rounded"
                  style={{ background: 'rgba(96, 165, 250, 0.15)', color: '#60a5fa' }}
                >
                  {showInputPrompt ? t('collapse', language) : t('expand', language)}
                </span>
              </div>
            </button>
            {showInputPrompt && (
              <div
                className="mt-2 rounded-lg p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
                style={{
                  background: 'var(--panel-bg-solid)',
                  border: '1px solid var(--panel-border)',
                  color: 'var(--text-primary)',
                }}
              >
                {decision.input_prompt}
              </div>
            )}
          </div>
        )}

        {/* AI Thinking */}
        {decision.cot_trace && (
          <div>
            <button
              onClick={() => setShowCoT(!showCoT)}
              className="flex items-center gap-2 text-sm transition-colors w-full justify-between p-2 rounded"
              style={{ color: 'var(--text-primary)' }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = 'var(--panel-bg)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = 'transparent'
              }}
            >
              <div className="flex items-center gap-2">
                <span className="text-base">🧠</span>
                <span className="font-semibold" style={{ color: '#F0B90B' }}>
                  {t('aiThinking', language)}
                </span>
              </div>
              <span
                className="text-xs px-2 py-0.5 rounded"
                style={{ background: 'rgba(240, 185, 11, 0.15)', color: '#F0B90B' }}
              >
                {showCoT ? t('collapse', language) : t('expand', language)}
              </span>
            </button>
            {showCoT && (
              <div
                className="mt-2 rounded-lg p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
                style={{
                  background: 'var(--panel-bg-solid)',
                  border: '1px solid var(--panel-border)',
                  color: 'var(--text-primary)',
                }}
              >
                {decision.cot_trace}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
