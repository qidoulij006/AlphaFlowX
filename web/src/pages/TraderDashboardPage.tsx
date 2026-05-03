import { useEffect, useState, useRef, type CSSProperties, type ReactNode, type Ref } from 'react'
import { mutate } from 'swr'
import { api } from '../lib/api'
import { ChartTabs } from '../components/charts/ChartTabs'
import { DecisionCard } from '../components/trader/DecisionCard'
import { PositionHistory } from '../components/trader/PositionHistory'
import { PunkAvatar, getTraderAvatar } from '../components/common/PunkAvatar'
import { CopyButton } from '../components/common/CopyButton'
import { confirmToast, notify } from '../lib/notify'
import { formatPrice, formatQuantity } from '../utils/format'
import { t, type Language } from '../i18n/translations'
import { LogOut, Loader2, Eye, EyeOff, ChevronDown, ChevronUp } from 'lucide-react'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { GridRiskPanel } from '../components/strategy/GridRiskPanel'
import type {
    SystemStatus,
    AccountInfo,
    Position,
    DecisionRecord,
    Statistics,
    TraderInfo,
    Exchange,
} from '../types'

// --- Helper Functions ---

// Get friendly AI model display name
function getModelDisplayName(modelId: string): string {
    switch (modelId.toLowerCase()) {
        case 'deepseek':
            return 'DeepSeek'
        case 'qwen':
            return 'Qwen'
        case 'claude':
            return 'Claude'
        case 'claw402':
            return 'Private Model'
        default:
            return modelId.toUpperCase()
    }
}

// Helper function to get exchange display name from exchange ID (UUID)
function getExchangeDisplayNameFromList(
    exchangeId: string | undefined,
    exchanges: Exchange[] | undefined
): string {
    if (!exchangeId) return 'Unknown'
    const exchange = exchanges?.find((e) => e.id === exchangeId)
    if (!exchange) return exchangeId.substring(0, 8).toUpperCase() + '...'
    const typeName = exchange.exchange_type?.toUpperCase() || exchange.name
    return exchange.account_name
        ? `${typeName} - ${exchange.account_name}`
        : typeName
}

// Helper function to get exchange type from exchange ID (UUID) - for kline charts
function getExchangeTypeFromList(
    exchangeId: string | undefined,
    exchanges: Exchange[] | undefined
): string {
    if (!exchangeId) return 'binance'
    const exchange = exchanges?.find((e) => e.id === exchangeId)
    if (!exchange) return 'binance' // Default to binance for charts
    return exchange.exchange_type?.toLowerCase() || 'binance'
}

// Helper function to check if exchange is a perp-dex type (wallet-based)
function isPerpDexExchange(exchangeType: string | undefined): boolean {
    if (!exchangeType) return false
    const perpDexTypes = ['hyperliquid', 'lighter', 'aster']
    return perpDexTypes.includes(exchangeType.toLowerCase())
}

// Helper function to get wallet address for perp-dex exchanges
function getWalletAddress(exchange: Exchange | undefined): string | undefined {
    if (!exchange) return undefined
    const type = exchange.exchange_type?.toLowerCase()
    switch (type) {
        case 'hyperliquid':
            return exchange.hyperliquidWalletAddr
        case 'lighter':
            return exchange.lighterWalletAddr
        case 'aster':
            return exchange.asterSigner
        default:
            return undefined
    }
}

// Helper function to truncate wallet address for display
function truncateAddress(address: string, startLen = 6, endLen = 4): string {
    if (address.length <= startLen + endLen + 3) return address
    return `${address.slice(0, startLen)}...${address.slice(-endLen)}`
}

// --- Components ---

interface TraderDashboardPageProps {
    selectedTrader?: TraderInfo
    traders?: TraderInfo[]
    tradersError?: Error
    selectedTraderId?: string
    onTraderSelect: (traderId: string) => void
    onNavigateToTraders: () => void
    status?: SystemStatus
    account?: AccountInfo
    positions?: Position[]
    decisions?: DecisionRecord[]
    decisionsLimit: number
    onDecisionsLimitChange: (limit: number) => void
    stats?: Statistics
    lastUpdate: string
    language: Language
    exchanges?: Exchange[]
    readOnly?: boolean
    hideTraderSelector?: boolean
    shareToken?: string
}

