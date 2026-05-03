import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { api } from '../../lib/api'
import type {
  TraderInfo,
  CreateTraderRequest,
	  AIModel,
	  Exchange,
	  ProxyServer,
	} from '../../types'
import { useLanguage } from '../../contexts/LanguageContext'
import { t } from '../../i18n/translations'
import { useAuth } from '../../contexts/AuthContext'
import { TraderConfigModal } from './TraderConfigModal'
import { DeepVoidBackground } from '../common/DeepVoidBackground'
import { ExchangeConfigModal } from './ExchangeConfigModal'
import { TelegramConfigModal } from './TelegramConfigModal'
import { ModelConfigModal } from './ModelConfigModal'
import { ConfigStatusGrid } from './ConfigStatusGrid'
import { TradersList } from './TradersList'
import {
  Bot,
  Plus,
  MessageCircle,
  Server,
  Trash2,
} from 'lucide-react'
import { confirmToast } from '../../lib/notify'
import { toast } from 'sonner'
import { filterVisibleModels } from './model-constants'

interface AITradersPageProps {
  onTraderSelect?: (traderId: string) => void
}

function buildModelConfigRequest(models: AIModel[]) {
  return {
    models: Object.fromEntries(
      models.map((model) => [
        model.id,
        {
          name: model.name,
          provider: model.provider,
          enabled: model.enabled,
          api_key: model.apiKey || '',
          custom_api_url: model.customApiUrl || '',
          custom_model_name: model.customModelName || '',
        },
      ])
    ),
  }
}

