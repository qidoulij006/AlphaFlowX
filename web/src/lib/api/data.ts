import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  CompetitionData,
  PositionHistoryResponse,
  RecentFilledOrder,
} from '../../types'
import { API_BASE, httpClient } from './helpers'

const toNumber = (value: unknown): number => {
  if (typeof value === 'number') return Number.isFinite(value) ? value : 0
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : 0
  }
  return 0
}

const normalizePosition = (raw: any): Position => {
  const quantity = Math.abs(toNumber(raw.quantity ?? raw.positionAmt ?? raw.position_amt))
  const entryPrice = toNumber(raw.entry_price ?? raw.entryPrice)
  const markPrice = toNumber(raw.mark_price ?? raw.markPrice)
  const unrealizedPnl = toNumber(raw.unrealized_pnl ?? raw.unRealizedProfit ?? raw.unrealizedProfit)
  const liquidationPrice = toNumber(raw.liquidation_price ?? raw.liquidationPrice)
  const leverage = toNumber(raw.leverage)
  const sideRaw = String(raw.side ?? '').toLowerCase()
  const derivedSide = quantity > 0 && toNumber(raw.positionAmt ?? raw.position_amt ?? raw.quantity) < 0 ? 'short' : 'long'
  const side = sideRaw === 'long' || sideRaw === 'short' ? sideRaw : derivedSide
  const unrealizedPnlPct = toNumber(raw.unrealized_pnl_pct ?? raw.unrealizedPnlPct)
  const marginUsed = toNumber(raw.margin_used ?? raw.marginUsed)

  return {
    symbol: String(raw.symbol ?? ''),
    side,
    entry_price: entryPrice,
    mark_price: markPrice,
    quantity,
    leverage,
    unrealized_pnl: unrealizedPnl,
    unrealized_pnl_pct: unrealizedPnlPct,
    liquidation_price: liquidationPrice,
    margin_used: marginUsed,
  }
}

const toIsoString = (value: unknown): string => {
  if (typeof value === 'string') return value
  if (typeof value === 'number' && Number.isFinite(value) && value > 0) {
    return new Date(value).toISOString()
  }
  return ''
}

const normalizeFilledOrder = (raw: any): RecentFilledOrder => ({
  id: String(raw.id ?? raw.ID ?? raw.order_id ?? raw.exchange_order_id ?? ''),
  symbol: String(raw.symbol ?? raw.Symbol ?? ''),
  side: String(raw.side ?? raw.Side ?? ''),
  position_side: String(raw.position_side ?? raw.PositionSide ?? ''),
  reduce_only: Boolean(raw.reduce_only ?? raw.ReduceOnly),
  quantity: toNumber(raw.quantity ?? raw.Quantity),
  filled_quantity: toNumber(raw.filled_quantity ?? raw.FilledQuantity ?? raw.executed_quantity ?? raw.executedQty),
  avg_fill_price: toNumber(raw.avg_fill_price ?? raw.AvgFillPrice ?? raw.price ?? raw.Price),
  quote_quantity: toNumber(raw.quote_quantity ?? raw.QuoteQuantity),
  is_maker: Boolean(raw.is_maker ?? raw.IsMaker),
  created_at: toIsoString(raw.created_at ?? raw.CreatedAt),
  filled_at: toIsoString(raw.filled_at ?? raw.FilledAt),
})