export function TraderDashboardPage({
    selectedTrader,
    status,
    account,
    positions,
    decisions,
    decisionsLimit,
    onDecisionsLimitChange,
    stats,
    lastUpdate,
    language,
    traders,
    tradersError,
    selectedTraderId,
    onTraderSelect,
    onNavigateToTraders,
    exchanges,
    readOnly = false,
    hideTraderSelector = false,
    shareToken,
}: TraderDashboardPageProps) {
    const [closingPosition, setClosingPosition] = useState<string | null>(null)
    const [selectedChartSymbol, setSelectedChartSymbol] = useState<string | undefined>(undefined)
    const [chartUpdateKey, setChartUpdateKey] = useState<number>(0)
    const chartSectionRef = useRef<HTMLDivElement>(null)
    const [showWalletAddress, setShowWalletAddress] = useState<boolean>(false)
    const [collapsedSections, setCollapsedSections] = useState<Record<string, boolean>>({
        overview: false,
        chart: false,
        positions: false,
        decisions: false,
        history: false,
    })
    const [collapsedSubSections, setCollapsedSubSections] = useState<Record<string, boolean>>({
        positionsTable: false,
    })

    // Current positions pagination
    const [positionsPageSize, setPositionsPageSize] = useState<number>(20)
    const [positionsCurrentPage, setPositionsCurrentPage] = useState<number>(1)
    const [runtimeNow, setRuntimeNow] = useState<number>(() => Date.now())

    // Calculate paginated positions
    const totalPositions = positions?.length || 0
    const totalPositionPages = Math.ceil(totalPositions / positionsPageSize)
    const paginatedPositions = positions?.slice(
        (positionsCurrentPage - 1) * positionsPageSize,
        positionsCurrentPage * positionsPageSize
    ) || []
    const selectedStrategyName = selectedTrader?.strategy_name || ''
    const isGridTrader =
        status?.strategy_type === 'grid_trading' ||
        selectedStrategyName.toLowerCase().includes('grid') ||
        selectedStrategyName.includes('网格')

    useEffect(() => {
        const timer = window.setInterval(() => {
            setRuntimeNow(Date.now())
        }, 60_000)
        return () => window.clearInterval(timer)
    }, [])

    const toggleSection = (sectionId: string) => {
        setCollapsedSections((prev) => ({
            ...prev,
            [sectionId]: !prev[sectionId],
        }))
    }

    const toggleSubSection = (sectionId: string) => {
        setCollapsedSubSections((prev) => ({
            ...prev,
            [sectionId]: !prev[sectionId],
        }))
    }

    // Reset page when positions change
    useEffect(() => {
        setPositionsCurrentPage(1)
    }, [selectedTraderId, positionsPageSize])

    // Auto-set chart symbol for grid trading
    useEffect(() => {
        if (isGridTrader && status?.grid_symbol) {
            setSelectedChartSymbol(status.grid_symbol)
        }
    }, [isGridTrader, status?.grid_symbol])

    // Get current exchange info for perp-dex wallet display
    const currentExchange = exchanges?.find(
        (e) => e.id === selectedTrader?.exchange_id
    )
    const walletAddress = getWalletAddress(currentExchange)
    const isPerpDex = isPerpDexExchange(currentExchange?.exchange_type)

    const persistentCycleCount = Math.max(stats?.total_cycles || 0, status?.call_count || 0)
    const persistentRuntimeMinutes = (() => {
        if (selectedTrader?.created_at) {
            const createdAt = new Date(selectedTrader.created_at).getTime()
            if (!Number.isNaN(createdAt) && createdAt > 0) {
                return Math.max(0, Math.floor((runtimeNow - createdAt) / 60000))
            }
        }
        return status?.runtime_minutes || 0
    })()

    const runtimeLabel = (() => {
        if (persistentRuntimeMinutes >= 1440) {
            const days = Math.floor(persistentRuntimeMinutes / 1440)
            const hours = Math.floor((persistentRuntimeMinutes % 1440) / 60)
            return language === 'zh' ? `${days}天 ${hours}小时` : `${days}d ${hours}h`
        }
        if (persistentRuntimeMinutes >= 60) {
            const hours = Math.floor(persistentRuntimeMinutes / 60)
            const minutes = persistentRuntimeMinutes % 60
            return language === 'zh' ? `${hours}小时 ${minutes}分钟` : `${hours}h ${minutes}m`
        }
        return language === 'zh' ? `${persistentRuntimeMinutes}分钟` : `${persistentRuntimeMinutes} min`
    })()

    // Handle symbol click from Decision Card
    const handleSymbolClick = (symbol: string) => {
        // Set the selected symbol
        setSelectedChartSymbol(symbol)
        // Scroll to chart section
        setTimeout(() => {
            chartSectionRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
        }, 100)
    }

    // Close position handler
    const handleClosePosition = async (symbol: string, side: string) => {
        if (!selectedTraderId) return

        const sideLabel = side === 'LONG' ? 'LONG' : 'SHORT'
        const confirmMsg = t('traderDashboard.confirmClosePosition', language, { symbol, side: sideLabel })

        const confirmed = await confirmToast(confirmMsg, {
            title: t('traderDashboard.confirmClose', language),
            okText: t('traderDashboard.confirm', language),
            cancelText: t('traderDashboard.cancel', language),
        })

        if (!confirmed) return

        setClosingPosition(symbol)
        try {
            await api.closePosition(selectedTraderId, symbol, side)
            notify.success(t('traderDashboard.positionClosed', language))
            // Use SWR mutate to refresh data instead of reloading page
            await Promise.all([
                mutate(`positions-${selectedTraderId}`),
                mutate(`account-${selectedTraderId}`),
            ])
        } catch (err: unknown) {
            const errorMsg =
                err instanceof Error
                    ? err.message
                    : t('traderDashboard.closeFailed', language)
            notify.error(errorMsg)
        } finally {
            setClosingPosition(null)
        }
    }

    const handleCreateShareLink = async () => {
        if (!selectedTrader) return
        try {
            const result = await api.createShareLink(selectedTrader.trader_id)
            const shareUrl = new URL(result.url)
            shareUrl.searchParams.set('lang', language)
            const shareCopy = language === 'zh'
                ? `来自AlphaFlowX的AI交易实盘，实盘名称："${selectedTrader.trader_name}"，地址：${shareUrl.toString()}`
                : `Live AI trading from AlphaFlowX. Trader name: "${selectedTrader.trader_name}", URL: ${shareUrl.toString()}`
            await navigator.clipboard.writeText(shareCopy)
            notify.success(language === 'zh' ? '内部分享链接已复制' : 'Internal share link copied')
        } catch (err) {
            notify.error(err instanceof Error ? err.message : (language === 'zh' ? '生成分享链接失败' : 'Failed to create share link'))
        }
    }

    // If API failed with error, show empty state (likely backend not running)
    if (tradersError) {
        return (
            <div className="flex items-center justify-center min-h-[60vh] relative z-10">
                <div className="text-center max-w-md mx-auto px-6">
                    <div
                        className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center nofx-glass"
                        style={{
                            background: 'rgba(240, 185, 11, 0.1)',
                            borderColor: 'rgba(240, 185, 11, 0.3)',
                        }}
                    >
                        <svg
                            className="w-12 h-12 text-nofx-gold"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                        >
                            <path
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                strokeWidth={2}
                                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                            />
                        </svg>
                    </div>
                    <h2 className="text-2xl font-bold mb-3 text-nofx-text-main">
                        {t('traderDashboard.connectionFailed', language)}
                    </h2>
                    <p className="text-base mb-6 text-nofx-text-muted">
                        {t('traderDashboard.connectionFailedDesc', language)}
                    </p>
                    <button
                        onClick={() => window.location.reload()}
                        className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95 nofx-glass border border-nofx-gold/30 text-nofx-gold hover:bg-nofx-gold/10"
                    >
                        {t('traderDashboard.retry', language)}
                    </button>
                </div>
            </div>
        )
    }

    // If traders is loaded and empty, show empty state
    if (traders && traders.length === 0) {
        return (
            <div className="flex items-center justify-center min-h-[60vh] relative z-10">
                <div className="text-center max-w-md mx-auto px-6">
                    <div
                        className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center nofx-glass"
                        style={{
                            background: 'rgba(240, 185, 11, 0.1)',
                            borderColor: 'rgba(240, 185, 11, 0.3)',
                        }}
                    >
                        <svg
                            className="w-12 h-12 text-nofx-gold"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                        >
                            <path
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                strokeWidth={2}
                                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                            />
                        </svg>
                    </div>
                    <h2 className="text-2xl font-bold mb-3 text-nofx-text-main">
                        {t('dashboardEmptyTitle', language)}
                    </h2>
                    <p className="text-base mb-6 text-nofx-text-muted">
                        {t('dashboardEmptyDescription', language)}
                    </p>
                    <button
                        onClick={onNavigateToTraders}
                        className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95 nofx-glass border border-nofx-gold/30 text-nofx-gold hover:bg-nofx-gold/10"
                    >
                        {t('goToTradersPage', language)}
                    </button>
                </div>
            </div>
        )
    }

    // If traders is still loading or selectedTrader is not ready, show skeleton
    if (!selectedTrader) {
        return (
            <div className="space-y-6 relative z-10">
                <div className="nofx-glass p-6 animate-pulse">
                    <div className="h-8 w-48 mb-3 bg-nofx-bg/50 rounded"></div>
                    <div className="flex gap-4">
                        <div className="h-4 w-32 bg-nofx-bg/50 rounded"></div>
                        <div className="h-4 w-24 bg-nofx-bg/50 rounded"></div>
                        <div className="h-4 w-28 bg-nofx-bg/50 rounded"></div>
                    </div>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                    {[1, 2, 3, 4].map((i) => (
                        <div key={i} className="nofx-glass p-5 animate-pulse">
                            <div className="h-4 w-24 mb-3 bg-nofx-bg/50 rounded"></div>
                            <div className="h-8 w-32 bg-nofx-bg/50 rounded"></div>
                        </div>
                    ))}
                </div>
                <div className="nofx-glass p-6 animate-pulse">
                    <div className="h-6 w-40 mb-4 bg-nofx-bg/50 rounded"></div>
                    <div className="h-64 w-full bg-nofx-bg/50 rounded"></div>
                </div>
            </div>
        )
    }

    return (
        <DeepVoidBackground className="trader-dashboard-page min-h-screen pb-12" disableAnimation>
            <div className="trader-dashboard-shell w-full px-4 md:px-8 relative z-10 pt-6">
                {/* Trader Header */}
                <div
                    className="trader-dashboard-header mb-6 rounded-lg p-6 animate-scale-in nofx-glass group"
                    style={{
                        background: 'linear-gradient(135deg, var(--panel-bg) 0%, color-mix(in srgb, var(--panel-bg-solid) 90%, transparent) 100%)',
                        border: '1px solid var(--panel-border)',
                    }}
                >
                    <div className="trader-dashboard-header-top flex items-start justify-between mb-4">
                        <h2 className="text-2xl font-bold flex items-center gap-4 text-nofx-text-main">
                            <div className="relative">
                                <PunkAvatar
                                    seed={getTraderAvatar(
                                        selectedTrader.trader_id,
                                        selectedTrader.trader_name
                                    )}
                                    size={56}
                                    className="rounded-xl border-2 border-nofx-gold/30 shadow-[0_0_15px_rgba(240,185,11,0.2)]"
                                />
                                <div className="absolute -bottom-1 -right-1 w-4 h-4 bg-nofx-green rounded-full shadow-[0_0_8px_rgba(14,203,129,0.8)] animate-pulse border-2 border-[var(--panel-bg-solid)]" />
                            </div>
                            <div className="flex flex-col">
                                <span className="text-3xl tracking-tight text-nofx-text font-semibold">
                                    {selectedTrader.trader_name}
                                </span>
                                <span className="text-xs font-mono text-nofx-text-muted opacity-60 flex items-center gap-2">
                                    <div className="w-1.5 h-1.5 bg-nofx-gold rounded-full" />
                                    <span>ID: {selectedTrader.trader_id.slice(0, 8)}...</span>
                                    <span
                                        className="inline-flex rounded transition-colors"
                                        onMouseEnter={(e) => {
                                            e.currentTarget.style.background = 'var(--panel-bg-hover)'
                                        }}
                                        onMouseLeave={(e) => {
                                            e.currentTarget.style.background = 'transparent'
                                        }}
                                    >
                                        <CopyButton
                                            text={selectedTrader.trader_id}
                                            title={`${t('copy', language)} Trader ID`}
                                            successMessage={language === 'zh' ? 'Trader ID 已复制' : 'Trader ID copied'}
                                            className="p-0.5 rounded transition-colors"
                                            iconClassName="w-3 h-3 text-nofx-text-muted"
                                        />
                                    </span>
                                </span>
                            </div>
                        </h2>

                        <div className="trader-dashboard-header-actions flex items-center gap-4">
                            {/* Trader Selector */}
                            {!hideTraderSelector && traders && traders.length > 0 && (
                                <div className="flex items-center gap-2 px-1 py-1 rounded-lg" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}>
                                    <select
                                        value={selectedTraderId}
                                        onChange={(e) => onTraderSelect(e.target.value)}
                                        className="bg-transparent text-sm font-medium cursor-pointer transition-colors text-nofx-text-main focus:outline-none px-2 py-1"
                                    >
                                        {traders.map((trader) => (
                                            <option key={trader.trader_id} value={trader.trader_id} style={{ background: 'var(--panel-bg-solid)', color: 'var(--text-primary)' }}>
                                                {trader.trader_name}
                                            </option>
                                        ))}
                                    </select>
                                </div>
                            )}

                            {!readOnly && (
                                <button
                                    type="button"
                                    onClick={() => void handleCreateShareLink()}
                                    className="px-3 py-1.5 rounded-lg text-sm font-semibold transition-all text-nofx-text-main hover:text-nofx-gold"
                                    style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}
                                >
                                    {language === 'zh' ? '内部分享' : 'Internal Share'}
                                </button>
                            )}

                            {/* Wallet Address Display for Perp-DEX */}
                            {exchanges && isPerpDex && (
                                <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg nofx-glass border border-nofx-gold/20">
                                    {walletAddress ? (
                                        <>
                                            <span className="text-xs font-mono text-nofx-gold">
                                                {showWalletAddress
                                                    ? walletAddress
                                                    : truncateAddress(walletAddress)}
                                            </span>
                                            <button
                                                type="button"
                                                onClick={() => setShowWalletAddress(!showWalletAddress)}
                                                className="p-1 rounded transition-colors"
                                                title={
                                                    showWalletAddress
                                                        ? t('traderDashboard.hideAddress', language)
                                                        : t('traderDashboard.showFullAddress', language)
                                                }
                                                onMouseEnter={(e) => {
                                                    e.currentTarget.style.background = 'var(--panel-bg-hover)'
                                                }}
                                                onMouseLeave={(e) => {
                                                    e.currentTarget.style.background = 'transparent'
                                                }}
                                            >
                                                {showWalletAddress ? (
                                                    <EyeOff className="w-3.5 h-3.5 text-nofx-text-muted" />
                                                ) : (
                                                    <Eye className="w-3.5 h-3.5 text-nofx-text-muted" />
                                                )}
                                            </button>
                                            <CopyButton
                                                text={walletAddress}
                                                title={t('traderDashboard.copyAddress', language)}
                                                successMessage={language === 'zh' ? '地址已复制' : 'Address copied'}
                                            />
                                        </>
                                    ) : (
                                        <span className="text-xs text-nofx-text-muted">
                                            {t('traderDashboard.noAddressConfigured', language)}
                                        </span>
                                    )}
                                </div>
                            )}
                        </div>
                    </div>
                    <div className="trader-dashboard-meta flex items-center gap-4 md:gap-6 text-sm flex-wrap text-nofx-text-muted font-mono pl-2">
                        <span className="flex items-center gap-2">
                            <span className="opacity-60">{language === 'zh' ? 'AI 模型:' : 'AI Model:'}</span>
                            <span
                                className="font-bold px-2 py-0.5 rounded text-xs tracking-wide"
                                style={{
                                    background: selectedTrader.ai_model.includes('qwen') ? 'rgba(192, 132, 252, 0.15)' : 'rgba(96, 165, 250, 0.15)',
                                    color: selectedTrader.ai_model.includes('qwen') ? '#c084fc' : '#60a5fa',
                                    border: `1px solid ${selectedTrader.ai_model.includes('qwen') ? '#c084fc' : '#60a5fa'}40`
                                }}
                            >
                                {getModelDisplayName(
                                    selectedTrader.ai_model.split('_').pop() ||
                                    selectedTrader.ai_model
                                )}
                            </span>
                        </span>
                        <span className="w-px h-3 hidden md:block" style={{ background: 'var(--panel-border)' }} />
                        <span className="flex items-center gap-2">
                            <span className="opacity-60">{language === 'zh' ? '交易所:' : 'Exchange:'}</span>
                            <span className="text-nofx-text-main font-semibold">
                                {getExchangeDisplayNameFromList(
                                    selectedTrader.exchange_id,
                                    exchanges
                                )}
                            </span>
                        </span>
                        <span className="w-px h-3 hidden md:block" style={{ background: 'var(--panel-border)' }} />
                        <span className="flex items-center gap-2">
                            <span className="opacity-60">{language === 'zh' ? '策略:' : 'Strategy:'}</span>
                            <span className="text-nofx-gold font-semibold tracking-wide">
                                {selectedTrader.strategy_name || (language === 'zh' ? '未配置策略' : 'No Strategy')}
                            </span>
                        </span>
                        {status && (
                            <>
                                <span className="w-px h-3 hidden md:block" style={{ background: 'var(--panel-border)' }} />
                                <div className="basis-full md:basis-auto flex flex-wrap items-center gap-4 md:gap-6">
                                    <span>{language === 'zh' ? '循环次数:' : 'Cycles:'} <span className="text-nofx-text-main">{persistentCycleCount}</span></span>
                                    <span className="w-px h-3 hidden md:block" style={{ background: 'var(--panel-border)' }} />
                                    <span>{language === 'zh' ? '运行时长:' : 'Runtime:'} <span className="text-nofx-text-main">{runtimeLabel}</span></span>
                                </div>
                            </>
                        )}
                    </div>
                </div>

                {/* Debug Info */}
                {account && (
                    <div
                        className="group mb-5 flex flex-wrap items-center justify-between gap-3 rounded-xl px-4 py-2.5 text-[11px] font-mono text-nofx-text-muted transition-all duration-200"
                        style={{
                            background: 'linear-gradient(135deg, color-mix(in srgb, var(--panel-bg) 86%, rgba(240,185,11,0.10) 14%), var(--panel-bg-solid))',
                            border: '1px solid color-mix(in srgb, var(--panel-border) 72%, rgba(240,185,11,0.34) 28%)',
                            boxShadow: '0 10px 28px rgba(15, 23, 42, 0.06)',
                        }}
                    >
                        <span className="font-semibold tracking-[0.18em] text-nofx-text-main transition-colors duration-200 group-hover:text-nofx-gold">
                            {language === 'zh' ? '系统状态::在线' : 'SYSTEM_STATUS::ONLINE'}
                        </span>
                        <div className="flex flex-wrap gap-x-5 gap-y-1.5 text-nofx-text-muted transition-colors duration-200 group-hover:text-nofx-text-main">
                            <span>{language === 'zh' ? '最近更新' : 'LAST_UPDATE'}::{lastUpdate}</span>
                            <span>{language === 'zh' ? '净值' : 'EQ'}::{account?.total_equity?.toFixed(2)}</span>
                            <span>{language === 'zh' ? '盈亏' : 'PNL'}::{account?.total_pnl?.toFixed(2)}</span>
                        </div>
                    </div>
                )}

                <DashboardSection
                    title={language === 'zh' ? '账户概览' : 'Account Overview'}
                    subtitle={language === 'zh' ? '资金、可用余额、盈亏与仓位总体情况' : 'Equity, free balance, PnL, and position overview'}
                    icon="◉"
                    collapsed={collapsedSections.overview}
                    onToggle={() => toggleSection('overview')}
                    className="dashboard-overview-section mb-8 animate-slide-in"
                    contentClassName="p-0"
                >
                    <div className="trader-dashboard-stats grid grid-cols-2 md:grid-cols-4 gap-4 p-5 md:p-6">
                        <StatCard
                            title={t('totalEquity', language)}
                            value={`${account?.total_equity?.toFixed(2) || '0.00'}`}
                            unit="USDT"
                            change={account?.total_pnl_pct || 0}
                            positive={(account?.total_pnl ?? 0) > 0}
                            icon="💰"
                        />
                        <StatCard
                            title={t('availableBalance', language)}
                            value={`${account?.available_balance?.toFixed(2) || '0.00'}`}
                            unit="USDT"
                            subtitle={`${account?.available_balance && account?.total_equity ? ((account.available_balance / account.total_equity) * 100).toFixed(1) : '0.0'}% ${t('free', language)}`}
                            icon="💳"
                        />
                        <StatCard
                            title={t('totalPnL', language)}
                            value={`${account?.total_pnl !== undefined && account.total_pnl >= 0 ? '+' : ''}${account?.total_pnl?.toFixed(2) || '0.00'}`}
                            unit="USDT"
                            change={account?.total_pnl_pct || 0}
                            positive={(account?.total_pnl ?? 0) >= 0}
                            icon="📈"
                        />
                        <StatCard
                            title={t('positions', language)}
                            value={`${account?.position_count || 0}`}
                            unit="ACTIVE"
                            subtitle={`${t('margin', language)}: ${account?.margin_used_pct?.toFixed(1) || '0.0'}%`}
                            icon="📊"
                        />
                    </div>
                </DashboardSection>

                <div className="space-y-6 mb-6">
                    <DashboardSection
                        title={language === 'zh' ? '图表分析' : 'Chart Analysis'}
                        subtitle={language === 'zh' ? '净值曲线、K 线与品种切换分析' : 'Equity, market chart, and symbol analysis'}
                        icon="◈"
                        collapsed={collapsedSections.chart}
                        onToggle={() => toggleSection('chart')}
                        className="dashboard-chart-section animate-slide-in scroll-mt-32"
                        contentClassName="p-0"
                        contentRef={chartSectionRef}
                    >
                        <div className="chart-container backdrop-blur-sm">
                            <ChartTabs
                                traderId={selectedTrader.trader_id}
                                selectedSymbol={selectedChartSymbol}
                                updateKey={chartUpdateKey}
                                exchangeId={getExchangeTypeFromList(
                                    selectedTrader.exchange_id,
                                    exchanges
                                )}
                                shareToken={shareToken}
                            />
                        </div>
                    </DashboardSection>

                    <DashboardSection
                        title={t('recentDecisions', language)}
                        subtitle={language === 'zh' ? '最近决策链路、信号与 AI 判断记录' : 'Latest decision cycles, signals, and AI reasoning records'}
                        icon="🧠"
                        collapsed={collapsedSections.decisions}
                        onToggle={() => toggleSection('decisions')}
                        className="dashboard-decisions-section animate-slide-in"
                        style={{ animationDelay: '0.2s' }}
                        headerBadge={decisions && decisions.length > 0 ? t('lastCycles', language, { count: decisions.length }) : undefined}
                        headerExtra={(
                            <select
                                value={decisionsLimit}
                                onChange={(e) => onDecisionsLimitChange(Number(e.target.value))}
                                onClick={(e) => e.stopPropagation()}
                                className="px-3 py-1.5 rounded-lg text-sm font-medium cursor-pointer transition-all text-nofx-text-main focus:outline-none"
                                style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}
                            >
                                <option value={5}>5</option>
                                <option value={10}>10</option>
                                <option value={20}>20</option>
                                <option value={50}>50</option>
                                <option value={100}>100</option>
                            </select>
                        )}
                        contentClassName="dashboard-decisions-content flex-1 pt-0 min-h-0"
                    >
                        <div
                            className="trader-dashboard-decisions-grid grid grid-cols-1 xl:grid-cols-2 gap-4 overflow-y-auto pr-2 custom-scrollbar"
                            style={{ minHeight: '28rem', maxHeight: 'calc(100vh - 220px)' }}
                        >
                            {decisions && decisions.length > 0 ? (
                                decisions.map((decision, i) => (
                                    <DecisionCard key={i} decision={decision} language={language} onSymbolClick={handleSymbolClick} />
                                ))
                            ) : (
                                <div className="py-16 text-center text-nofx-text-muted opacity-60">
                                    <div className="text-6xl mb-4 opacity-30 grayscale">🧠</div>
                                    <div className="text-lg font-semibold mb-2 text-nofx-text-main">
                                        {t('noDecisionsYet', language)}
                                    </div>
                                    <div className="text-sm">
                                        {t('aiDecisionsWillAppear', language)}
                                    </div>
                                </div>
                            )}
                        </div>
                    </DashboardSection>
                </div>

                <DashboardSection
                    title={isGridTrader
                        ? (language === 'zh' ? '持仓与挂单' : 'Positions & Orders')
                        : t('currentPositions', language)}
                    subtitle={isGridTrader
                        ? (language === 'zh' ? '当前持仓、网格挂单、未实现盈亏与快速平仓' : 'Live positions, grid orders, unrealized PnL, and quick close actions')
                        : (language === 'zh' ? '当前持仓、杠杆、未实现盈亏与快速平仓' : 'Live positions, leverage, unrealized PnL, and quick close actions')}
                    icon="◉"
                    collapsed={collapsedSections.positions}
                    onToggle={() => toggleSection('positions')}
                    className="dashboard-positions-section mb-6 animate-slide-in relative overflow-hidden group"
                    contentClassName="px-5 pb-5 pt-5"
                    style={{ animationDelay: '0.15s' }}
                    headerBadge={positions && positions.length > 0 ? `${positions.length} ${t('active', language)}` : undefined}
                >
                    <div className="absolute top-0 right-0 p-3 opacity-10 group-hover:opacity-20 transition-opacity">
                        <div className="w-24 h-24 rounded-full bg-blue-500 blur-3xl" />
                    </div>
                    <SubModuleSection
                        title={language === 'zh' ? '持仓' : 'Positions'}
                        accent="#5B8DEF"
                        collapsed={collapsedSubSections.positionsTable}
                        onToggle={() => toggleSubSection('positionsTable')}
                    >
                    {positions && positions.length > 0 ? (
                        <div>
                            <div className="trader-dashboard-positions-table-wrap overflow-x-auto">
                                <table className="trader-dashboard-positions-table w-full text-xs">
                                            <thead className="text-left" style={{ borderBottom: '1px solid var(--panel-border)' }}>
                                                <tr>
                                                    <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-left">{t('symbol', language)}</th>
                                                    <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-center">{t('side', language)}</th>
                                                    <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-center">{t('traderDashboard.action', language)}</th>
                                                    <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-right hidden md:table-cell" title={t('entryPrice', language)}>{t('traderDashboard.entry', language)}</th>
                                                    <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-right hidden md:table-cell" title={t('markPrice', language)}>{t('traderDashboard.mark', language)}</th>
                                                    <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-right" title={t('quantity', language)}>{t('traderDashboard.qty', language)}</th>
                                                    <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-right hidden md:table-cell" title={t('positionValue', language)}>{t('traderDashboard.value', language)}</th>
                                                    <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-center hidden md:table-cell" title={t('leverage', language)}>{t('traderDashboard.lev', language)}</th>
                                                    <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-right" title={t('unrealizedPnL', language)}>{t('traderDashboard.uPnL', language)}</th>
                                                    <th className="px-1 pb-3 font-semibold text-nofx-text-muted whitespace-nowrap text-right hidden md:table-cell" title={t('liqPrice', language)}>{t('traderDashboard.liq', language)}</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {paginatedPositions.map((pos, i) => (
                                                    <tr
                                                        key={i}
                                                        className="last:border-0 transition-all cursor-pointer group/row"
                                                        style={{ borderBottom: '1px solid var(--panel-border)' }}
                                                        onClick={() => {
                                                            setSelectedChartSymbol(pos.symbol)
                                                            setChartUpdateKey(Date.now())
                                                            if (chartSectionRef.current) {
                                                                chartSectionRef.current.scrollIntoView({
                                                                    behavior: 'smooth',
                                                                    block: 'start',
                                                                })
                                                            }
                                                        }}
                                                        onMouseEnter={(e) => {
                                                            e.currentTarget.style.background = 'var(--panel-bg)'
                                                        }}
                                                        onMouseLeave={(e) => {
                                                            e.currentTarget.style.background = 'transparent'
                                                        }}
                                                    >
                                                        <td className="px-1 py-3 font-mono font-semibold whitespace-nowrap text-left text-nofx-text-main transition-colors">
                                                            {pos.symbol}
                                                        </td>
                                                        <td className="px-1 py-3 whitespace-nowrap text-center">
                                                            <span
                                                                className={`px-1.5 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider ${pos.side === 'long' ? 'bg-nofx-green/10 text-nofx-green shadow-[0_0_8px_rgba(14,203,129,0.2)]' : 'bg-nofx-red/10 text-nofx-red shadow-[0_0_8px_rgba(246,70,93,0.2)]'}`}
                                                            >
                                                                {t(pos.side === 'long' ? 'long' : 'short', language)}
                                                            </span>
                                                        </td>
                                                        <td className="px-1 py-3 whitespace-nowrap text-center">
                                                            {!readOnly && (
                                                            <button
                                                                type="button"
                                                                onClick={(e) => {
                                                                    e.stopPropagation()
                                                                    handleClosePosition(pos.symbol, pos.side.toUpperCase())
                                                                }}
                                                                disabled={closingPosition === pos.symbol}
                                                                className="inline-flex items-center gap-1 px-2 py-1 rounded text-[10px] font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed mx-auto bg-nofx-red/10 text-nofx-red border border-nofx-red/30 hover:bg-nofx-red/20"
                                                                title={t('traderDashboard.closePosition', language)}
                                                            >
                                                                {closingPosition === pos.symbol ? (
                                                                    <Loader2 className="w-3 h-3 animate-spin" />
                                                                ) : (
                                                                    <LogOut className="w-3 h-3" />
                                                                )}
                                                                {t('traderDashboard.close', language)}
                                                            </button>
                                                            )}
                                                        </td>
                                                        <td className="px-1 py-3 font-mono whitespace-nowrap text-right text-nofx-text-main hidden md:table-cell">{formatPrice(pos.entry_price)}</td>
                                                        <td className="px-1 py-3 font-mono whitespace-nowrap text-right text-nofx-text-main hidden md:table-cell">{formatPrice(pos.mark_price)}</td>
                                                        <td className="px-1 py-3 font-mono whitespace-nowrap text-right text-nofx-text-main">{formatQuantity(pos.quantity)}</td>
                                                        <td className="px-1 py-3 font-mono font-bold whitespace-nowrap text-right text-nofx-text-main hidden md:table-cell">{(pos.quantity * pos.mark_price).toFixed(2)}</td>
                                                        <td className="px-1 py-3 font-mono whitespace-nowrap text-center text-nofx-gold hidden md:table-cell">{pos.leverage}x</td>
                                                        <td className="px-1 py-3 font-mono whitespace-nowrap text-right">
                                                            <span
                                                                className={`font-bold ${pos.unrealized_pnl >= 0 ? 'text-nofx-green shadow-nofx-green' : 'text-nofx-red shadow-nofx-red'}`}
                                                                style={{ textShadow: pos.unrealized_pnl >= 0 ? '0 0 10px rgba(14,203,129,0.3)' : '0 0 10px rgba(246,70,93,0.3)' }}
                                                            >
                                                                {pos.unrealized_pnl >= 0 ? '+' : ''}
                                                                {pos.unrealized_pnl.toFixed(2)}
                                                            </span>
                                                        </td>
                                                        <td className="px-1 py-3 font-mono whitespace-nowrap text-right text-nofx-text-muted hidden md:table-cell">{formatPrice(pos.liquidation_price)}</td>
                                                    </tr>
                                                ))}
                                            </tbody>
                                </table>
                            </div>
                            {totalPositions > 10 && (
                                <div className="flex flex-wrap items-center justify-between gap-3 pt-4 mt-4 text-xs text-nofx-text-muted" style={{ borderTop: '1px solid var(--panel-border)' }}>
                                    <span>
                                        {t('traderDashboard.showingPositions', language, { shown: paginatedPositions.length, total: totalPositions })}
                                    </span>
                                    <div className="flex items-center gap-3">
                                        <div className="flex items-center gap-2">
                                            <span>{t('traderDashboard.perPage', language)}:</span>
                                            <select
                                                value={positionsPageSize}
                                                onChange={(e) => setPositionsPageSize(Number(e.target.value))}
                                                className="rounded px-2 py-1 text-xs text-nofx-text-main focus:outline-none transition-colors"
                                                style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}
                                            >
                                                <option value={20}>20</option>
                                                <option value={50}>50</option>
                                                <option value={100}>100</option>
                                            </select>
                                        </div>
                                        {totalPositionPages > 1 && (
                                            <div className="flex items-center gap-1">
                                                {['«', '‹', `${positionsCurrentPage} / ${totalPositionPages}`, '›', '»'].map((label, idx) => {
                                                    const isText = idx === 2;
                                                    const isFirst = idx === 0;
                                                    const isPrev = idx === 1;
                                                    const isNext = idx === 3;
                                                    const isLast = idx === 4;
                                                    if (isText) return <span key={idx} className="px-3 text-nofx-text-main">{label}</span>;

                                                    let onClick = () => { };
                                                    let disabled = false;

                                                    if (isFirst) { onClick = () => setPositionsCurrentPage(1); disabled = positionsCurrentPage === 1; }
                                                    if (isPrev) { onClick = () => setPositionsCurrentPage(p => Math.max(1, p - 1)); disabled = positionsCurrentPage === 1; }
                                                    if (isNext) { onClick = () => setPositionsCurrentPage(p => Math.min(totalPositionPages, p + 1)); disabled = positionsCurrentPage === totalPositionPages; }
                                                    if (isLast) { onClick = () => setPositionsCurrentPage(totalPositionPages); disabled = positionsCurrentPage === totalPositionPages; }

                                                    return (
                                                        <button
                                                            key={idx}
                                                            onClick={onClick}
                                                            disabled={disabled}
                                                            className="px-2 py-1 rounded transition-colors text-nofx-text-main"
                                                            style={{
                                                                opacity: disabled ? 0.3 : 1,
                                                                cursor: disabled ? 'not-allowed' : 'pointer',
                                                                background: disabled ? 'var(--panel-bg)' : 'var(--panel-bg)',
                                                                border: '1px solid var(--panel-border)',
                                                            }}
                                                        >
                                                            {label}
                                                        </button>
                                                    )
                                                })}
                                            </div>
                                        )}
                                    </div>
                                </div>
                            )}
                        </div>
                    ) : (
                        <div className="text-center py-16 text-nofx-text-muted opacity-60">
                            <div className="text-6xl mb-4 opacity-50 grayscale">📊</div>
                            <div className="text-lg font-semibold mb-2">{t('noPositions', language)}</div>
                            <div className="text-sm">{t('noActivePositions', language)}</div>
                        </div>
                    )}
                    </SubModuleSection>
                    {isGridTrader && selectedTraderId && (
                        <div className="mt-6">
                            <GridRiskPanel
                                traderId={selectedTraderId}
                                language={language}
                                refreshInterval={5000}
                                embedded
                                shareToken={shareToken}
                            />
                        </div>
                    )}
                </DashboardSection>

                {/* Position History Section */}
                {selectedTraderId && (
                    <DashboardSection
                        title={t('positionHistory.title', language)}
                        subtitle={language === 'zh' ? '已平仓记录、历史盈亏与过去仓位轨迹' : 'Closed positions, historical PnL, and prior position activity'}
                        icon="📜"
                        collapsed={collapsedSections.history}
                        onToggle={() => toggleSection('history')}
                        className="dashboard-history-section animate-slide-in"
                        style={{ animationDelay: '0.25s' }}
                    >
                        <PositionHistory traderId={selectedTraderId} shareToken={shareToken} />
                    </DashboardSection>
                )}
            </div>
        </DeepVoidBackground>
    )
}

