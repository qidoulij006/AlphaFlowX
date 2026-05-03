import { useState } from 'react'
import { Activity, BarChart3, BrainCircuit, LineChart, ShieldCheck, Sparkles, Zap } from 'lucide-react'
import HeaderBar from '../components/common/HeaderBar'
import LoginModal from '../components/landing/LoginModal'
import { LoginRequiredOverlay } from '../components/auth/LoginRequiredOverlay'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'

const metrics = [
  { label: 'AI 信号 / AI Signals', value: '24/7' },
  { label: '实时风控 / Risk Engine', value: '实时 / Live' },
  { label: '多市场 / Markets', value: '多市场 / Multi' },
]

const capabilities = [
  {
    icon: BrainCircuit,
    title: 'AI 决策引擎 / AI Decision Engine',
    desc: '大模型推理把行情、持仓和风控规则转化为可审计的交易决策。Large-model reasoning turns market data, positions, and risk rules into auditable trading decisions.',
  },
  {
    icon: BarChart3,
    title: '量化策略工作台 / Quant Strategy Studio',
    desc: '无需代码即可配置币种来源、技术指标、杠杆、保证金限制、提示词 Prompt 和网格参数。Configure coin sources, indicators, leverage, margin limits, prompts, and grid parameters without code.',
  },
  {
    icon: ShieldCheck,
    title: '受控交易执行 / Controlled Execution',
    desc: '交易所密钥保留在你的部署中，同时强制执行仓位规模、置信度门槛和风险限制。Exchange keys stay in your deployment while position sizing, confidence gates, and risk limits stay enforced.',
  },
]

const signalRows = [
  { symbol: 'BTCUSDT', side: '做多 / LONG', score: 82, color: '#0ECB81' },
  { symbol: 'ETHUSDT', side: '观察 / WATCH', score: 67, color: '#F0B90B' },
  { symbol: 'SOLUSDT', side: '降风险 / RISK OFF', score: 41, color: '#F6465D' },
]