export const dataApi = {
  async getSharedTrader(token: string): Promise<any> {
    const result = await httpClient.get<any>(`${API_BASE}/shared/trader?token=${encodeURIComponent(token)}`)
    if (!result.success) throw new Error('Failed to fetch shared trader')
    return result.data!
  },

  async getStatus(traderId?: string): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`
    const result = await httpClient.get<SystemStatus>(url)
    if (!result.success) throw new Error('Failed to fetch system status')
    return result.data!
  },

  async getAccount(traderId?: string): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`
    const result = await httpClient.get<AccountInfo>(
      url,
      undefined,
      undefined,
      { suppressSystemErrors: true }
    )
    if (!result.success) {
      throw new Error(result.message || 'Failed to fetch account info')
    }
    return result.data!
  },

  async getPositions(traderId?: string): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`
    const result = await httpClient.get<Position[]>(
      url,
      undefined,
      undefined,
      { suppressSystemErrors: true }
    )
    if (!result.success) {
      throw new Error(result.message || 'Failed to fetch positions')
    }
    return Array.isArray(result.data) ? result.data.map(normalizePosition) : []
  },

  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`
    const result = await httpClient.get<DecisionRecord[]>(url)
    if (!result.success) throw new Error('Failed to fetch decision logs')
    return result.data!
  },

  async getLatestDecisions(
    traderId?: string,
    limit: number = 5
  ): Promise<DecisionRecord[]> {
    const params = new URLSearchParams()
    if (traderId) {
      params.append('trader_id', traderId)
    }
    params.append('limit', limit.toString())

    const result = await httpClient.get<DecisionRecord[]>(
      `${API_BASE}/decisions/latest?${params}`
    )
    if (!result.success) throw new Error('Failed to fetch latest decisions')
    return result.data!
  },

  async getStatistics(traderId?: string): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`
    const result = await httpClient.get<Statistics>(url)
    if (!result.success) throw new Error('Failed to fetch statistics')
    return result.data!
  },

  async getEquityHistory(traderId?: string): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`
    const result = await httpClient.get<any[]>(url)
    if (!result.success) throw new Error('Failed to fetch equity history')
    return result.data!
  },

  async getEquityHistoryBatch(traderIds: string[], hours?: number): Promise<any> {
    const result = await httpClient.post<any>(
      `${API_BASE}/equity-history-batch`,
      { trader_ids: traderIds, hours: hours || 0 }
    )
    if (!result.success) throw new Error('Failed to fetch batch equity history')
    return result.data!
  },

  async getTopTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/top-traders`)
    if (!result.success) throw new Error('Failed to fetch top traders')
    return result.data!
  },

  async getPublicTraderConfig(traderId: string): Promise<any> {
    const result = await httpClient.get<any>(
      `${API_BASE}/trader/${traderId}/config`
    )
    if (!result.success) throw new Error('Failed to fetch public trader config')
    return result.data!
  },

  async getCompetition(): Promise<CompetitionData> {
    const result = await httpClient.get<CompetitionData>(
      `${API_BASE}/competition`
    )
    if (!result.success) throw new Error('Failed to fetch competition data')
    return result.data!
  },

  async getPositionHistory(traderId: string, limit: number = 100): Promise<PositionHistoryResponse> {
    const result = await httpClient.get<PositionHistoryResponse>(
      `${API_BASE}/positions/history?trader_id=${traderId}&limit=${limit}`
    )
    if (!result.success) throw new Error('Failed to fetch position history')
    return result.data!
  },

  async getRecentFilledOrders(traderId: string, symbol?: string, limit: number = 50): Promise<RecentFilledOrder[]> {
    const query = new URLSearchParams({
      trader_id: traderId,
      limit: String(limit),
    })
    if (symbol) query.set('symbol', symbol)
    const result = await httpClient.get<any[]>(
      `${API_BASE}/recent-fills?${query.toString()}`
    )
    if (!result.success) throw new Error('Failed to fetch recent filled orders')
    return Array.isArray(result.data) ? result.data.map(normalizeFilledOrder) : []
  },


  async getSharedStatus(token: string): Promise<SystemStatus> {
    const result = await httpClient.get<SystemStatus>(`${API_BASE}/shared/status?token=${encodeURIComponent(token)}`)
    if (!result.success) throw new Error('Failed to fetch shared system status')
    return result.data!
  },

  async getSharedAccount(token: string): Promise<AccountInfo> {
    const result = await httpClient.get<AccountInfo>(
      `${API_BASE}/shared/account?token=${encodeURIComponent(token)}`,
      undefined,
      undefined,
      { suppressSystemErrors: true }
    )
    if (!result.success) throw new Error(result.message || 'Failed to fetch shared account info')
    return result.data!
  },

  async getSharedPositions(token: string): Promise<Position[]> {
    const result = await httpClient.get<Position[]>(
      `${API_BASE}/shared/positions?token=${encodeURIComponent(token)}`,
      undefined,
      undefined,
      { suppressSystemErrors: true }
    )
    if (!result.success) throw new Error(result.message || 'Failed to fetch shared positions')
    return Array.isArray(result.data) ? result.data.map(normalizePosition) : []
  },

  async getSharedLatestDecisions(token: string, limit: number = 5): Promise<DecisionRecord[]> {
    const result = await httpClient.get<DecisionRecord[]>(
      `${API_BASE}/shared/decisions/latest?token=${encodeURIComponent(token)}&limit=${limit}`
    )
    if (!result.success) throw new Error('Failed to fetch shared latest decisions')
    return result.data!
  },

  async getSharedStatistics(token: string): Promise<Statistics> {
    const result = await httpClient.get<Statistics>(`${API_BASE}/shared/statistics?token=${encodeURIComponent(token)}`)
    if (!result.success) throw new Error('Failed to fetch shared statistics')
    return result.data!
  },

  async getSharedEquityHistory(token: string): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/shared/equity-history?token=${encodeURIComponent(token)}`)
    if (!result.success) throw new Error('Failed to fetch shared equity history')
    return result.data!
  },

  async getSharedPositionHistory(token: string, limit: number = 100): Promise<PositionHistoryResponse> {
    const result = await httpClient.get<PositionHistoryResponse>(
      `${API_BASE}/shared/positions/history?token=${encodeURIComponent(token)}&limit=${limit}`
    )
    if (!result.success) throw new Error('Failed to fetch shared position history')
    return result.data!
  },
}
