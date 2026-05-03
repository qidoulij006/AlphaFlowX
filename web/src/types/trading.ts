export interface SystemStatus {
  trader_id: string
  trader_name: string
  ai_model: string
  is_running: boolean
  start_time: string
  runtime_minutes: number
  call_count: number
  initial_balance: number
  scan_interval: string
  stop_until: string
  last_reset_time: string
  ai_provider: string
  strategy_type?: 'ai_trading' | 'grid_trading'
  grid_symbol?: string
}

export interface AccountInfo {
  total_equity: number
  wallet_balance: number
  unrealized_profit: number // 未实现盈亏（交易所API官方值）
  available_balance: number
  total_pnl: number
  total_pnl_pct: number
  initial_balance: number
  daily_pnl: number
  position_count: number
  margin_used: number
  margin_used_pct: number
}

export interface Position {
  symbol: string
  side: string
  entry_price: number
  mark_price: number
  quantity: number
  leverage: number
  unrealized_pnl: number
  unrealized_pnl_pct: number
  liquidation_price: number
  margin_used: number
}

export interface DecisionAction {
  action: string
  symbol: string
  quantity: number
  leverage: number
  price: number
  stop_loss?: number      // Stop loss price
  take_profit?: number    // Take profit price
  confidence?: number     // AI confidence (0-100)
  reasoning?: string      // Brief reasoning
  order_id: number
  timestamp: string
  success: boolean
  error?: string
}

export interface AccountSnapshot {
  total_balance: number
  available_balance: number
  total_unrealized_profit: number
  position_count: number
  margin_used_pct: number
}

export interface DecisionRecord {
  timestamp: string
  cycle_number: number
  system_prompt: string
  input_prompt: string
  cot_trace: string
  decision_json: string
  account_state: AccountSnapshot
  positions: any[]
  candidate_coins: string[]
  decisions: DecisionAction[]
  execution_log: string[]
  success: boolean
  error_message?: string
}

export interface Statistics {
  total_cycles: number
  successful_cycles: number
  failed_cycles: number
  total_open_positions: number
  total_close_positions: number
}

// AI Trading相关类型
export interface TraderInfo {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange_id?: string
  created_at?: string
  is_running?: boolean
  show_in_competition?: boolean
  strategy_id?: string
  strategy_name?: string
  custom_prompt?: string
  use_ai500?: boolean
  use_oi_top?: boolean
  system_prompt_template?: string
  error_hint_code?: string
  error_hint?: string
  error_message?: string
}

// Competition related types
export interface CompetitionTraderData {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange: string
  total_equity: number
  total_pnl: number
  total_pnl_pct: number
  position_count: number
  margin_used_pct: number
  is_running: boolean
}

export interface CompetitionData {
  traders: CompetitionTraderData[]
  count: number
}

// Trader Configuration Data for View Modal
export interface TraderConfigData {
  trader_id?: string
  trader_name: string
  ai_model: string
  exchange_id: string
  strategy_id?: string  // 策略ID
  strategy_name?: string  // 策略名称
  is_cross_margin: boolean
  show_in_competition: boolean  // 是否在竞技场显示
  scan_interval_minutes: number
  initial_balance: number
  is_running: boolean
  // 以下为旧版字段（向后兼容）
  btc_eth_leverage?: number
  altcoin_leverage?: number
  trading_symbols?: string
  custom_prompt?: string
  override_base_prompt?: boolean
  system_prompt_template?: string
  use_ai500?: boolean
  use_oi_top?: boolean
}

// Position History Types
export interface HistoricalPosition {
  id: number
  trader_id: string
  exchange_id: string
  exchange_type: string
  symbol: string
  side: string
  quantity: number
  entry_quantity: number
  entry_price: number
  entry_order_id: string
  entry_time: string
  exit_price: number
  exit_order_id: string
  exit_time: string
  realized_pnl: number
  fee: number
  leverage: number
  status: string
  close_reason: string
  created_at: string
  updated_at: string
}

// Matches Go TraderStats struct exactly
export interface TraderStats {
  total_trades: number
  win_trades: number
  loss_trades: number
  win_rate: number
  profit_factor: number
  sharpe_ratio: number
  total_pnl: number
  total_fee: number
  avg_win: number
  avg_loss: number
  max_drawdown_pct: number
}