function DashboardSection({
    title,
    subtitle,
    icon,
    collapsed,
    onToggle,
    children,
    className = '',
    contentClassName = 'p-6',
    headerBadge,
    headerExtra,
    style,
    contentRef,
}: {
    title: string
    subtitle?: string
    icon?: string
    collapsed: boolean
    onToggle: () => void
    children: ReactNode
    className?: string
    contentClassName?: string
    headerBadge?: string
    headerExtra?: ReactNode
    style?: CSSProperties
    contentRef?: Ref<HTMLDivElement>
}) {
    return (
        <div className={`nofx-glass overflow-hidden rounded-2xl ${className}`} style={style}>
            <div
                role="button"
                tabIndex={0}
                onClick={onToggle}
                onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        onToggle()
                    }
                }}
                className="flex w-full items-start justify-between gap-4 px-5 py-4 text-left transition-colors"
                style={{ borderBottom: collapsed ? 'none' : '1px solid var(--panel-border)' }}
            >
                <div className="flex items-start gap-3 min-w-0">
                    <div
                        className="mt-1 h-10 w-1 shrink-0 rounded-full"
                        style={{ background: 'linear-gradient(180deg, rgba(240,185,11,0.88), rgba(240,185,11,0.24))' }}
                    />
                    <div className="min-w-0 pt-0.5">
                        <div className="flex items-center gap-2">
                            {icon && (
                                <span className="text-[13px] opacity-70" style={{ color: 'var(--text-secondary)' }}>
                                    {icon}
                                </span>
                            )}
                            <div className="text-[1.02rem] font-semibold tracking-[0.01em] text-nofx-text-main">{title}</div>
                        </div>
                        {subtitle && (
                            <div className="mt-1.5 max-w-2xl text-[13px] leading-6 text-nofx-text-muted">{subtitle}</div>
                        )}
                    </div>
                </div>
                <div className="flex items-center gap-3 shrink-0">
                    {headerBadge && (
                        <div
                            className="hidden md:inline-flex rounded-md px-2.5 py-1 text-[11px] font-medium"
                            style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-secondary)' }}
                        >
                            {headerBadge}
                        </div>
                    )}
                    {headerExtra && (
                        <div onClick={(e) => e.stopPropagation()}>
                            {headerExtra}
                        </div>
                    )}
                    <div className="rounded-full border px-2.5 py-2 transition-colors" style={{ borderColor: 'var(--panel-border)', background: 'var(--panel-bg)' }}>
                        {collapsed ? (
                            <ChevronDown className="h-4 w-4 text-nofx-text-muted" />
                        ) : (
                            <ChevronUp className="h-4 w-4 text-nofx-text-muted" />
                        )}
                    </div>
                </div>
            </div>
            {!collapsed && (
                <div ref={contentRef} className={contentClassName}>
                    {children}
                </div>
            )}
        </div>
    )
}

