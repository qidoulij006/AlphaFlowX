import { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import useSWR from 'swr'
import {
  TrendingUp,
  Shield,
  Zap,
  Eye,
  EyeOff,
  Copy,
  Check,
  Hexagon,
  Layers,
  Target,
  Activity,
  Terminal,
  Cpu,
  Database
} from 'lucide-react'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
import { toast } from 'sonner'
import { t } from '../i18n/translations'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'

interface PublicStrategy {
  id: string
  name: string
  description: string
  author_email?: string
  is_public: boolean
  config_visible: boolean
  config?: any
  stats?: {
    used_by: number
    rating: number
  }
  created_at: string
  updated_at: string
}

const strategyStyles: Record<string, { color: string; border: string; glow: string; shadow: string; icon: any; bg: string }> = {
  scalper: {
    color: 'text-[#F0B90B]',
    border: 'border-[#F0B90B]/30',
    glow: 'shadow-[0_0_20px_rgba(240,185,11,0.15)]',
    shadow: 'hover:shadow-[0_0_30px_rgba(240,185,11,0.25)]',
    bg: 'bg-[#F0B90B]/5',
    icon: Zap
  },
  swing: {
    color: 'text-cyan-400',
    border: 'border-cyan-400/30',
    glow: 'shadow-[0_0_20px_rgba(34,211,238,0.15)]',
    shadow: 'hover:shadow-[0_0_30px_rgba(34,211,238,0.25)]',
    bg: 'bg-cyan-400/5',
    icon: TrendingUp
  },
  arbitrage: {
    color: 'text-purple-400',
    border: 'border-purple-400/30',
    glow: 'shadow-[0_0_20px_rgba(192,132,252,0.15)]',
    shadow: 'hover:shadow-[0_0_30px_rgba(192,132,252,0.25)]',
    bg: 'bg-purple-400/5',
    icon: Layers
  },
  conservative: {
    color: 'text-emerald-400',
    border: 'border-emerald-400/30',
    glow: 'shadow-[0_0_20px_rgba(52,211,153,0.15)]',
    shadow: 'hover:shadow-[0_0_30px_rgba(52,211,153,0.25)]',
    bg: 'bg-emerald-400/5',
    icon: Shield
  },
  aggressive: {
    color: 'text-red-500',
    border: 'border-red-500/30',
    glow: 'shadow-[0_0_20px_rgba(239,68,68,0.15)]',
    shadow: 'hover:shadow-[0_0_30px_rgba(239,68,68,0.25)]',
    bg: 'bg-red-500/5',
    icon: Target
  },
  default: {
    color: 'text-[var(--text-secondary)]',
    border: 'border-[var(--panel-border)]',
    glow: '',
    shadow: 'hover:shadow-[0_0_20px_rgba(255,255,255,0.05)]',
    bg: 'bg-[var(--panel-bg)]',
    icon: Activity
  }
}

function getStrategyStyle(name: string) {
  const lowerName = name.toLowerCase()
  if (lowerName.includes('scalp')) return strategyStyles.scalper
  if (lowerName.includes('swing')) return strategyStyles.swing
  if (lowerName.includes('arb')) return strategyStyles.arbitrage
  if (lowerName.includes('safe') || lowerName.includes('conserv')) return strategyStyles.conservative
  if (lowerName.includes('aggress') || lowerName.includes('high')) return strategyStyles.aggressive
  return strategyStyles.default
}

export function StrategyMarketPage() {
  const { language } = useLanguage()
  const { token, user } = useAuth()
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedCategory, setSelectedCategory] = useState<string>('all')
  const [copiedId, setCopiedId] = useState<string | null>(null)

  const tr = (key: string) => t(`strategyMarket.${key}`, language)

  // Fetch public strategies
  const { data: strategies, isLoading } = useSWR<PublicStrategy[]>(
    'public-strategies',
    async () => {
      const response = await fetch('/api/strategies/public')
      if (!response.ok) throw new Error('Failed to fetch strategies')
      const data = await response.json()
      return data.strategies || []
    },
    {
      refreshInterval: 60000,
      revalidateOnFocus: false
    }
  )

  const filteredStrategies = strategies?.filter(s => {
    if (searchQuery) {
      const query = searchQuery.toLowerCase()
      return s.name.toLowerCase().includes(query) ||
        s.description?.toLowerCase().includes(query)
    }
    return true
  }) || []

  const handleCopyConfig = async (strategy: PublicStrategy) => {
    if (!strategy.config) return
    try {
      await navigator.clipboard.writeText(JSON.stringify(strategy.config, null, 2))
      setCopiedId(strategy.id)
      toast.success(tr('copied'))
      setTimeout(() => setCopiedId(null), 2000)
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr)
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false
    }).replace(',', '')
  }

  const getIndicatorList = (config: any) => {
    if (!config?.indicators) return []
    const indicators = []
    if (config.indicators.enable_ema) indicators.push('EMA')
    if (config.indicators.enable_macd) indicators.push('MACD')
    if (config.indicators.enable_rsi) indicators.push('RSI')
    if (config.indicators.enable_atr) indicators.push('ATR')
    if (config.indicators.enable_boll) indicators.push('BOLL')
    if (config.indicators.enable_volume) indicators.push('VOL')
    if (config.indicators.enable_oi) indicators.push('OI')
    if (config.indicators.enable_funding_rate) indicators.push('FR')
    return indicators
  }

  const isGridStrategy = (config: any) => config?.strategy_type === 'grid_trading' && config?.grid_config

  const formatGridBaseLabel = (config: any) => {
    const base = config?.grid_config?.total_investment
    if (typeof base !== 'number' || Number.isNaN(base) || base <= 0) return '-'
    return `${base.toLocaleString('en-US', { maximumFractionDigits: 0 })} USDT`
  }

  return (
    <DeepVoidBackground className="min-h-screen text-nofx-text font-mono py-12">
      <div className="w-full px-4 md:px-8 space-y-8">

        <div className="w-full relative z-10">

          {/* Header Section */}
          <div className="mb-12 pb-8 relative" style={{ borderBottom: '1px solid var(--panel-border)' }}>
            <div className="absolute top-0 right-0 p-2 rounded text-xs font-mono hidden md:block" style={{ border: '1px solid var(--panel-border)', background: 'var(--panel-bg)', color: 'var(--text-secondary)' }}>
              SYSTEM_STATUS: <span className="text-emerald-500 animate-pulse">ONLINE</span>
              <br />
              MARKET_UPLINK: <span className="text-emerald-500">ESTABLISHED</span>
            </div>

            <div className="flex items-center gap-4 mb-4">
              <div className="p-3 rounded-none relative group overflow-hidden" style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}>
                <div className="absolute inset-0 bg-nofx-gold/20 opacity-0 group-hover:opacity-100 transition-opacity"></div>
                <Database className="w-8 h-8 text-nofx-gold relative z-10" />
              </div>
              <div>
                <h1 className="text-4xl font-bold tracking-tighter uppercase" data-text={tr('title')} style={{ color: 'var(--text-primary)' }}>
                  {tr('title')}
                </h1>
                <p className="text-xs text-nofx-gold tracking-[0.3em] font-bold mt-1">
                // {tr('subtitle')}
                </p>
              </div>
            </div>
            <p className="text-sm max-w-2xl pl-4" style={{ color: 'var(--text-secondary)', borderLeft: '2px solid var(--panel-border)' }}>
              {tr('description')}
            </p>
          </div>

          {/* Search and Filter Bar */}
          <div className="flex flex-col md:flex-row gap-4 mb-8">
            {/* Search */}
            <div className="relative flex-1 group">
              <div
                className="absolute -inset-0.5 rounded opacity-0 group-hover:opacity-100 transition duration-500 blur"
                style={{ background: 'linear-gradient(90deg, rgba(214, 144, 7, 0.18), rgba(180, 142, 92, 0.08))' }}
              ></div>
              <div className="relative flex items-center transition-colors" style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}>
                <div className="pl-4 pr-3 transition-colors" style={{ color: 'var(--text-secondary)' }}>
                  <Terminal size={16} />
                </div>
                <input
                  type="text"
                  placeholder={tr('search')}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="w-full bg-transparent py-3 text-sm focus:outline-none font-mono"
                  style={{ color: 'var(--text-primary)' }}
                />
                <div className="pr-4">
                  <div className="w-2 h-4 bg-nofx-gold animate-pulse"></div>
                </div>
              </div>
            </div>

            {/* Category Filter */}
            <div className="flex gap-2 p-1" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}>
              {['all', 'popular', 'recent'].map((cat) => (
                <button
                  key={cat}
                  onClick={() => setSelectedCategory(cat)}
                  className={`px-4 py-2 text-xs font-mono uppercase tracking-wider transition-all relative overflow-hidden ${selectedCategory === cat
                    ? 'text-black font-bold'
                    : ''
                    }`}
                  style={selectedCategory === cat ? undefined : { color: 'var(--text-secondary)' }}
                  onMouseEnter={(e) => {
                    if (selectedCategory !== cat) {
                      e.currentTarget.style.background = 'var(--panel-bg-hover)'
                      e.currentTarget.style.color = 'var(--text-primary)'
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (selectedCategory !== cat) {
                      e.currentTarget.style.background = 'transparent'
                      e.currentTarget.style.color = 'var(--text-secondary)'
                    }
                  }}
                >
                  {selectedCategory === cat && (
                    <motion.div
                      layoutId="filter-highlight"
                      className="absolute inset-0 bg-nofx-gold"
                      transition={{ type: "spring", bounce: 0.2, duration: 0.6 }}
                    />
                  )}
                  <span className="relative z-10">{tr(cat)}</span>
                </button>
              ))}
            </div>
          </div>

          {/* Loading State */}
          {isLoading && (
            <div className="flex flex-col items-center justify-center py-32 space-y-4">
              <div className="relative w-16 h-16">
                <div className="absolute inset-0 border-2 rounded-full" style={{ borderColor: 'var(--panel-border)' }}></div>
                <div className="absolute inset-0 border-2 border-nofx-gold rounded-full border-t-transparent animate-spin"></div>
                <div className="absolute inset-0 flex items-center justify-center">
                  <Cpu size={24} className="text-nofx-gold/50" />
                </div>
              </div>
              <p className="text-nofx-gold text-xs tracking-widest animate-pulse">{tr('loading')}</p>
              <div className="flex gap-1">
                <div className="w-1 h-1 bg-nofx-gold rounded-full animate-bounce" style={{ animationDelay: '0s' }}></div>
                <div className="w-1 h-1 bg-nofx-gold rounded-full animate-bounce" style={{ animationDelay: '0.2s' }}></div>
                <div className="w-1 h-1 bg-nofx-gold rounded-full animate-bounce" style={{ animationDelay: '0.4s' }}></div>
              </div>
            </div>
          )}

          {/* Empty State */}
          {!isLoading && filteredStrategies.length === 0 && (
            <div className="flex flex-col items-center justify-center py-32 border border-dashed rounded" style={{ borderColor: 'var(--panel-border)', background: 'var(--panel-bg)' }}>
              <div className="relative mb-6">
                <div className="absolute -inset-4 bg-red-500/10 rounded-full blur-xl animate-pulse"></div>
                <Activity className="w-16 h-16 relative z-10" style={{ color: 'var(--text-secondary)' }} />
              </div>
              <h3 className="text-xl font-bold font-mono tracking-tight mb-2" style={{ color: 'var(--text-primary)' }}>
                [{tr('noStrategies')}]
              </h3>
              <p className="text-xs tracking-wide uppercase" style={{ color: 'var(--text-secondary)' }}>{tr('noStrategiesDesc')}</p>
            </div>
          )}

          {/* Strategy Grid */}
          {!isLoading && filteredStrategies.length > 0 && (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              <AnimatePresence>
                {filteredStrategies.map((strategy, i) => {
                  const style = getStrategyStyle(strategy.name)
                  const Icon = style.icon
                  const indicators = strategy.config_visible && strategy.config
                    ? getIndicatorList(strategy.config)
                    : []

                  return (
                    <motion.div
                      key={strategy.id}
                      initial={{ opacity: 0, scale: 0.95 }}
                      animate={{ opacity: 1, scale: 1 }}
                      exit={{ opacity: 0, scale: 0.95 }}
                      transition={{ delay: i * 0.05 }}
                      className={`group relative transition-all duration-300 ${style.shadow}`}
                      style={{ background: 'linear-gradient(135deg, var(--panel-bg-solid) 0%, var(--panel-bg) 100%)', border: '1px solid var(--panel-border)' }}
                      onMouseEnter={(e) => {
                        e.currentTarget.style.borderColor = 'rgba(214, 144, 7, 0.22)'
                        e.currentTarget.style.transform = 'translateY(-2px)'
                      }}
                      onMouseLeave={(e) => {
                        e.currentTarget.style.borderColor = 'var(--panel-border)'
                        e.currentTarget.style.transform = 'translateY(0)'
                      }}
                    >
                      {/* Holographic Border Highlight */}
                      <div className={`absolute top-0 left-0 w-full h-[1px] bg-gradient-to-r from-transparent via-${style.color.split('-')[1]}-500 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500`}></div>
                      <div className={`absolute bottom-0 right-0 w-full h-[1px] bg-gradient-to-r from-transparent via-${style.color.split('-')[1]}-500 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500`}></div>

                      {/* Category Side Strip */}
                      <div className={`absolute left-0 top-0 bottom-0 w-[2px] ${style.bg.replace('/5', '/50')}`}></div>

                      <div className="p-6 relative">
                        {/* Header */}
                        <div className="flex justify-between items-start mb-6">
                          <div className={`p-2 rounded-none border ${style.border} ${style.bg}`}>
                            <Icon className={`w-5 h-5 ${style.color}`} />
                          </div>
                          <div className="text-[10px] font-mono">
                            {strategy.config_visible ? (
                              <div className="flex items-center gap-1.5 text-emerald-500 border border-emerald-500/20 bg-emerald-500/10 px-2 py-1">
                                <Eye size={10} />
                                PUBLIC_ACCESS
                              </div>
                            ) : (
                              <div className="flex items-center gap-1.5 px-2 py-1" style={{ color: 'var(--text-secondary)', border: '1px solid var(--panel-border)', background: 'var(--panel-bg)' }}>
                                <EyeOff size={10} />
                                RESTRICTED
                              </div>
                            )}
                          </div>
                        </div>

                        {/* Name and Description */}
                        <h3 className={`text-lg font-bold mb-2 tracking-tight transition-colors uppercase truncate relative`} style={{ color: 'var(--text-primary)' }}>
                          {strategy.name}
                          <span className="absolute -bottom-1 left-0 w-8 h-[2px] transition-colors" style={{ background: 'var(--panel-border)' }}></span>
                        </h3>
                        <p className="text-xs mb-6 line-clamp-2 h-8 leading-relaxed font-sans" style={{ color: 'var(--text-secondary)' }}>
                          {strategy.description || 'NO_DESCRIPTION_AVAILABLE'}
                        </p>

                        {/* Meta Data */}
                        <div className="grid grid-cols-2 gap-y-2 mb-6 text-[10px] font-mono" style={{ color: 'var(--text-secondary)' }}>
                          <div className="flex flex-col">
                            <span className="uppercase" style={{ color: 'var(--text-tertiary)' }}>{tr('author')}</span>
                            <span style={{ color: 'var(--text-primary)' }}>@{strategy.author_email?.split('@')[0] || 'UNKNOWN'}</span>
                          </div>
                          <div className="flex flex-col text-right">
                            <span className="uppercase" style={{ color: 'var(--text-tertiary)' }}>{tr('createdAt')}</span>
                            <span style={{ color: 'var(--text-primary)' }}>{formatDate(strategy.created_at)}</span>
                          </div>
                        </div>

                        {/* Config / Indicators */}
                        <div className="p-3 mb-4 backdrop-blur-sm min-h-[90px]" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}>
                          {strategy.config_visible && strategy.config ? (
                            <div className="space-y-3">
                              {/* Indicators */}
                              <div className="flex items-center gap-2 overflow-x-auto scrollbar-hide pb-1">
                                {indicators.length > 0 ? indicators.map((ind) => (
                                  <span
                                    key={ind}
                                    className="px-1.5 py-0.5 text-[9px] font-mono whitespace-nowrap"
                                    style={{ border: '1px solid var(--panel-border)', background: 'var(--panel-bg-solid)', color: 'var(--text-primary)' }}
                                  >
                                    {ind}
                                  </span>
                                )) : <span className="text-[9px]" style={{ color: 'var(--text-secondary)' }}>NO_INDICATORS</span>}
                              </div>

                              {/* Risk Control */}
                              {strategy.config.risk_control && (
                                <div className="flex justify-between items-center text-[10px]">
                                  <div className="flex gap-3">
                                    <div className="flex flex-col">
                                      <span className="scale-90 origin-left" style={{ color: 'var(--text-secondary)' }}>LEV</span>
                                      <span className="font-bold" style={{ color: 'var(--text-primary)' }}>{strategy.config.risk_control.btc_eth_max_leverage || '-'}x</span>
                                    </div>
                                    <div className="flex flex-col">
                                      <span className="scale-90 origin-left" style={{ color: 'var(--text-secondary)' }}>POS</span>
                                      <span className="font-bold" style={{ color: 'var(--text-primary)' }}>{strategy.config.risk_control.max_positions || '-'}</span>
                                    </div>
                                  </div>
                                  <Activity size={12} style={{ color: 'var(--text-secondary)' }} />
                                </div>
                              )}

                              {isGridStrategy(strategy.config) && (
                                <div
                                  className="rounded px-2 py-2 text-[10px] leading-5"
                                  style={{ border: '1px solid rgba(240,185,11,0.18)', background: 'rgba(240,185,11,0.06)', color: 'var(--text-secondary)' }}
                                >
                                  <div style={{ color: 'var(--text-primary)' }}>
                                    {language === 'zh' ? '网格初始基准' : 'Grid Initial Base'}: {formatGridBaseLabel(strategy.config)}
                                  </div>
                                  <div>
                                    {language === 'zh'
                                      ? '实际挂单按实时账户权益复利计算，上述金额仅作启动与回退参考。'
                                      : 'Live grid orders compound from real-time account equity. The amount above is only the startup and fallback base.'}
                                  </div>
                                </div>
                              )}
                            </div>
                          ) : (
                            <div className="flex flex-col items-center justify-center h-full" style={{ color: 'var(--text-secondary)' }}>
                              <EyeOff size={16} className="mb-1 opacity-50" />
                              <span className="text-[9px] uppercase tracking-widest">{tr('configHiddenDesc')}</span>
                            </div>
                          )}
                        </div>

                        {/* Action Button */}
                        <div>
                          {strategy.config_visible && strategy.config ? (
                            <button
                              onClick={() => handleCopyConfig(strategy)}
                              className="w-full py-2.5 text-[10px] font-bold font-mono uppercase tracking-widest transition-all flex items-center justify-center gap-2 group/btn"
                              style={{ border: '1px solid var(--panel-border)', background: 'var(--panel-bg-solid)', color: 'var(--text-primary)' }}
                              onMouseEnter={(e) => {
                                e.currentTarget.style.borderColor = 'rgba(214, 144, 7, 0.28)'
                                e.currentTarget.style.background = 'var(--panel-bg-hover)'
                                e.currentTarget.style.color = '#D69007'
                              }}
                              onMouseLeave={(e) => {
                                e.currentTarget.style.borderColor = 'var(--panel-border)'
                                e.currentTarget.style.background = 'var(--panel-bg-solid)'
                                e.currentTarget.style.color = 'var(--text-primary)'
                              }}
                            >
                              {copiedId === strategy.id ? (
                                <>
                                  <Check className="w-3 h-3 text-emerald-500" />
                                  <span className="text-emerald-500">{tr('copied')}</span>
                                </>
                              ) : (
                                <>
                                  <Copy className="w-3 h-3 group-hover/btn:scale-110 transition-transform" />
                                  {tr('copyConfig')}
                                </>
                              )}
                            </button>
                          ) : (
                            <button disabled className="w-full py-2.5 text-[10px] font-bold font-mono uppercase tracking-widest cursor-not-allowed flex items-center justify-center gap-2" style={{ border: '1px solid var(--panel-border)', background: 'var(--panel-bg)', color: 'var(--text-secondary)' }}>
                              <Shield size={12} />
                              {tr('hideConfig')}
                            </button>
                          )}
                        </div>

                      </div>
                    </motion.div>
                  )
                })}
              </AnimatePresence>
            </div>
          )}

          {/* CTA - Share Strategy */}
          {user && token && (
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.3 }}
              className="mt-16 mb-20 flex justify-center"
            >
              <div className="relative group cursor-pointer" onClick={() => window.location.href = '/strategy'}>
                <div className="absolute -inset-1 bg-gradient-to-r from-nofx-gold to-yellow-600 rounded blur opacity-25 group-hover:opacity-75 transition duration-1000 group-hover:duration-200"></div>
                <div
                  className="relative px-8 py-4 flex items-center gap-4 transition-all"
                  style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.borderColor = 'rgba(214, 144, 7, 0.28)'
                    e.currentTarget.style.background = 'var(--panel-bg-hover)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.borderColor = 'var(--panel-border)'
                    e.currentTarget.style.background = 'var(--panel-bg-solid)'
                  }}
                >
                  <Hexagon className="text-nofx-gold animate-spin-slow" size={24} />
                  <div className="text-left">
                    <div className="text-sm font-bold uppercase tracking-wider group-hover:text-nofx-gold transition-colors" style={{ color: 'var(--text-primary)' }}>{tr('shareYours')}</div>
                    <div className="text-[10px] font-mono" style={{ color: 'var(--text-secondary)' }}>CONTRIBUTE TO THE GLOBAL DATABASE</div>
                  </div>
                  <div className="w-[1px] h-8 mx-2" style={{ background: 'var(--panel-border)' }}></div>
                  <div className="text-xs font-mono group-hover:translate-x-1 transition-transform" style={{ color: 'var(--text-secondary)' }}>
                    INITIALIZE_UPLOAD -&gt;
                  </div>
                </div>
              </div>
            </motion.div>
          )}

        </div>
      </div>
    </DeepVoidBackground>
  )
}