export function AITradersPage({ onTraderSelect }: AITradersPageProps) {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  const navigate = useNavigate()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [showModelModal, setShowModelModal] = useState(false)
  const [showExchangeModal, setShowExchangeModal] = useState(false)
  const [showProxyModal, setShowProxyModal] = useState(false)
  const [showTelegramModal, setShowTelegramModal] = useState(false)
  const [editingModel, setEditingModel] = useState<string | null>(null)
  const [editingExchange, setEditingExchange] = useState<string | null>(null)
  const [editingTrader, setEditingTrader] = useState<any>(null)
	  const [allModels, setAllModels] = useState<AIModel[]>([])
	  const [allExchanges, setAllExchanges] = useState<Exchange[]>([])
	  const [proxyServers, setProxyServers] = useState<ProxyServer[]>([])
	  const [supportedModels, setSupportedModels] = useState<AIModel[]>([])
  const [proxyName, setProxyName] = useState('')
  const [proxyUrl, setProxyUrl] = useState('')
  const [proxySaving, setProxySaving] = useState(false)
  const [proxySaveStatus, setProxySaveStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null)
  const [testingProxyId, setTestingProxyId] = useState<string | null>(null)
  const [visibleTraderAddresses, setVisibleTraderAddresses] = useState<Set<string>>(new Set())
  const [visibleExchangeAddresses, setVisibleExchangeAddresses] = useState<Set<string>>(new Set())
  const [copiedId, setCopiedId] = useState<string | null>(null)

  // Toggle wallet address visibility for a trader
  const toggleTraderAddressVisibility = (traderId: string) => {
    setVisibleTraderAddresses(prev => {
      const next = new Set(prev)
      if (next.has(traderId)) {
        next.delete(traderId)
      } else {
        next.add(traderId)
      }
      return next
    })
  }

  // Toggle wallet address visibility for an exchange
  const toggleExchangeAddressVisibility = (exchangeId: string) => {
    setVisibleExchangeAddresses(prev => {
      const next = new Set(prev)
      if (next.has(exchangeId)) {
        next.delete(exchangeId)
      } else {
        next.add(exchangeId)
      }
      return next
    })
  }

  // Copy wallet address to clipboard
  const handleCopyAddress = async (id: string, address: string) => {
    try {
      await navigator.clipboard.writeText(address)
      setCopiedId(id)
      setTimeout(() => setCopiedId(null), 2000)
    } catch (err) {
      console.error('Failed to copy address:', err)
    }
  }

  const { data: traders, mutate: mutateTraders, isLoading: isTradersLoading } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    api.getTraders,
    { refreshInterval: 5000 }
  )

  useEffect(() => {
    const loadConfigs = async () => {
      if (!user || !token) {
        try {
          const models = await api.getSupportedModels()
          setSupportedModels(filterVisibleModels(models))
        } catch (err) {
          console.error('Failed to load supported configs:', err)
        }
        return
      }

      try {
	        const [
	          modelConfigs,
	          exchangeConfigs,
	          proxyConfigs,
	          models,
	        ] = await Promise.all([
	          api.getModelConfigs(),
	          api.getExchangeConfigs(),
	          api.getProxyServers(),
	          api.getSupportedModels(),
	        ])
	        setAllModels(filterVisibleModels(modelConfigs))
	        setAllExchanges(exchangeConfigs)
	        setProxyServers(proxyConfigs)
	        setSupportedModels(filterVisibleModels(models))
      } catch (error) {
        console.error('Failed to load configs:', error)
      }
    }
    loadConfigs()
  }, [user, token])

  const configuredModels =
    allModels?.filter((m) => {
      return m.enabled || (m.customApiUrl && m.customApiUrl.trim() !== '')
    }) || []

  const configuredExchanges =
    allExchanges?.filter((e) => {
      if (e.id === 'aster') {
        return e.asterUser && e.asterUser.trim() !== ''
      }
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
      }
      return e.enabled
    }) || []

  const enabledModels = allModels?.filter((m) => m.enabled) || []
  const enabledExchanges =
    allExchanges?.filter((e) => {
      if (!e.enabled) return false
      if (e.id === 'aster') {
        return (
          e.asterUser &&
          e.asterUser.trim() !== '' &&
          e.asterSigner &&
          e.asterSigner.trim() !== ''
        )
      }
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
      }
      return true
    }) || []

  const isModelInUse = (modelId: string) => {
    return traders?.some((tr) => tr.ai_model === modelId && tr.is_running)
  }

  const getModelUsageInfo = (modelId: string) => {
    const usingTraders = traders?.filter((tr) => tr.ai_model === modelId) || []
    const runningCount = usingTraders.filter((tr) => tr.is_running).length
    const totalCount = usingTraders.length
    return { runningCount, totalCount, usingTraders }
  }

  const isExchangeInUse = (exchangeId: string) => {
    return traders?.some((tr) => tr.exchange_id === exchangeId && tr.is_running)
  }

  const getExchangeUsageInfo = (exchangeId: string) => {
    const usingTraders = traders?.filter((tr) => tr.exchange_id === exchangeId) || []
    const runningCount = usingTraders.filter((tr) => tr.is_running).length
    const totalCount = usingTraders.length
    return { runningCount, totalCount, usingTraders }
  }

  const isModelUsedByAnyTrader = (modelId: string) => {
    return traders?.some((tr) => tr.ai_model === modelId) || false
  }

  const isExchangeUsedByAnyTrader = (exchangeId: string) => {
    return traders?.some((tr) => tr.exchange_id === exchangeId) || false
  }

  const getTradersUsingModel = (modelId: string) => {
    return traders?.filter((tr) => tr.ai_model === modelId) || []
  }

  const getTradersUsingExchange = (exchangeId: string) => {
    return traders?.filter((tr) => tr.exchange_id === exchangeId) || []
  }

  const buildModelInstanceId = (provider: string) => {
    const baseId = user?.id ? `${user.id}_${provider}` : provider
    const existingIDs = new Set(allModels.map((model) => model.id))
    if (!existingIDs.has(baseId)) {
      return baseId
    }

    let suffix = 2
    while (existingIDs.has(`${baseId}_${suffix}`)) {
      suffix += 1
    }
    return `${baseId}_${suffix}`
  }

  const buildModelInstanceName = (template: AIModel) => {
    const sameProviderModels = allModels.filter((model) => model.provider === template.provider)
    if (sameProviderModels.length === 0) {
      return template.name
    }
    return `${template.name} #${sameProviderModels.length + 1}`
  }

  const handleCreateTrader = async (data: CreateTraderRequest) => {
    try {
      const model = allModels?.find((m) => m.id === data.ai_model_id)
      const exchange = allExchanges?.find((e) => e.id === data.exchange_id)

      if (!model?.enabled) {
        toast.error(t('modelNotConfigured', language))
        return
      }

      if (!exchange?.enabled) {
        toast.error(t('exchangeNotConfigured', language))
        return
      }

      await toast.promise(api.createTrader(data), {
        loading: t('aiTradersToast.creating', language),
        success: t('aiTradersToast.created', language),
        error: t('aiTradersToast.createFailed', language),
      })
      setShowCreateModal(false)
      await mutateTraders()
    } catch (error) {
      console.error('Failed to create trader:', error)
      toast.error(t('createTraderFailed', language))
    }
  }

  const handleEditTrader = async (traderId: string) => {
    try {
      const traderConfig = await api.getTraderConfig(traderId)
      setEditingTrader(traderConfig)
      setShowEditModal(true)
    } catch (error) {
      console.error('Failed to fetch trader config:', error)
      toast.error(t('getTraderConfigFailed', language))
    }
  }

  const handleSaveEditTrader = async (data: CreateTraderRequest) => {
    console.log('🔥🔥🔥 handleSaveEditTrader CALLED with data:', data)
    if (!editingTrader) return

    try {
      const model = enabledModels?.find((m) => m.id === data.ai_model_id)
      const exchange = enabledExchanges?.find((e) => e.id === data.exchange_id)

      if (!model) {
        toast.error(t('modelConfigNotExist', language))
        return
      }

      if (!exchange) {
        toast.error(t('exchangeConfigNotExist', language))
        return
      }

      const request = {
        name: data.name,
        ai_model_id: data.ai_model_id,
        exchange_id: data.exchange_id,
        strategy_id: data.strategy_id,
        initial_balance: data.initial_balance,
        scan_interval_minutes: data.scan_interval_minutes,
        is_cross_margin: data.is_cross_margin,
        show_in_competition: data.show_in_competition,
      }

      console.log('🔥 handleSaveEditTrader - data:', data)
      console.log('🔥 handleSaveEditTrader - data.strategy_id:', data.strategy_id)
      console.log('🔥 handleSaveEditTrader - request:', request)

      await toast.promise(api.updateTrader(editingTrader.trader_id, request), {
        loading: t('aiTradersToast.saving', language),
        success: t('aiTradersToast.saved', language),
        error: t('aiTradersToast.saveFailed', language),
      })
      setShowEditModal(false)
      setEditingTrader(null)
      await mutateTraders()
    } catch (error) {
      console.error('Failed to update trader:', error)
      toast.error(t('updateTraderFailed', language))
    }
  }

  const handleDeleteTrader = async (traderId: string) => {
    {
      const ok = await confirmToast(t('confirmDeleteTrader', language))
      if (!ok) return
    }

    try {
      await toast.promise(api.deleteTrader(traderId), {
        loading: t('aiTradersToast.deleting', language),
        success: t('aiTradersToast.deleted', language),
        error: t('aiTradersToast.deleteFailed', language),
      })

      await mutateTraders()
    } catch (error) {
      console.error('Failed to delete trader:', error)
      toast.error(t('deleteTraderFailed', language))
    }
  }

  const handleToggleTrader = async (traderId: string, running: boolean) => {
    try {
      if (running) {
        await toast.promise(api.stopTrader(traderId), {
          loading: t('aiTradersToast.stopping', language),
          success: t('aiTradersToast.stopped', language),
          error: t('aiTradersToast.stopFailed', language),
        })
      } else {
        await toast.promise(api.startTrader(traderId), {
          loading: t('aiTradersToast.starting', language),
          success: t('aiTradersToast.started', language),
          error: t('aiTradersToast.startFailed', language),
        })
      }

      await mutateTraders()
    } catch (error) {
      console.error('Failed to toggle trader:', error)
      toast.error(t('operationFailed', language))
    }
  }

  const handleToggleCompetition = async (traderId: string, currentShowInCompetition: boolean) => {
    try {
      const newValue = !currentShowInCompetition
      await toast.promise(api.toggleCompetition(traderId, newValue), {
        loading: t('aiTradersToast.updating', language),
        success: newValue ? t('aiTradersToast.showInCompetition', language) : t('aiTradersToast.hideInCompetition', language),
        error: t('aiTradersToast.updateFailed', language),
      })

      await mutateTraders()
    } catch (error) {
      console.error('Failed to toggle competition visibility:', error)
      toast.error(t('operationFailed', language))
    }
  }

  const handleModelClick = (modelId: string) => {
    if (!isModelInUse(modelId)) {
      setEditingModel(modelId)
      setShowModelModal(true)
    }
  }

  const handleExchangeClick = (exchangeId: string) => {
    if (!isExchangeInUse(exchangeId)) {
      setEditingExchange(exchangeId)
      setShowExchangeModal(true)
    }
  }

  const handleDeleteConfig = async <T extends { id: string }>(config: {
    id: string
    type: 'model' | 'exchange'
    checkInUse: (id: string) => boolean
    getUsingTraders: (id: string) => any[]
    cannotDeleteKey: string
    confirmDeleteKey: string
    allItems: T[] | undefined
    clearFields: (item: T) => T
    buildRequest: (items: T[]) => any
    updateApi: (request: any) => Promise<void>
    refreshApi: () => Promise<T[]>
    setItems: (items: T[]) => void
    closeModal: () => void
    errorKey: string
  }) => {
    if (config.checkInUse(config.id)) {
      const usingTraders = config.getUsingTraders(config.id)
      const traderNames = usingTraders.map((tr) => tr.trader_name).join(', ')
      toast.error(
        `${t(config.cannotDeleteKey, language)} · ${t('tradersUsing', language)}: ${traderNames} · ${t('pleaseDeleteTradersFirst', language)}`
      )
      return
    }

    {
      const ok = await confirmToast(t(config.confirmDeleteKey, language))
      if (!ok) return
    }

    try {
      const updatedItems =
        config.allItems?.map((item) =>
          item.id === config.id ? config.clearFields(item) : item
        ) || []

      const request = config.buildRequest(updatedItems)
      await toast.promise(config.updateApi(request), {
        loading: t('aiTradersToast.updatingConfig', language),
        success: t('aiTradersToast.configUpdated', language),
        error: t('aiTradersToast.configUpdateFailed', language),
      })

      const refreshedItems = await config.refreshApi()
      config.setItems(refreshedItems)

      config.closeModal()
    } catch (error) {
      console.error(`Failed to delete ${config.type} config:`, error)
      toast.error(t(config.errorKey, language))
    }
  }

  const handleDeleteModelConfig = async (modelId: string) => {
    await handleDeleteConfig({
      id: modelId,
      type: 'model',
      checkInUse: isModelUsedByAnyTrader,
      getUsingTraders: getTradersUsingModel,
      cannotDeleteKey: 'cannotDeleteModelInUse',
      confirmDeleteKey: 'confirmDeleteModel',
      allItems: allModels,
      clearFields: (m) => ({
        ...m,
        apiKey: '',
        customApiUrl: '',
        customModelName: '',
        enabled: false,
      }),
      buildRequest: buildModelConfigRequest,
      updateApi: api.updateModelConfigs,
      refreshApi: api.getModelConfigs,
      setItems: (items) => {
        setAllModels(filterVisibleModels(items))
      },
      closeModal: () => {
        setShowModelModal(false)
        setEditingModel(null)
      },
      errorKey: 'deleteConfigFailed',
    })
  }

  const handleSaveModelConfig = async (
    modelId: string,
    apiKey: string,
    customApiUrl?: string,
    customModelName?: string
  ) => {
    try {
      const existingModel = allModels?.find((m) => m.id === modelId)
      let updatedModels

      const modelToUpdate =
        existingModel || supportedModels?.find((m) => m.id === modelId)
      if (!modelToUpdate) {
        toast.error(t('modelNotExist', language))
        return
      }

      if (existingModel) {
        updatedModels =
          allModels?.map((m) =>
            m.id === modelId
              ? {
                ...m,
                apiKey,
                customApiUrl: customApiUrl || '',
                customModelName: customModelName || '',
                enabled: true,
              }
              : m
          ) || []
      } else {
        const instanceId = buildModelInstanceId(modelToUpdate.provider)
        const newModel = {
          ...modelToUpdate,
          id: instanceId,
          name: buildModelInstanceName(modelToUpdate),
          apiKey,
          customApiUrl: customApiUrl || '',
          customModelName: customModelName || '',
          enabled: true,
        }
        updatedModels = [...(allModels || []), newModel]
      }

      const request = buildModelConfigRequest(updatedModels)

      await toast.promise(api.updateModelConfigs(request), {
        loading: t('aiTradersToast.updatingModelConfig', language),
        success: t('aiTradersToast.modelConfigUpdated', language),
        error: t('aiTradersToast.modelConfigUpdateFailed', language),
      })

      const refreshedModels = await api.getModelConfigs()
      setAllModels(filterVisibleModels(refreshedModels))

      setShowModelModal(false)
      setEditingModel(null)
    } catch (error) {
      console.error('Failed to save model config:', error)
      toast.error(t('saveConfigFailed', language))
    }
  }

  const handleDeleteExchangeConfig = async (exchangeId: string) => {
    if (isExchangeUsedByAnyTrader(exchangeId)) {
      const tradersUsing = getTradersUsingExchange(exchangeId)
      toast.error(
        `${t('cannotDeleteExchangeInUse', language)}: ${tradersUsing.join(', ')}`
      )
      return
    }

    const ok = await confirmToast(t('confirmDeleteExchange', language))
    if (!ok) return

    try {
      await toast.promise(api.deleteExchange(exchangeId), {
        loading: t('aiTradersToast.deletingExchange', language),
        success: t('aiTradersToast.exchangeDeleted', language),
        error: t('aiTradersToast.exchangeDeleteFailed', language),
      })

      const refreshedExchanges = await api.getExchangeConfigs()
      setAllExchanges(refreshedExchanges)

      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch (error) {
      console.error('Failed to delete exchange config:', error)
      toast.error(t('deleteExchangeConfigFailed', language))
    }
  }

  const handleSaveExchangeConfig = async (
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
        const existingExchange = allExchanges?.find((e) => e.id === exchangeId)
        if (!existingExchange) {
          toast.error(t('exchangeNotExist', language))
          return
        }

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
          loading: t('aiTradersToast.updatingExchangeConfig', language),
          success: t('aiTradersToast.exchangeConfigUpdated', language),
          error: t('aiTradersToast.exchangeConfigUpdateFailed', language),
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
          loading: t('aiTradersToast.creatingExchange', language),
          success: t('aiTradersToast.exchangeCreated', language),
          error: t('aiTradersToast.exchangeCreateFailed', language),
        })
      }

	      const refreshedExchanges = await api.getExchangeConfigs()
	      setAllExchanges(refreshedExchanges)
	      const refreshedProxies = await api.getProxyServers()
	      setProxyServers(refreshedProxies)

      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch (error) {
      console.error('Failed to save exchange config:', error)
      toast.error(t('saveConfigFailed', language))
    }
  }

  const handleAddModel = () => {
    setEditingModel(null)
    setShowModelModal(true)
  }

  const handleAddExchange = () => {
    setEditingExchange(null)
    setShowExchangeModal(true)
  }

  const refreshProxyServers = async () => {
    const refreshed = await api.getProxyServers()
    setProxyServers(refreshed)
  }

  const handleCreateProxyServer = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!proxyUrl.trim()) {
      toast.error(language === 'zh' ? '请填写代理地址' : 'Enter a proxy URL')
      return
    }

    setProxySaving(true)
    setProxySaveStatus(null)
    try {
      await toast.promise(
        api.createProxyServer({
          name: proxyName.trim() || (language === 'zh' ? '未命名代理' : 'Unnamed proxy'),
          proxy_url: proxyUrl.trim(),
        }),
        {
          loading: language === 'zh' ? '正在测试并创建代理...' : 'Testing and creating proxy...',
          success: language === 'zh' ? '代理服务器已创建' : 'Proxy server created',
          error: language === 'zh' ? '创建代理服务器失败' : 'Failed to create proxy server',
        }
      )
      setProxyName('')
      setProxyUrl('')
      await refreshProxyServers()
      setProxySaveStatus({
        type: 'success',
        message: language === 'zh' ? '测试通过，已保存' : 'Test passed and saved',
      })
    } catch (error) {
      console.error('Failed to create proxy server:', error)
      setProxySaveStatus({
        type: 'error',
        message: language === 'zh' ? '测试未通过' : 'Test failed',
      })
    } finally {
      setProxySaving(false)
    }
  }

  const handleTestSavedProxyServer = async (proxyId: string) => {
    setTestingProxyId(proxyId)
    try {
      const result = await api.testSavedProxyServer(proxyId)
      toast.success(language === 'zh' ? `代理测试通过，出口 IP：${result.public_ip}` : `Proxy test passed. Exit IP: ${result.public_ip}`)
      await refreshProxyServers()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : (language === 'zh' ? '代理测试失败' : 'Proxy test failed'))
      await refreshProxyServers()
    } finally {
      setTestingProxyId(null)
    }
  }

  const handleDeleteProxyServer = async (proxyId: string) => {
    try {
      await toast.promise(api.deleteProxyServer(proxyId), {
        loading: language === 'zh' ? '正在删除代理服务器...' : 'Deleting proxy server...',
        success: language === 'zh' ? '代理服务器已删除' : 'Proxy server deleted',
        error: language === 'zh' ? '删除代理服务器失败' : 'Failed to delete proxy server',
      })
      await refreshProxyServers()
    } catch (error) {
      console.error('Failed to delete proxy server:', error)
    }
  }

  return (
    <DeepVoidBackground className="traders-page py-5 md:py-8" disableAnimation>
      <div className="w-full px-4 md:px-8 space-y-8 animate-fade-in">
        {/* Header - Terminal Style */}
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 pb-6" style={{ borderBottom: '1px solid var(--panel-border)' }}>
          <div className="flex items-center gap-4">
            <div className="relative group">
              <div className="absolute -inset-1 bg-nofx-gold/20 rounded-xl blur opacity-0 group-hover:opacity-100 transition duration-500"></div>
              <div className="w-12 h-12 md:w-14 md:h-14 rounded-xl flex items-center justify-center border border-nofx-gold/30 text-nofx-gold relative z-10 shadow-[0_0_15px_rgba(240,185,11,0.1)]" style={{ background: 'var(--panel-bg)' }}>
                <Bot className="w-6 h-6 md:w-7 md:h-7" />
              </div>
            </div>
            <div>
              <h1 className="text-2xl md:text-3xl font-bold font-mono tracking-tight flex items-center gap-3 uppercase" style={{ color: 'var(--text-primary)' }}>
                {t('aiTraders', language)}
                <span className="text-xs font-mono font-normal px-2 py-0.5 rounded bg-nofx-gold/10 text-nofx-gold border border-nofx-gold/20 tracking-wider">
                  {traders?.length || 0} {language === 'zh' ? '活跃节点' : 'ACTIVE_NODES'}
                </span>
              </h1>
              <p className="text-xs font-mono uppercase tracking-widest mt-1 ml-1 flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
                <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse"></span>
                {language === 'zh' ? '系统就绪' : 'SYSTEM_READY'}
              </p>
              <p className="mt-2 ml-1 text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                {language === 'zh'
                  ? '创建AI模型 --> 创建交易所 --> 创建AI交易员 --> 开始交易'
                  : 'Create AI model --> Create exchange --> Create AI trader --> Start trading'}
              </p>
            </div>
          </div>

          <div className="flex gap-2 w-full md:w-auto overflow-x-auto pb-1 md:pb-0 hide-scrollbar">
            <button
              onClick={handleAddModel}
              className="px-4 py-2 rounded text-xs font-mono uppercase tracking-wider transition-all whitespace-nowrap backdrop-blur-sm"
              style={{ border: '1px solid var(--panel-border)', background: 'var(--panel-bg-solid)', color: 'var(--text-secondary)' }}
            >
              <div className="flex items-center gap-2">
                <Plus className="w-3 h-3" />
                <span>{language === 'zh' ? '模型配置' : 'MODELS_CONFIG'}</span>
              </div>
            </button>

            <button
              onClick={handleAddExchange}
              className="px-4 py-2 rounded text-xs font-mono uppercase tracking-wider transition-all whitespace-nowrap backdrop-blur-sm"
              style={{ border: '1px solid var(--panel-border)', background: 'var(--panel-bg-solid)', color: 'var(--text-secondary)' }}
            >
              <div className="flex items-center gap-2">
                <Plus className="w-3 h-3" />
                <span>{language === 'zh' ? '交易所密钥' : 'EXCHANGE_KEYS'}</span>
              </div>
            </button>

            <button
              onClick={() => setShowProxyModal(true)}
              className="px-4 py-2 rounded text-xs font-mono uppercase tracking-wider transition-all whitespace-nowrap backdrop-blur-sm"
              style={{ border: '1px solid var(--panel-border)', background: 'var(--panel-bg-solid)', color: 'var(--text-secondary)' }}
            >
              <div className="flex items-center gap-2">
                <Server className="w-3 h-3" />
                <span>{language === 'zh' ? '代理服务器' : 'PROXY_SERVERS'}</span>
              </div>
            </button>

            <button
              onClick={() => setShowTelegramModal(true)}
              className="px-4 py-2 rounded text-xs font-mono uppercase tracking-wider transition-all whitespace-nowrap backdrop-blur-sm"
              style={{ border: '1px solid color-mix(in srgb, #0ea5e9 40%, var(--panel-border))', background: 'var(--panel-bg-solid)', color: '#0ea5e9' }}
            >
              <div className="flex items-center gap-2">
                <MessageCircle className="w-3 h-3" />
                <span>{language === 'zh' ? 'Telegram 机器人' : 'TELEGRAM_BOT'}</span>
              </div>
            </button>

            <button
              onClick={() => setShowCreateModal(true)}
              disabled={configuredModels.length === 0 || configuredExchanges.length === 0}
              className="group relative px-6 py-2 rounded text-xs font-bold font-mono uppercase tracking-wider transition-all disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap overflow-hidden bg-nofx-gold text-black hover:bg-yellow-400 shadow-[0_0_20px_rgba(240,185,11,0.2)] hover:shadow-[0_0_30px_rgba(240,185,11,0.4)]"
            >
              <span className="relative z-10 flex items-center gap-2">
                <Plus className="w-4 h-4" />
                {t('createTrader', language)}
              </span>
              <div className="absolute inset-0 bg-white/20 translate-y-full group-hover:translate-y-0 transition-transform duration-300"></div>
            </button>
          </div>
        </div>

        {/* Configuration Status Grid */}
        <ConfigStatusGrid
          configuredModels={configuredModels}
          configuredExchanges={configuredExchanges}
          configuredProxyServers={proxyServers}
          visibleExchangeAddresses={visibleExchangeAddresses}
          copiedId={copiedId}
          language={language}
          isModelInUse={isModelInUse}
          getModelUsageInfo={getModelUsageInfo}
          isExchangeInUse={isExchangeInUse}
          getExchangeUsageInfo={getExchangeUsageInfo}
          onModelClick={handleModelClick}
          onExchangeClick={handleExchangeClick}
          onProxyClick={() => setShowProxyModal(true)}
          onToggleExchangeAddress={toggleExchangeAddressVisibility}
          onCopyAddress={handleCopyAddress}
        />

        {/* Traders List */}
        <TradersList
          traders={traders}
          isLoading={isTradersLoading}
          allExchanges={allExchanges}
          configuredModelsCount={configuredModels.length}
          configuredExchangesCount={configuredExchanges.length}
          visibleTraderAddresses={visibleTraderAddresses}
          copiedId={copiedId}
          language={language}
          onTraderSelect={onTraderSelect}
          onNavigate={(path) => navigate(path)}
          onEditTrader={handleEditTrader}
          onToggleTrader={handleToggleTrader}
          onToggleCompetition={handleToggleCompetition}
          onDeleteTrader={handleDeleteTrader}
          onToggleTraderAddress={toggleTraderAddressVisibility}
          onCopyAddress={handleCopyAddress}
        />

        {/* Create Trader Modal */}
        {showCreateModal && (
          <TraderConfigModal
            isOpen={showCreateModal}
            isEditMode={false}
            availableModels={enabledModels}
            availableExchanges={enabledExchanges}
            onSave={handleCreateTrader}
            onClose={() => setShowCreateModal(false)}
          />
        )}

        {/* Edit Trader Modal */}
        {showEditModal && editingTrader && (
          <TraderConfigModal
            isOpen={showEditModal}
            isEditMode={true}
            traderData={editingTrader}
            availableModels={enabledModels}
            availableExchanges={enabledExchanges}
            onSave={handleSaveEditTrader}
            onClose={() => {
              setShowEditModal(false)
              setEditingTrader(null)
            }}
          />
        )}

        {/* Model Configuration Modal */}
        {showModelModal && (
          <ModelConfigModal
            allModels={supportedModels}
            configuredModels={allModels}
            editingModelId={editingModel}
            onSave={handleSaveModelConfig}
            onDelete={handleDeleteModelConfig}
            onClose={() => {
              setShowModelModal(false)
              setEditingModel(null)
            }}
            language={language}
          />
        )}

        {/* Exchange Configuration Modal */}
        {showExchangeModal && (
          <ExchangeConfigModal
            allExchanges={allExchanges}
	            proxyServers={proxyServers}
            editingExchangeId={editingExchange}
            onSave={handleSaveExchangeConfig}
            onDelete={handleDeleteExchangeConfig}
            onClose={() => {
              setShowExchangeModal(false)
              setEditingExchange(null)
            }}
            language={language}
          />
        )}

        {showProxyModal && (
          <div className="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4" onClick={() => setShowProxyModal(false)}>
            <div
              className="w-full max-w-2xl rounded-2xl overflow-hidden"
              style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)' }}
              onClick={(event) => event.stopPropagation()}
            >
              <div className="px-6 py-4 flex items-center justify-between" style={{ borderBottom: '1px solid var(--panel-border)' }}>
                <div>
                  <h3 className="text-lg font-bold" style={{ color: 'var(--text-primary)' }}>
                    {language === 'zh' ? '代理服务器' : 'Proxy Servers'}
                  </h3>
                  <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
                    {language === 'zh' ? '一个交易所 API 绑定一个独立代理服务器。' : 'Bind one independent proxy server to each exchange API.'}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => setShowProxyModal(false)}
                  className="px-3 py-1.5 rounded text-xs font-semibold"
                  style={{ background: 'var(--panel-bg)', color: 'var(--text-secondary)', border: '1px solid var(--panel-border)' }}
                >
                  {language === 'zh' ? '关闭' : 'Close'}
                </button>
              </div>

              <div className="p-6 space-y-5 max-h-[80vh] overflow-y-auto">
                <form onSubmit={handleCreateProxyServer} className="space-y-3 rounded-xl p-4" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}>
                  <div className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                    {language === 'zh' ? '新增代理服务器' : 'Add Proxy Server'}
                  </div>
                  <input
                    type="text"
                    value={proxyName}
                    onChange={(event) => setProxyName(event.target.value)}
                    placeholder={language === 'zh' ? '代理名称，例如：OKX 代理 1' : 'Proxy name, e.g. OKX Proxy 1'}
                    className="w-full px-4 py-3 rounded-xl text-sm"
                    style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                  />
                  <input
                    type="text"
                    value={proxyUrl}
                    onChange={(event) => setProxyUrl(event.target.value)}
                    placeholder="socks5://user:pass@1.2.3.4:1080"
                    className="w-full px-4 py-3 rounded-xl text-sm font-mono"
                    style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)', color: 'var(--text-primary)' }}
                    required
                  />
                  <button
                    type="submit"
                    disabled={proxySaving}
                    className="w-full bg-nofx-gold hover:bg-yellow-400 text-black font-semibold py-3 rounded-xl text-sm transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {proxySaving ? (language === 'zh' ? '测试并保存中...' : 'Testing and saving...') : (language === 'zh' ? '测试并保存代理' : 'Test and Save Proxy')}
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
	                    {language === 'zh' ? '保存前会实际走代理访问公网 IP 查询接口；测试失败不会保存。' : 'The proxy is tested against a public IP endpoint before saving; failed tests are not saved.'}
	                  </p>
	                  <div
	                    className="rounded-xl px-4 py-3 text-xs leading-6"
	                    style={{ background: 'var(--panel-bg-solid)', border: '1px solid var(--panel-border)', color: 'var(--text-secondary)' }}
	                  >
	                    {language === 'zh' ? (
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
                    <div className="text-center py-8 text-sm rounded-xl" style={{ color: 'var(--text-tertiary)', background: 'var(--panel-bg)', border: '1px dashed var(--panel-border)' }}>
                      {language === 'zh' ? '尚未配置代理服务器' : 'No proxy servers configured yet'}
                    </div>
                  ) : (
                    proxyServers.map((proxy) => (
                      <div
                        key={proxy.id}
                        className="rounded-xl px-4 py-3"
                        style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <p className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>{proxy.name}</p>
                            <p className="text-xs mt-1" style={{ color: 'var(--text-tertiary)' }}>
                              {language === 'zh' ? '出口 IP：' : 'Exit IP: '}
                              <span className="font-mono">{proxy.last_exit_ip || '-'}</span>
                            </p>
                            <p className="text-xs mt-1" style={{ color: 'var(--text-tertiary)' }}>
                              {proxy.bound_exchange_name
                                ? (language === 'zh' ? `已绑定：${proxy.bound_exchange_name}` : `Bound: ${proxy.bound_exchange_name}`)
                                : (language === 'zh' ? '未绑定交易所' : 'Unbound')}
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
                              {testingProxyId === proxy.id ? (language === 'zh' ? '测试中' : 'Testing') : (language === 'zh' ? '测试' : 'Test')}
                            </button>
                            <button
                              type="button"
                              onClick={() => handleDeleteProxyServer(proxy.id)}
                              disabled={Boolean(proxy.bound_exchange_id)}
                              className="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                              style={{ background: 'rgba(246, 70, 93, 0.12)', color: '#F6465D' }}
                              title={proxy.bound_exchange_id ? (language === 'zh' ? '已绑定交易所，不能删除' : 'Bound to an exchange and cannot be deleted') : undefined}
                            >
                              <Trash2 className="w-3 h-3" />
                            </button>
                          </div>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Telegram Bot Modal */}
        {showTelegramModal && (
          <TelegramConfigModal
            onClose={() => setShowTelegramModal(false)}
            language={language}
          />
        )}
      </div>
    </DeepVoidBackground>
  )
}