// Matches Go SymbolStats struct exactly
export interface SymbolStats {
  symbol: string
  total_trades: number
  win_trades: number
  win_rate: number
  total_pnl: number
  avg_pnl: number
  avg_hold_mins: number
}

// Matches Go DirectionStats struct exactly
export interface DirectionStats {
  side: string
  trade_count: number
  win_rate: number
  total_pnl: number
  avg_pnl: number
}

export interface PositionHistoryResponse {
  positions: HistoricalPosition[]
  stats: TraderStats | null
  symbol_stats: SymbolStats[]
  direction_stats: DirectionStats[]
}

export interface RecentFilledOrder {
  id: string
  symbol: string
  side: string
  position_side: string
  reduce_only: boolean
  quantity: number
  filled_quantity: number
  avg_fill_price: number
  quote_quantity: number
  is_maker: boolean
  created_at: string
  filled_at: string
}


// Grid Risk Information for frontend display
export interface GridRiskInfo {
  // Leverage info
  current_leverage: number
  effective_leverage: number
  recommended_leverage: number
  compounding_base: number

  // Position info
  current_position: number
  max_position: number
  position_percent: number
  current_long_position: number
  current_short_position: number
  current_long_value: number
  current_short_value: number
  entry_order_count: number
  reduce_only_order_count: number
  long_order_count: number
  short_order_count: number
  long_filled_levels: number
  short_filled_levels: number
  average_order_notional: number

  // Liquidation info
  liquidation_price: number
  liquidation_distance: number

  // Market state
  regime_level: string

  // Box state
  short_box_upper: number
  short_box_lower: number
  mid_box_upper: number
  mid_box_lower: number
  long_box_upper: number
  long_box_lower: number
  current_price: number
  grid_upper_price: number
  grid_lower_price: number
  grid_boundary_source: string
  grid_spacing: number
  min_grid_spacing: number
  max_grid_spacing: number

  // Breakout state
  breakout_level: string
  breakout_direction: string

  first_entry_guard_threshold_pct: number
  first_entry_guard_atr_multiplier: number
  first_entry_guard_fixed_band: number
  first_entry_guard_atr_band: number
  first_entry_guard_active_source: string
  short_first_entry_guard_price: number
  long_first_entry_guard_price: number
  effective_atr14: number
  long_reference_stop_price: number
  short_reference_stop_price: number

  current_grid_direction: string
  direction_change_count: number
  enable_direction_adjust: boolean

  current_risk_state: string
  previous_risk_state: string
  current_risk_reason: string
  current_risk_side: string
  risk_state_changed_at: number
  stop_loss_trigger_price: number
  stop_loss_execution_price: number
  stop_loss_entry_price: number
  stop_loss_position_qty: number
  stop_loss_reduced_qty: number
  stop_loss_remaining_qty: number
  stop_loss_position_value: number
  stop_loss_reduced_value: number
  stop_loss_unrealized_loss: number
  stop_loss_unrealized_loss_pct: number
  stop_loss_unrealized_loss_equity_pct: number
  risk_history: GridRiskHistoryItem[]

  grid_levels: GridRiskLevel[]
  grid_orders: GridRiskOrder[]
}

export interface GridRiskLevel {
  index: number
  price: number
}

export interface GridRiskOrder {
  order_id: string
  price: number
  quantity: number
  notional: number
  side: string
  position_side: string
  reduce_only: boolean
  level_index: number
  linked_level_index: number
  source_entry_price: number
  bucket: 'long_entry' | 'short_entry' | 'long_exit' | 'short_exit' | string
  status: string
}

export interface GridRiskHistoryItem {
  id: string
  risk_state: string
  previous_risk_state: string
  position_side: string
  event_type: string
  reason: string
  trigger_price: number
  execution_price: number
  entry_price: number
  position_qty: number
  reduced_qty: number
  remaining_qty: number
  position_notional: number
  reduced_notional: number
  unrealized_loss: number
  unrealized_loss_pct: number
  unrealized_loss_equity_pct: number
  position_percent: number
  effective_leverage: number
  liquidation_price: number
  liquidation_distance: number
  created_at: number
  metadata?: Record<string, unknown>
}
