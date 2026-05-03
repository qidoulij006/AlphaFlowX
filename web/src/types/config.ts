export interface AIModel {
  id: string
  name: string
  provider: string
  enabled: boolean
  apiKey?: string
  customApiUrl?: string
  customModelName?: string
}

export interface TelegramConfig {
  token_masked: string    // Masked token like "123456:ABC***XYZ"
  is_bound: boolean       // Whether a user has sent /start
  bound_chat_id?: number  // The bound chat ID (if any)
  model_id?: string       // AI model selected for Telegram replies
}

export interface Exchange {
  id: string                     // UUID (empty for supported exchange templates)
  exchange_type: string          // "binance", "bybit", "okx", "hyperliquid", "aster", "lighter"
  account_name: string           // User-defined account name
  name: string                   // Display name
  type: 'cex' | 'dex'
  enabled: boolean
  proxy_configured?: boolean
  proxy_server_id?: string
  proxy_name?: string
  proxy_exit_ip?: string
  apiKey?: string
  secretKey?: string
  passphrase?: string            // OKX specific
  testnet?: boolean
  // Hyperliquid specific
  hyperliquidWalletAddr?: string
  // Aster specific
  asterUser?: string
  asterSigner?: string
  asterPrivateKey?: string
  // LIGHTER specific
  lighterWalletAddr?: string
  lighterPrivateKey?: string
  lighterApiKeyPrivateKey?: string
  lighterApiKeyIndex?: number
}

export interface CreateExchangeRequest {
  exchange_type: string          // "binance", "bybit", "okx", "hyperliquid", "aster", "lighter"
  account_name: string           // User-defined account name
  enabled: boolean
  proxy_url?: string
  proxy_server_id?: string
  api_key?: string
  secret_key?: string
  passphrase?: string
  testnet?: boolean
  hyperliquid_wallet_addr?: string
  aster_user?: string
  aster_signer?: string
  aster_private_key?: string
  lighter_wallet_addr?: string
  lighter_private_key?: string
  lighter_api_key_private_key?: string
  lighter_api_key_index?: number
}

export interface CreateTraderRequest {
  name: string
  ai_model_id: string
  exchange_id: string
  strategy_id?: string // 策略ID（新版，使用保存的策略配置）
  initial_balance?: number // 可选：创建时由后端自动获取，编辑时可手动更新
  scan_interval_minutes?: number
  is_cross_margin?: boolean
  show_in_competition?: boolean // 是否在竞技场显示
  // 以下字段为向后兼容保留，新版使用策略配置
  btc_eth_leverage?: number
  altcoin_leverage?: number
  trading_symbols?: string
  custom_prompt?: string
  override_base_prompt?: boolean
  system_prompt_template?: string
  use_ai500?: boolean
  use_oi_top?: boolean
}

export interface UpdateModelConfigRequest {
  models: {
    [key: string]: {
      name?: string
      provider?: string
      enabled: boolean
      api_key: string
      custom_api_url?: string
      custom_model_name?: string
    }
  }
}

export interface UpdateExchangeConfigRequest {
  exchanges: {
    [key: string]: {
      enabled: boolean
      api_key: string
      secret_key: string
      proxy_url?: string
      proxy_server_id?: string
      passphrase?: string
      testnet?: boolean
      // Hyperliquid 特定字段
      hyperliquid_wallet_addr?: string
      // Aster 特定字段
      aster_user?: string
      aster_signer?: string
      aster_private_key?: string
      // LIGHTER 特定字段
      lighter_wallet_addr?: string
      lighter_private_key?: string
      lighter_api_key_private_key?: string
      lighter_api_key_index?: number
    }
  }
}

export interface ProxyServer {
  id: string
  name: string
  enabled: boolean
  configured: boolean
  last_test_status?: string
  last_exit_ip?: string
  last_test_at?: string
  bound_exchange_id?: string
  bound_exchange_name?: string
}

export interface AdminOverview {
  admin_mode: boolean
  admin_password_configured: boolean
  db_type: string
  transport_encryption: boolean
  nofxos_enabled: boolean
  cors_origins: number
  registration_enabled: boolean
  users_total: number
  new_users_7d: number
  traders_total: number
  traders_running: number
  models_total: number
  models_enabled: number
  exchanges_total: number
  exchanges_enabled: number
  server_time: string
}

export interface AdminUserSummary {
  id: string
  email: string
  is_admin: boolean
  is_active: boolean
  is_archived: boolean
  created_at: string
  updated_at: string
  session_revoked_at?: string
  trader_count: number
  running_traders: number
  model_count: number
  enabled_models: number
  exchange_count: number
  enabled_exchanges: number
}

export interface AdminAuditLog {
  id: number
  actor_id: string
  actor_email: string
  action: string
  target_type: string
  target_id: string
  summary: string
  created_at: string
}