export function LandingPage() {
  const [showLoginModal, setShowLoginModal] = useState(false)
  const [loginOverlayOpen, setLoginOverlayOpen] = useState(false)
  const [loginOverlayFeature, setLoginOverlayFeature] = useState('')
  const { user, logout } = useAuth()
  const { language, setLanguage } = useLanguage()
  const isLoggedIn = !!user

  const handleLoginRequired = (featureName: string) => {
    setLoginOverlayFeature(featureName)
    setLoginOverlayOpen(true)
  }

  return (
    <>
      <HeaderBar
        onLoginClick={() => setShowLoginModal(true)}
        isLoggedIn={isLoggedIn}
        isHomePage={true}
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onLoginRequired={handleLoginRequired}
        onPageChange={(page) => {
          const pathMap: Record<string, string> = {
            'data': '/data',
            'competition': '/competition',
            'strategy-market': '/strategy-market',
            'traders': '/traders',
            'trader': '/dashboard',
            'strategy': '/strategy',
            'faq': '/faq',
          }
          const path = pathMap[page]
          if (path) {
            window.location.href = path
          }
        }}
      />
      <div className="min-h-screen bg-nofx-bg text-nofx-text font-sans selection:bg-nofx-gold selection:text-black">
        <main className="relative overflow-hidden pt-24">
          <section className="mx-auto flex min-h-[calc(100vh-6rem)] w-full max-w-7xl flex-col justify-center px-5 py-10 sm:px-8 lg:px-10">
            <div className="grid items-center gap-10 lg:grid-cols-[1fr_0.88fr]">
              <div className="max-w-3xl">
                <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-[rgba(240,185,11,0.28)] bg-[rgba(240,185,11,0.08)] px-4 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-nofx-gold">
                  <Sparkles className="h-4 w-4" />
                  AI 量化交易系统 / AI Quant Trading OS
                </div>
                <h1 className="text-4xl font-bold leading-tight text-[var(--text-primary)] sm:text-5xl lg:text-6xl">
                  AlphaFlowX
                  <span className="mt-3 block text-2xl font-semibold text-[var(--text-secondary)] sm:text-3xl lg:text-4xl">
                    AI 驱动的量化交易工作台
                  </span>
                </h1>
                <p className="mt-6 max-w-2xl text-base leading-8 text-[var(--text-secondary)] sm:text-lg">
                  把策略、行情、风控和交易执行放到一个清晰的系统里。用 AI 分析多周期市场数据，
                  用量化规则约束仓位和风险，再把每一次决策沉淀为可复盘的交易记录。
                </p>

                <div className="mt-8 flex flex-col gap-3 sm:flex-row">
                  <button
                    onClick={() => { window.location.href = '/login' }}
                    className="inline-flex items-center justify-center gap-2 rounded-lg bg-nofx-gold px-6 py-3 text-sm font-bold text-black transition hover:bg-yellow-400"
                  >
                    <Zap className="h-4 w-4" />
                    进入控制台
                  </button>
                  <button
                    onClick={() => { window.location.href = '/strategy-market' }}
                    className="inline-flex items-center justify-center gap-2 rounded-lg border border-[var(--panel-border)] bg-[var(--panel-bg)] px-6 py-3 text-sm font-semibold text-[var(--text-primary)] transition hover:border-[var(--panel-border-hover)]"
                  >
                    <LineChart className="h-4 w-4" />
                    查看策略市场
                  </button>
                </div>

                <div className="mt-10 grid max-w-xl grid-cols-3 gap-3">
                  {metrics.map((item) => (
                    <div key={item.label} className="rounded-lg border border-[var(--panel-border)] bg-[var(--panel-bg)] p-4">
                      <div className="text-xl font-bold text-[var(--text-primary)]">{item.value}</div>
                      <div className="mt-1 text-xs text-[var(--text-secondary)]">{item.label}</div>
                    </div>
                  ))}
                </div>
              </div>

              <div className="rounded-xl border border-[rgba(240,185,11,0.22)] bg-[var(--panel-bg)] p-4 shadow-[var(--shadow-lg)] backdrop-blur">
                <div className="mb-4 flex items-center justify-between">
                  <div>
                    <div className="text-sm font-semibold text-[var(--text-primary)]">AI 市场控制台 / AI Market Console</div>
                    <div className="text-xs text-[var(--text-secondary)]">信号 · 风控 · 执行 / Signals · Risk · Execution</div>
                  </div>
                  <div className="flex items-center gap-2 rounded-full bg-[rgba(14,203,129,0.12)] px-3 py-1 text-xs font-semibold text-[var(--binance-green)]">
                    <Activity className="h-3.5 w-3.5" />
                    实时 / Live
                  </div>
                </div>

                <div className="rounded-lg border border-[var(--panel-border)] bg-[var(--panel-bg-solid)] p-4">
                  <div className="mb-4 flex items-center justify-between text-xs text-[var(--text-secondary)]">
                    <span>决策流 / Decision Flow</span>
                    <span>均衡策略 / Balanced Profile</span>
                  </div>
                  <div className="h-28 rounded-md border border-[var(--panel-border)] bg-[linear-gradient(180deg,rgba(14,203,129,0.14),rgba(240,185,11,0.06))] p-4">
                    <svg viewBox="0 0 420 96" className="h-full w-full" role="img" aria-label="量化信号曲线 / Quant signal curve">
                      <polyline
                        points="0,70 42,62 84,68 126,42 168,49 210,30 252,36 294,18 336,28 378,16 420,24"
                        fill="none"
                        stroke="#0ECB81"
                        strokeWidth="4"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                      <polyline
                        points="0,80 42,77 84,74 126,73 168,66 210,61 252,55 294,52 336,46 378,44 420,39"
                        fill="none"
                        stroke="#F0B90B"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeDasharray="6 8"
                      />
                    </svg>
                  </div>

                  <div className="mt-4 space-y-3">
                    {signalRows.map((row) => (
                      <div key={row.symbol} className="flex items-center justify-between rounded-md bg-[var(--panel-bg)] px-3 py-3">
                        <div>
                          <div className="text-sm font-semibold text-[var(--text-primary)]">{row.symbol}</div>
                          <div className="text-xs text-[var(--text-secondary)]">模型置信度 / model confidence {row.score}%</div>
                        </div>
                        <span className="rounded-full px-3 py-1 text-xs font-bold" style={{ background: `${row.color}1F`, color: row.color }}>
                          {row.side}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section className="mx-auto grid w-full max-w-7xl gap-4 px-5 pb-16 sm:px-8 lg:grid-cols-3 lg:px-10">
            {capabilities.map(({ icon: Icon, title, desc }) => (
              <div key={title} className="rounded-xl border border-[var(--panel-border)] bg-[var(--panel-bg)] p-6">
                <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-lg bg-[rgba(240,185,11,0.12)] text-nofx-gold">
                  <Icon className="h-5 w-5" />
                </div>
                <h2 className="text-lg font-bold text-[var(--text-primary)]">{title}</h2>
                <p className="mt-3 text-sm leading-6 text-[var(--text-secondary)]">{desc}</p>
              </div>
            ))}
          </section>

          <footer className="border-t border-[var(--panel-border)] px-5 py-6 text-center text-xs leading-6 text-[var(--text-secondary)]">
            <div>AlphaFlowX is non-custodial trading software. Trading involves risk.</div>
            <div>AlphaFlowX 是非托管交易软件。交易存在风险，请谨慎使用。</div>
            <div>
              <a
                href="https://github.com/qidoulij006/AlphaFlowX"
                target="_blank"
                rel="noreferrer"
                className="font-medium text-nofx-gold hover:underline"
              >
                AGPL-3.0 Source from nofx / 基于 nofx 二次开发后的源码
              </a>
            </div>
          </footer>
        </main>

        {showLoginModal && (
          <LoginModal
            onClose={() => setShowLoginModal(false)}
            language={language}
          />
        )}

        <LoginRequiredOverlay
          isOpen={loginOverlayOpen}
          onClose={() => setLoginOverlayOpen(false)}
          featureName={loginOverlayFeature}
        />
      </div>
    </>
  )
}
