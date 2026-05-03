import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { User, Cpu, Building2, MessageCircle, Eye, EyeOff, ChevronRight, Plus, Pencil, Server } from 'lucide-react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { api } from '../lib/api'
import { invalidateSystemConfig } from '../lib/config'
import { ExchangeConfigModal } from '../components/trader/ExchangeConfigModal'
import { TelegramConfigModal } from '../components/trader/TelegramConfigModal'
import { ModelConfigModal } from '../components/trader/ModelConfigModal'
import type { Exchange, AIModel, ProxyServer } from '../types'

type Tab = 'account' | 'models' | 'exchanges' | 'proxies' | 'telegram'

export function SettingsPage() {
  const { user } = useAuth()
  const { language } = useLanguage()
  const [activeTab, setActiveTab] = useState<Tab>('account')
  const isZh = language === 'zh'

  // Account state
  const [newPassword, setNewPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [changingPassword, setChangingPassword] = useState(false)
  const [registrationEnabled, setRegistrationEnabled] = useState(false)
  const [registrationLoading, setRegistrationLoading] = useState(false)
  const [registrationSaving, setRegistrationSaving] = useState(false)

  // AI Models state
  const [configuredModels, setConfiguredModels] = useState<AIModel[]>([])
  const [supportedModels, setSupportedModels] = useState<AIModel[]>([])
  const [showModelModal, setShowModelModal] = useState(false)
  const [editingModel, setEditingModel] = useState<string | null>(null)

  // Exchanges state
  const [exchanges, setExchanges] = useState<Exchange[]>([])
  const [showExchangeModal, setShowExchangeModal] = useState(false)
  const [editingExchange, setEditingExchange] = useState<string | null>(null)

  // Proxy servers state
  const [proxyServers, setProxyServers] = useState<ProxyServer[]>([])
  const [proxyName, setProxyName] = useState('')
  const [proxyUrl, setProxyUrl] = useState('')
  const [proxySaving, setProxySaving] = useState(false)
  const [proxySaveStatus, setProxySaveStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null)
  const [testingProxyId, setTestingProxyId] = useState<string | null>(null)

  // Telegram state
  const [showTelegramModal, setShowTelegramModal] = useState(false)

  // Fetch data when tabs are visited
  useEffect(() => {
    if (activeTab === 'account' && user?.email === 'admin@localhost') {
      setRegistrationLoading(true)
      fetch('/api/admin/registration', {
        headers: {
          Authorization: `Bearer ${localStorage.getItem('auth_token') || ''}`,
        },
      })
        .then(async (res) => {
          if (!res.ok) throw new Error('Failed to load registration setting')
          return res.json()
        })
        .then((data) => setRegistrationEnabled(Boolean(data.registration_enabled)))
        .catch((err) => toast.error(err instanceof Error ? err.message : 'Failed to load registration setting'))
        .finally(() => setRegistrationLoading(false))
    }
    if (activeTab === 'models') {
      Promise.all([api.getModelConfigs(), api.getSupportedModels()])
        .then(([configs, supported]) => {
          setConfiguredModels(configs)
          setSupportedModels(supported)
        })
        .catch(() => toast.error('Failed to load AI models'))
    }
    if (activeTab === 'exchanges') {
      Promise.all([api.getExchangeConfigs(), api.getProxyServers()])
        .then(([exchangeConfigs, proxyConfigs]) => {
          setExchanges(exchangeConfigs)
          setProxyServers(proxyConfigs)
        })
        .catch(() => toast.error('Failed to load exchanges'))
    }
    if (activeTab === 'proxies') {
      api.getProxyServers()
        .then(setProxyServers)
        .catch(() => toast.error('Failed to load proxy servers'))
    }
  }, [activeTab])

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword.length < 8) {
      toast.error('Password must be at least 8 characters')
      return
    }
    setChangingPassword(true)
    try {
      const res = await fetch('/api/user/password', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${localStorage.getItem('token') || ''}`,
        },
        body: JSON.stringify({ new_password: newPassword }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || 'Failed to update password')
      }
      toast.success('Password updated successfully')
      setNewPassword('')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update password')
    } finally {
      setChangingPassword(false)
    }
  }

  const handleRegistrationToggle = async () => {
    setRegistrationSaving(true)
    try {
      const nextValue = !registrationEnabled
      const res = await fetch('/api/admin/registration', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${localStorage.getItem('auth_token') || ''}`,
        },
        body: JSON.stringify({ registration_enabled: nextValue }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || 'Failed to update registration setting')
      }
      setRegistrationEnabled(nextValue)
      invalidateSystemConfig()
      toast.success(nextValue ? 'User registration enabled' : 'User registration disabled')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update registration setting')
    } finally {
      setRegistrationSaving(false)
    }
  }

  const handleSaveModel = async (
    modelId: string,
    apiKey: string,
    customApiUrl?: string,
    customModelName?: string
  ) => {
    try {
      const existingModel = configuredModels.find((m) => m.id === modelId)
      const modelTemplate = supportedModels.find((m) => m.id === modelId)
      const modelToUpdate = existingModel || modelTemplate
      if (!modelToUpdate) { toast.error('Model not found'); return }

      let updatedModels: AIModel[]
      if (existingModel) {
        updatedModels = configuredModels.map((m) =>
          m.id === modelId
            ? { ...m, apiKey, customApiUrl: customApiUrl || '', customModelName: customModelName || '', enabled: true }
            : m
        )
      } else {
        updatedModels = [...configuredModels, {
          ...modelToUpdate,
          apiKey,
          customApiUrl: customApiUrl || '',
          customModelName: customModelName || '',
          enabled: true,
        }]
      }

      const request = {
        models: Object.fromEntries(
          updatedModels.map((m) => [m.provider, {
            enabled: m.enabled,
            api_key: m.apiKey || '',
            custom_api_url: m.customApiUrl || '',
            custom_model_name: m.customModelName || '',
          }])
        ),
      }
      await toast.promise(api.updateModelConfigs(request), {
        loading: 'Saving model config...',
        success: 'Model config saved',
        error: 'Failed to save model config',
      })
      const refreshed = await api.getModelConfigs()
      setConfiguredModels(refreshed)
      setShowModelModal(false)
      setEditingModel(null)
    } catch {
      toast.error('Failed to save model config')
    }
  }

  const handleDeleteModel = async (modelId: string) => {
    try {
      const updatedModels = configuredModels.map((m) =>
        m.id === modelId ? { ...m, apiKey: '', customApiUrl: '', customModelName: '', enabled: false } : m
      )
      const request = {
        models: Object.fromEntries(
          updatedModels.map((m) => [m.provider, {
            enabled: m.enabled,
            api_key: m.apiKey || '',
            custom_api_url: m.customApiUrl || '',
            custom_model_name: m.customModelName || '',
          }])
        ),
      }
      await api.updateModelConfigs(request)
      const refreshed = await api.getModelConfigs()
      setConfiguredModels(refreshed)
      setShowModelModal(false)
      setEditingModel(null)
      toast.success('Model config removed')
    } catch {
      toast.error('Failed to remove model config')
    }
  }

  const refreshProxyServers = async () => {
    const refreshed = await api.getProxyServers()
    setProxyServers(refreshed)
  }

  const handleCreateProxyServer = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!proxyUrl.trim()) {
      toast.error(isZh ? '请填写代理地址' : 'Enter a proxy URL')
      return
    }
    setProxySaving(true)
    setProxySaveStatus(null)
    try {
      await toast.promise(
        api.createProxyServer({
          name: proxyName.trim() || (isZh ? '未命名代理' : 'Unnamed proxy'),
          proxy_url: proxyUrl.trim(),
        }),
        {
          loading: isZh ? '正在测试并创建代理...' : 'Testing and creating proxy...',
          success: isZh ? '代理服务器已创建' : 'Proxy server created',
          error: isZh ? '创建代理服务器失败' : 'Failed to create proxy server',
        }
      )
      setProxyName('')
      setProxyUrl('')
      await refreshProxyServers()
      setProxySaveStatus({
        type: 'success',
        message: isZh ? '测试通过，已保存' : 'Test passed and saved',
      })
    } catch {
      setProxySaveStatus({
        type: 'error',
        message: isZh ? '测试未通过' : 'Test failed',
      })
      toast.error(isZh ? '创建代理服务器失败' : 'Failed to create proxy server')
    } finally {
      setProxySaving(false)
    }
  }

  const handleTestSavedProxyServer = async (proxyId: string) => {
    setTestingProxyId(proxyId)
    try {
      const result = await api.testSavedProxyServer(proxyId)
      toast.success(isZh ? `代理测试通过，出口 IP：${result.public_ip}` : `Proxy test passed. Exit IP: ${result.public_ip}`)
      await refreshProxyServers()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : (isZh ? '代理测试失败' : 'Proxy test failed'))
      await refreshProxyServers()
    } finally {
      setTestingProxyId(null)
    }
  }

  const handleDeleteProxyServer = async (proxyId: string) => {
    try {
      await toast.promise(api.deleteProxyServer(proxyId), {
        loading: isZh ? '正在删除代理服务器...' : 'Deleting proxy server...',
        success: isZh ? '代理服务器已删除' : 'Proxy server deleted',
        error: isZh ? '删除代理服务器失败' : 'Failed to delete proxy server',
      })
      await refreshProxyServers()
    } catch {
      toast.error(isZh ? '删除代理服务器失败，确认没有交易所绑定该代理' : 'Failed to delete proxy server. Make sure no exchange is bound to it.')
    }
  }

  const handleSaveExchange = async (
    exchangeId: string | null,
    exchangeType: string,
    accountName: string,
    apiKey: string,
    proxyUrl?: string,
    secretKey?: string,
    passphrase?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string,
    lighterWalletAddr?: string,
    lighterPrivateKey?: string,
    lighterApiKeyPrivateKey?: string,
    lighterApiKeyIndex?: number,
    proxyServerId?: string
  ) => {
    try {
      if (exchangeId) {
        const request = {
          exchanges: {
            [exchangeId]: {
	              enabled: true,
	              ...(proxyUrl?.trim() ? { proxy_url: proxyUrl.trim() } : {}),
	              proxy_server_id: proxyServerId || '',
	              api_key: apiKey || '',
              secret_key: secretKey || '',
              passphrase: passphrase || '',
              testnet: testnet || false,
              hyperliquid_wallet_addr: hyperliquidWalletAddr || '',
              aster_user: asterUser || '',
              aster_signer: asterSigner || '',
              aster_private_key: asterPrivateKey || '',
              lighter_wallet_addr: lighterWalletAddr || '',
              lighter_private_key: lighterPrivateKey || '',
              lighter_api_key_private_key: lighterApiKeyPrivateKey || '',
              lighter_api_key_index: lighterApiKeyIndex || 0,
            },
          },
        }
        await toast.promise(api.updateExchangeConfigsEncrypted(request), {
          loading: 'Updating exchange config...',
          success: 'Exchange config updated',
          error: 'Failed to update exchange config',
        })
      } else {
        const createRequest = {
          exchange_type: exchangeType,
          account_name: accountName,
	          enabled: true,
	          proxy_url: proxyUrl?.trim() || '',
	          proxy_server_id: proxyServerId || '',
	          api_key: apiKey || '',
          secret_key: secretKey || '',
          passphrase: passphrase || '',
          testnet: testnet || false,
          hyperliquid_wallet_addr: hyperliquidWalletAddr || '',
          aster_user: asterUser || '',
          aster_signer: asterSigner || '',
          aster_private_key: asterPrivateKey || '',
          lighter_wallet_addr: lighterWalletAddr || '',
          lighter_private_key: lighterPrivateKey || '',
          lighter_api_key_private_key: lighterApiKeyPrivateKey || '',
          lighter_api_key_index: lighterApiKeyIndex || 0,
        }
        await toast.promise(api.createExchangeEncrypted(createRequest), {
          loading: 'Creating exchange account...',
          success: 'Exchange account created',
          error: 'Failed to create exchange account',
        })
      }
	      const refreshed = await api.getExchangeConfigs()
	      setExchanges(refreshed)
	      await refreshProxyServers()
      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch {
      toast.error('Failed to save exchange config')
    }
  }

  const handleDeleteExchange = async (exchangeId: string) => {
    try {
      await toast.promise(api.deleteExchange(exchangeId), {
        loading: 'Deleting exchange account...',
        success: 'Exchange account deleted',
        error: 'Failed to delete exchange account',
      })
      const refreshed = await api.getExchangeConfigs()
      setExchanges(refreshed)
      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch {
      toast.error('Failed to delete exchange account')
    }
  }

  const tabs: { key: Tab; label: string; icon: React.ReactNode }[] = [
    { key: 'account', label: isZh ? '账户' : 'Account', icon: <User size={16} /> },
    { key: 'models', label: isZh ? 'AI 模型' : 'AI Models', icon: <Cpu size={16} /> },
    { key: 'exchanges', label: isZh ? '交易所' : 'Exchanges', icon: <Building2 size={16} /> },
    { key: 'proxies', label: isZh ? '代理服务器' : 'Proxies', icon: <Server size={16} /> },
    { key: 'telegram', label: 'Telegram', icon: <MessageCircle size={16} /> },
  ]

  return (
    <div className="min-h-screen pt-20 pb-12 px-4">
      <div className="max-w-2xl mx-auto">
        <h1 className="text-xl font-bold mb-6" style={{ color: 'var(--text-primary)' }}>
          {isZh ? '设置' : 'Settings'}
        </h1>

        {/* Tabs */}
        <div
          className="flex gap-1 mb-6 rounded-xl p-1"
          style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}
        >
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-all
                ${activeTab === tab.key
                  ? 'bg-nofx-gold text-black'
                  : 'hover:text-[var(--text-primary)]'
                }`}
              style={activeTab === tab.key ? undefined : { color: 'var(--text-secondary)' }}
            >
              {tab.icon}
              <span className="hidden sm:inline">{tab.label}</span>
            </button>
          ))}
        </div>

        {/* Tab Content */}
        <div
          className="backdrop-blur-xl rounded-2xl p-6"
          style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}
        >

          {/* Account Tab */}
          {activeTab === 'account' && (
            <div className="space-y-6">
              <div>
                <p className="text-xs mb-1" style={{ color: 'var(--text-tertiary)' }}>Email</p>
                <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{user?.email}</p>
              </div>

              {user?.email === 'admin@localhost' && (
                <div className="pt-6" style={{ borderTop: '1px solid var(--panel-border)' }}>
                  <h3 className="text-sm font-semibold mb-4" style={{ color: 'var(--text-primary)' }}>
                    {isZh ? '注册开关' : 'Registration Access'}
                  </h3>
                  <div className="rounded-xl p-4" style={{ border: '1px solid var(--panel-border)', background: 'var(--panel-bg-solid)' }}>
                    <div className="flex items-center justify-between gap-4">
                      <div>
                        <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                          {isZh ? '新用户注册' : 'New user registration'}
                        </p>
                        <p className="text-xs mt-1" style={{ color: 'var(--text-tertiary)' }}>
                          {isZh ? '控制 `/register` 是否接受新用户注册。' : 'Control whether `/register` accepts new user signups.'}
                        </p>
                      </div>
                      <button
                        type="button"
                        onClick={handleRegistrationToggle}
                        disabled={registrationLoading || registrationSaving}
                        className={`min-w-[112px] rounded-lg px-4 py-2 text-sm font-semibold transition-all disabled:opacity-50 disabled:cursor-not-allowed ${
                          registrationEnabled
                            ? 'bg-emerald-500/15 text-emerald-400 hover:bg-emerald-500/25'
                            : 'hover:opacity-80'
                        }`}
                        style={registrationEnabled ? undefined : { background: 'var(--panel-bg-solid)', color: 'var(--text-secondary)' }}
                      >
                        {registrationLoading
                          ? (isZh ? '加载中...' : 'Loading...')
                          : registrationSaving
                            ? (isZh ? '保存中...' : 'Saving...')
                            : registrationEnabled
                              ? (isZh ? '已开启' : 'Enabled')
                              : (isZh ? '已关闭' : 'Disabled')}
                      </button>
                    </div>
                  </div>
                </div>
              )}

              <div className="pt-6" style={{ borderTop: '1px solid var(--panel-border)' }}>
                <h3 className="text-sm font-semibold mb-4" style={{ color: 'var(--text-primary)' }}>
                  {isZh ? '修改密码' : 'Change Password'}
                </h3>
                <form onSubmit={handleChangePassword} className="space-y-4">
                  <div>
                    <label className="block text-xs font-medium mb-2" style={{ color: 'var(--text-secondary)' }}>
                      {isZh ? '新密码' : 'New Password'}
                    </label>
                    <div className="relative">
                      <input
                        type={showPassword ? 'text' : 'password'}
                        value={newPassword}
                        onChange={(e) => setNewPassword(e.target.value)}
                        className="w-full rounded-xl px-4 py-3 pr-11 text-sm focus:outline-none focus:border-nofx-gold/60 focus:ring-1 focus:ring-nofx-gold/30 transition-all"
                        style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                        placeholder={isZh ? '至少 8 位字符' : 'At least 8 characters'}
                        required
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="absolute right-3.5 top-1/2 -translate-y-1/2 transition-colors"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                      </button>
                    </div>
                  </div>
                  <button
                    type="submit"
                    disabled={changingPassword || newPassword.length < 8}
                    className="w-full bg-nofx-gold hover:bg-yellow-400 active:scale-[0.98] text-black font-semibold py-3 rounded-xl text-sm transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {changingPassword ? (isZh ? '更新中...' : 'Updating...') : (isZh ? '更新密码' : 'Update Password')}
                  </button>
                </form>
              </div>
            </div>
          )}

          {/* AI Models Tab */}
          {activeTab === 'models' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                  {isZh
                    ? `已配置 ${configuredModels.length} 个模型`
                    : `${configuredModels.length} model${configuredModels.length !== 1 ? 's' : ''} configured`}
                </p>
                <button
                  onClick={() => { setEditingModel(null); setShowModelModal(true) }}
                  className="flex items-center gap-1.5 text-xs font-medium bg-nofx-gold/10 hover:bg-nofx-gold/20 text-nofx-gold px-3 py-1.5 rounded-lg transition-colors"
                >
                  <Plus size={14} />
                  {isZh ? '新增模型' : 'Add Model'}
                </button>
              </div>

              {configuredModels.length === 0 ? (
                <div className="text-center py-8 text-sm" style={{ color: 'var(--text-tertiary)' }}>
                  {isZh ? '尚未配置 AI 模型' : 'No AI models configured yet'}
                </div>
              ) : (
                <div className="space-y-2">
                  {configuredModels.map((model) => (
                    <button
                      key={model.id}
                      onClick={() => { setEditingModel(model.id); setShowModelModal(true) }}
                      className="w-full flex items-center justify-between px-4 py-3 rounded-xl transition-colors group"
                      style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}
                    >
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: 'var(--panel-bg)' }}>
                          <Cpu size={14} style={{ color: 'var(--text-secondary)' }} />
                        </div>
                        <div className="text-left">
                          <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{model.name}</p>
                          <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>{model.provider}</p>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className={`text-xs px-2 py-0.5 rounded-full ${model.enabled ? 'bg-emerald-500/10 text-emerald-400' : 'bg-zinc-700 text-zinc-500'}`}>
                          {model.enabled ? (isZh ? '启用' : 'Active') : (isZh ? '未启用' : 'Inactive')}
                        </span>
                        <Pencil size={14} className="transition-colors" style={{ color: 'var(--text-tertiary)' }} />
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Exchanges Tab */}
          {activeTab === 'exchanges' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                  {isZh
                    ? `已连接 ${exchanges.length} 个账户`
                    : `${exchanges.length} account${exchanges.length !== 1 ? 's' : ''} connected`}
                </p>
                <button
                  onClick={() => { setEditingExchange(null); setShowExchangeModal(true) }}
                  className="flex items-center gap-1.5 text-xs font-medium bg-nofx-gold/10 hover:bg-nofx-gold/20 text-nofx-gold px-3 py-1.5 rounded-lg transition-colors"
                >
                  <Plus size={14} />
                  {isZh ? '新增交易所' : 'Add Exchange'}
                </button>
              </div>

              {exchanges.length === 0 ? (
                <div className="text-center py-8 text-sm" style={{ color: 'var(--text-tertiary)' }}>
                  {isZh ? '尚未连接交易所账户' : 'No exchange accounts connected yet'}
                </div>
              ) : (
                <div className="space-y-2">
                  {exchanges.map((exchange) => (
                    <button
                      key={exchange.id}
                      onClick={() => { setEditingExchange(exchange.id); setShowExchangeModal(true) }}
                      className="w-full flex items-center justify-between px-4 py-3 rounded-xl transition-colors group"
                      style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}
                    >
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: 'var(--panel-bg)' }}>
                          <Building2 size={14} style={{ color: 'var(--text-secondary)' }} />
                        </div>
                        <div className="text-left">
                          <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{exchange.account_name || exchange.name}</p>
                          <p className="text-xs capitalize" style={{ color: 'var(--text-tertiary)' }}>{exchange.exchange_type || exchange.type}</p>
                        </div>
                      </div>
                      <ChevronRight size={14} className="transition-colors" style={{ color: 'var(--text-tertiary)' }} />
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Proxy Servers Tab */}
          {activeTab === 'proxies' && (
            <div className="space-y-5">
              <div>
                <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                  {isZh
                    ? `已配置 ${proxyServers.length} 个代理服务器。一个交易所 API 只能绑定一个代理服务器。`
                    : `${proxyServers.length} proxy server${proxyServers.length !== 1 ? 's' : ''} configured. One exchange API can bind one proxy server.`}
                </p>
              </div>

              <form onSubmit={handleCreateProxyServer} className="space-y-3 rounded-xl p-4" style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}>
                <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                  {isZh ? '新增代理服务器' : 'Add Proxy Server'}
                </h3>
                <input
                  type="text"
                  value={proxyName}
                  onChange={(e) => setProxyName(e.target.value)}
                  placeholder={isZh ? '代理名称，例如：OKX 代理 1' : 'Proxy name, e.g. OKX Proxy 1'}
                  className="w-full rounded-xl px-4 py-3 text-sm focus:outline-none focus:border-nofx-gold/60"
                  style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                />
                <input
                  type="text"
                  value={proxyUrl}
                  onChange={(e) => setProxyUrl(e.target.value)}
                  placeholder="socks5://user:pass@1.2.3.4:1080"
                  className="w-full rounded-xl px-4 py-3 text-sm font-mono focus:outline-none focus:border-nofx-gold/60"
                  style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                  required
                />
                <button
                  type="submit"
                  disabled={proxySaving}
                  className="w-full bg-nofx-gold hover:bg-yellow-400 active:scale-[0.98] text-black font-semibold py-3 rounded-xl text-sm transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {proxySaving ? (isZh ? '测试并保存中...' : 'Testing and saving...') : (isZh ? '测试并保存代理' : 'Test and Save Proxy')}
                </button>
                {proxySaveStatus && (
                  <p
                    className="text-sm font-semibold"
                    style={{ color: proxySaveStatus.type === 'success' ? '#0ECB81' : '#F6465D' }}
                  >
                    {proxySaveStatus.message}
                  </p>
                )}
	                <p className="text-xs leading-5" style={{ color: 'var(--text-tertiary)' }}>
	                  {isZh ? '保存前会实际走代理访问公网 IP 查询接口；测试失败不会保存。' : 'The proxy is tested against a public IP endpoint before saving; failed tests are not saved.'}
	                </p>
	                <div
	                  className="rounded-xl px-4 py-3 text-xs leading-6"
	                  style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', color: 'var(--text-secondary)' }}
	                >
	                  {isZh ? (
	                    <>
	                      <div className="font-semibold" style={{ color: 'var(--text-primary)' }}>代理创建方式</div>
	                      <div>
	                        1. 购买一台云主机（
	                        <a
	                          href="https://cn.aliyun.com/product/swas?from_alibabacloud=&spm=5176.32270579.nav-v2-dropdown-menu-1.d_main_2_0_4.1c0b3aa8keVDeQ&scm=20140722.X_data-d4b68a29ba28f53e56fa._.V_1"
	                          target="_blank"
	                          rel="noreferrer"
	                          className="underline"
	                          style={{ color: 'var(--text-primary)' }}
	                        >
	                          阿里云轻量应用服务器
	                        </a>
	                        ）。
	                      </div>
	                      <div>2. 开放端口 1080。</div>
	                      <div>3. 运行代码：</div>
	                      <div>先下载：</div>
	                      <div className="font-mono break-all">wget https://raw.githubusercontent.com/qidoulij006/qt-proxy-installer/main/install-proxy-from-github.sh -O qt-install.sh</div>
	                      <div>检查安全性：</div>
	                      <div className="font-mono break-all">cat qt-install.sh</div>
	                      <div>安装：</div>
	                      <div className="font-mono break-all">chmod +x qt-install.sh</div>
	                      <div>运行：</div>
	                      <div className="font-mono break-all">sudo ./qt-install.sh</div>
	                      <div>脚本会自动使用用户名 afx、端口 1080、地址 0.0.0.0，并自动生成代理密码。</div>
	                      <div>4. 当显示 <span className="font-mono">Status: active</span>，并输出 <span className="font-mono">AFX proxy_url:</span>，则安装成功。</div>
	                      <div>复制 <span className="font-mono">proxy_url:</span> 后面的内容到上面的代理地址输入框。</div>
	                      <div className="mt-2" style={{ color: '#F6465D' }}>注意事项：本代理只可以用于量化网站的实盘测试，不可用于其它用途；如有违背，后果自负。</div>
	                    </>
	                  ) : (
	                    <>
	                      <div className="font-semibold" style={{ color: 'var(--text-primary)' }}>How to create the proxy</div>
	                      <div>
	                        1. Buy a cloud server (
	                        <a
	                          href="https://cn.aliyun.com/product/swas?from_alibabacloud=&spm=5176.32270579.nav-v2-dropdown-menu-1.d_main_2_0_4.1c0b3aa8keVDeQ&scm=20140722.X_data-d4b68a29ba28f53e56fa._.V_1"
	                          target="_blank"
	                          rel="noreferrer"
	                          className="underline"
	                          style={{ color: 'var(--text-primary)' }}
	                        >
	                          Alibaba Cloud Simple Application Server
	                        </a>
	                        ).
	                      </div>
	                      <div>2. Open port 1080.</div>
	                      <div>3. Run the setup flow:</div>
	                      <div>Download:</div>
	                      <div className="font-mono break-all">wget https://raw.githubusercontent.com/qidoulij006/qt-proxy-installer/main/install-proxy-from-github.sh -O qt-install.sh</div>
	                      <div>Inspect the script:</div>
	                      <div className="font-mono break-all">cat qt-install.sh</div>
	                      <div>Make it executable:</div>
	                      <div className="font-mono break-all">chmod +x qt-install.sh</div>
	                      <div>Run it:</div>
	                      <div className="font-mono break-all">sudo ./qt-install.sh</div>
	                      <div>The script automatically uses username afx, port 1080, bind address 0.0.0.0, and generates the proxy password.</div>
	                      <div>4. When it shows <span className="font-mono">Status: active</span> and prints <span className="font-mono">AFX proxy_url:</span>, the proxy is ready.</div>
	                      <div>Copy the value after <span className="font-mono">proxy_url:</span> into the proxy URL field above.</div>
	                      <div className="mt-2" style={{ color: '#F6465D' }}>Notice: This proxy may only be used for live trading tests on the quantitative trading website. Do not use it for other purposes; violations are at your own risk.</div>
	                    </>
	                  )}
	                </div>
	              </form>

              <div className="space-y-2">
                {proxyServers.length === 0 ? (
                  <div className="text-center py-8 text-sm" style={{ color: 'var(--text-tertiary)' }}>
                    {isZh ? '尚未配置代理服务器' : 'No proxy servers configured yet'}
                  </div>
                ) : (
                  proxyServers.map((proxy) => (
                    <div
                      key={proxy.id}
                      className="rounded-xl px-4 py-3"
                      style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{proxy.name}</p>
                          <p className="text-xs mt-1" style={{ color: 'var(--text-tertiary)' }}>
                            {isZh ? '出口 IP：' : 'Exit IP: '}
                            <span className="font-mono">{proxy.last_exit_ip || '-'}</span>
                          </p>
                          <p className="text-xs mt-1" style={{ color: 'var(--text-tertiary)' }}>
                            {proxy.bound_exchange_name
                              ? (isZh ? `已绑定：${proxy.bound_exchange_name}` : `Bound: ${proxy.bound_exchange_name}`)
                              : (isZh ? '未绑定交易所' : 'Unbound')}
                          </p>
                        </div>
                        <div className="flex items-center gap-2">
                          <button
                            type="button"
                            onClick={() => handleTestSavedProxyServer(proxy.id)}
                            disabled={testingProxyId === proxy.id}
                            className="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-50"
                            style={{ background: 'rgba(240, 185, 11, 0.12)', color: '#F0B90B' }}
                          >
                            {testingProxyId === proxy.id ? (isZh ? '测试中' : 'Testing') : (isZh ? '测试' : 'Test')}
                          </button>
                          <button
                            type="button"
                            onClick={() => handleDeleteProxyServer(proxy.id)}
                            disabled={Boolean(proxy.bound_exchange_id)}
                            className="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                            style={{ background: 'rgba(246, 70, 93, 0.12)', color: '#F6465D' }}
                          >
                            {isZh ? '删除' : 'Delete'}
                          </button>
                        </div>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          )}

          {/* Telegram Tab */}
          {activeTab === 'telegram' && (
            <div className="space-y-4">
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                {isZh
                  ? '连接 Telegram Bot，以接收交易通知并与交易员交互。'
                  : 'Connect a Telegram bot to receive trading notifications and interact with your traders.'}
              </p>
              <button
                onClick={() => setShowTelegramModal(true)}
                className="w-full flex items-center justify-between px-4 py-3 rounded-xl transition-colors group"
                style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}
              >
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-lg bg-[#0088cc]/20 flex items-center justify-center">
                    <MessageCircle size={14} className="text-[#0088cc]" />
                  </div>
                  <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                    {isZh ? '配置 Telegram Bot' : 'Configure Telegram Bot'}
                  </span>
                </div>
                <ChevronRight size={14} className="transition-colors" style={{ color: 'var(--text-tertiary)' }} />
              </button>
            </div>
          )}
        </div>
      </div>

      {/* AI Model Modal */}
      {showModelModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm px-4">
          <ModelConfigModal
            allModels={supportedModels}
            configuredModels={configuredModels}
            editingModelId={editingModel}
            onSave={handleSaveModel}
            onDelete={handleDeleteModel}
            onClose={() => { setShowModelModal(false); setEditingModel(null) }}
            language={language}
          />
        </div>
      )}

      {/* Exchange Modal */}
      {showExchangeModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm px-4">
	          <ExchangeConfigModal
	            allExchanges={exchanges}
	            proxyServers={proxyServers}
	            editingExchangeId={editingExchange}
            onSave={handleSaveExchange}
            onDelete={handleDeleteExchange}
            onClose={() => { setShowExchangeModal(false); setEditingExchange(null) }}
            language={language}
          />
        </div>
      )}

      {/* Telegram Modal */}
      {showTelegramModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm px-4">
          <TelegramConfigModal
            onClose={() => setShowTelegramModal(false)}
            language={language}
          />
        </div>
      )}
    </div>
  )
}