function SubModuleSection({
    title,
    accent = '#F0B90B',
    collapsed,
    onToggle,
    children,
}: {
    title: string
    accent?: string
    collapsed: boolean
    onToggle: () => void
    children: ReactNode
}) {
    return (
        <div className="space-y-4">
            <button
                type="button"
                onClick={onToggle}
                className="flex w-full items-center justify-between gap-4 rounded-xl px-1 py-1 text-left transition-colors"
            >
                <div className="flex min-w-0 items-center gap-3">
                    <div className="h-5 w-0.5 rounded-full" style={{ background: accent }} />
                    <div
                        className="text-sm font-semibold uppercase tracking-[0.08em]"
                        style={{ color: 'var(--text-primary)' }}
                    >
                        {title}
                    </div>
                </div>
                <div className="rounded-full border px-2.5 py-2 transition-colors" style={{ borderColor: 'var(--panel-border)', background: 'var(--panel-bg)' }}>
                    {collapsed ? (
                        <ChevronDown className="h-4 w-4 text-nofx-text-muted" />
                    ) : (
                        <ChevronUp className="h-4 w-4 text-nofx-text-muted" />
                    )}
                </div>
            </button>
            {!collapsed && children}
        </div>
    )
}

// Stat Card Component - Deep Void Style
function StatCard({
    title,
    value,
    unit,
    change,
    positive,
    subtitle,
    icon,
}: {
    title: string
    value: string
    unit?: string
    change?: number
    positive?: boolean
    subtitle?: string
    icon?: string
}) {
    return (
        <div className="group p-5 rounded-lg transition-all duration-300 hover:translate-y-[-2px] relative overflow-hidden" style={{ background: 'linear-gradient(135deg, var(--panel-bg) 0%, color-mix(in srgb, var(--panel-bg-solid) 92%, transparent) 100%)', border: '1px solid var(--panel-border)' }}>
            <div className="absolute top-0 right-0 p-4 opacity-5 group-hover:opacity-10 transition-opacity text-4xl grayscale group-hover:grayscale-0">
                {icon}
            </div>
            <div className="text-xs mb-2 font-mono uppercase tracking-wider text-nofx-text-muted flex items-center gap-2">
                {title}
            </div>
            <div className="flex items-baseline gap-1 mb-1">
                <div className="text-2xl font-bold font-mono text-nofx-text-main tracking-tight transition-colors">
                    {value}
                </div>
                {unit && <span className="text-xs font-mono text-nofx-text-muted opacity-60">{unit}</span>}
            </div>

            {change !== undefined && (
                <div className="flex items-center gap-1">
                    <div
                        className={`text-sm mono font-bold flex items-center gap-1 ${positive ? 'text-nofx-green' : 'text-nofx-red'}`}
                    >
                        <span>{positive ? '▲' : '▼'}</span>
                        <span>{positive ? '+' : ''}{change.toFixed(2)}%</span>
                    </div>
                </div>
            )}
            {subtitle && (
                <div className="text-xs mt-2 mono text-nofx-text-muted opacity-80">
                    {subtitle}
                </div>
            )}
        </div>
    )
}
