import type {
  AIModel,
  AdminOverview,
  AdminAuditLog,
  AdminUserSummary,
  Exchange,
  ProxyServer,
  UpdateModelConfigRequest,
  UpdateExchangeConfigRequest,
  CreateExchangeRequest,
} from '../../types'
import { API_BASE, httpClient, CryptoService } from './helpers'

export const configApi = {
  async getModelConfigs(): Promise<AIModel[]> {
    const result = await httpClient.get<AIModel[]>(`${API_BASE}/models`)
    if (!result.success) throw new Error('Failed to fetch model configs')
    return Array.isArray(result.data) ? result.data : []
  },

  async getSupportedModels(): Promise<AIModel[]> {
    const result = await httpClient.get<AIModel[]>(
      `${API_BASE}/supported-models`
    )
    if (!result.success) throw new Error('Failed to fetch supported models')
    return result.data!
  },

  async getPromptTemplates(): Promise<string[]> {
    const res = await fetch(`${API_BASE}/prompt-templates`)
    if (!res.ok) throw new Error('Failed to fetch prompt templates')
    const data = await res.json()
    if (Array.isArray(data.templates)) {
      return data.templates.map((item: { name: string }) => item.name)
    }
    return []
  },

  async updateModelConfigs(request: UpdateModelConfigRequest): Promise<void> {
    // Check if transport encryption is enabled
    const config = await CryptoService.fetchCryptoConfig()

    if (!config.transport_encryption) {
      // Transport encryption disabled, send plaintext
      const result = await httpClient.put(`${API_BASE}/models`, request)
      if (!result.success) throw new Error('Failed to update model configs')
      return
    }

    // Fetch RSA public key
    const publicKey = await CryptoService.fetchPublicKey()

    // Initialize crypto service
    await CryptoService.initialize(publicKey)

    // Get user info from localStorage
    const userId = localStorage.getItem('user_id') || ''
    const sessionId = sessionStorage.getItem('session_id') || ''

    // Encrypt sensitive data
    const encryptedPayload = await CryptoService.encryptSensitiveData(
      JSON.stringify(request),
      userId,
      sessionId
    )

    // Send encrypted data
    const result = await httpClient.put(`${API_BASE}/models`, encryptedPayload)
    if (!result.success) throw new Error('Failed to update model configs')
  },

  async getExchangeConfigs(): Promise<Exchange[]> {
    const result = await httpClient.get<Exchange[]>(`${API_BASE}/exchanges`)
    if (!result.success) throw new Error('Failed to fetch exchange configs')
    return result.data!
  },

  async getSupportedExchanges(): Promise<Exchange[]> {
    const result = await httpClient.get<Exchange[]>(
      `${API_BASE}/supported-exchanges`
    )
    if (!result.success) throw new Error('Failed to fetch supported exchanges')
    return result.data!
  },

  async updateExchangeConfigs(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    const result = await httpClient.put(`${API_BASE}/exchanges`, request)
    if (!result.success) throw new Error('Failed to update exchange configs')
  },

  async createExchange(request: CreateExchangeRequest): Promise<{ id: string }> {
    const result = await httpClient.post<{ id: string }>(`${API_BASE}/exchanges`, request)
    if (!result.success) throw new Error('Failed to create exchange account')
    return result.data!
  },

  async createExchangeEncrypted(request: CreateExchangeRequest): Promise<{ id: string }> {
    // Check if transport encryption is enabled
    const config = await CryptoService.fetchCryptoConfig()

    if (!config.transport_encryption) {
      // Transport encryption disabled, send plaintext
      const result = await httpClient.post<{ id: string }>(`${API_BASE}/exchanges`, request)
      if (!result.success) throw new Error('Failed to create exchange account')
      return result.data!
    }

    // Fetch RSA public key
    const publicKey = await CryptoService.fetchPublicKey()

    // Initialize crypto service
    await CryptoService.initialize(publicKey)

    // Get user info
    const userId = localStorage.getItem('user_id') || ''
    const sessionId = sessionStorage.getItem('session_id') || ''

    // Encrypt sensitive data
    const encryptedPayload = await CryptoService.encryptSensitiveData(
      JSON.stringify(request),
      userId,
      sessionId
    )

    // Send encrypted data
    const result = await httpClient.post<{ id: string }>(
      `${API_BASE}/exchanges`,
      encryptedPayload
    )
    if (!result.success) throw new Error('Failed to create exchange account')
    return result.data!
  },

  async deleteExchange(exchangeId: string): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/exchanges/${exchangeId}`)
    if (!result.success) throw new Error('Failed to delete exchange account')
  },

  async updateExchangeConfigsEncrypted(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    // Check if transport encryption is enabled
    const config = await CryptoService.fetchCryptoConfig()

    if (!config.transport_encryption) {
      // Transport encryption disabled, send plaintext
      const result = await httpClient.put(`${API_BASE}/exchanges`, request)
      if (!result.success) throw new Error('Failed to update exchange configs')
      return
    }

    // Fetch RSA public key
    const publicKey = await CryptoService.fetchPublicKey()

    // Initialize crypto service
    await CryptoService.initialize(publicKey)

    // Get user info from localStorage
    const userId = localStorage.getItem('user_id') || ''
    const sessionId = sessionStorage.getItem('session_id') || ''

    // Encrypt sensitive data
    const encryptedPayload = await CryptoService.encryptSensitiveData(
      JSON.stringify(request),
      userId,
      sessionId
    )

    // Send encrypted data
    const result = await httpClient.put(
      `${API_BASE}/exchanges`,
      encryptedPayload
    )
    if (!result.success) throw new Error('Failed to update exchange configs')
  },

  async getServerIP(): Promise<{
    public_ip: string
    message: string
  }> {
    const result = await httpClient.get<{
      public_ip: string
      message: string
    }>(`${API_BASE}/server-ip`)
    if (!result.success) throw new Error('Failed to fetch server IP')
    return result.data!
  },

  async testProxy(proxyUrl: string): Promise<{
    public_ip: string
    message: string
  }> {
    const result = await httpClient.post<{
      public_ip: string
      message: string
    }>(`${API_BASE}/test-proxy`, {
      proxy_url: proxyUrl,
    })
    if (!result.success || !result.data) {
      throw new Error(result.message || 'Failed to test proxy server')
    }
    return result.data
  },

  async getProxyServers(): Promise<ProxyServer[]> {
    const result = await httpClient.get<ProxyServer[]>(`${API_BASE}/proxies`)
    if (!result.success) throw new Error('Failed to fetch proxy servers')
    return Array.isArray(result.data) ? result.data : []
  },

  async createProxyServer(request: {
    name: string
    proxy_url: string
  }): Promise<{ id: string; public_ip: string; message: string }> {
    const result = await httpClient.post<{ id: string; public_ip: string; message: string }>(
      `${API_BASE}/proxies`,
      request
    )
    if (!result.success || !result.data) {
      throw new Error(result.message || 'Failed to create proxy server')
    }
    return result.data
  },

  async updateProxyServer(
    id: string,
    request: { name: string; proxy_url?: string; enabled: boolean }
  ): Promise<{ public_ip?: string; message: string }> {
    const result = await httpClient.put<{ public_ip?: string; message: string }>(
      `${API_BASE}/proxies/${id}`,
      request
    )
    if (!result.success || !result.data) {
      throw new Error(result.message || 'Failed to update proxy server')
    }
    return result.data
  },

  async testSavedProxyServer(id: string): Promise<{
    public_ip: string
    message: string
  }> {
    const result = await httpClient.post<{ public_ip: string; message: string }>(
      `${API_BASE}/proxies/${id}/test`,
      {}
    )
    if (!result.success || !result.data) {
      throw new Error(result.message || 'Failed to test proxy server')
    }
    return result.data
  },

  async deleteProxyServer(id: string): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/proxies/${id}`)
    if (!result.success) throw new Error(result.message || 'Failed to delete proxy server')
  },

  async getAdminOverview(): Promise<AdminOverview> {
    const result = await httpClient.get<AdminOverview>(`${API_BASE}/admin/overview`)
    if (!result.success || !result.data) throw new Error('Failed to fetch admin overview')
    return result.data
  },

  async getAdminUsers(): Promise<{ users: AdminUserSummary[] }> {
    const result = await httpClient.get<{ users: AdminUserSummary[] }>(`${API_BASE}/admin/users`)
    if (!result.success || !result.data) throw new Error('Failed to fetch admin users')
    return result.data
  },

  async getAdminAuditLogs(filters?: {
    action?: string
    query?: string
    limit?: number
  }): Promise<{ logs: AdminAuditLog[] }> {
    const params = new URLSearchParams()
    if (filters?.action) params.set('action', filters.action)
    if (filters?.query) params.set('q', filters.query)
    if (filters?.limit) params.set('limit', String(filters.limit))
    const query = params.toString()
    const result = await httpClient.get<{ logs: AdminAuditLog[] }>(
      `${API_BASE}/admin/audit-logs${query ? `?${query}` : ''}`
    )
    if (!result.success || !result.data) throw new Error('Failed to fetch admin audit logs')
    return result.data
  },

  async updateAdminRegistration(registrationEnabled: boolean): Promise<void> {
    const result = await httpClient.put(`${API_BASE}/admin/registration`, {
      registration_enabled: registrationEnabled,
    })
    if (!result.success) throw new Error('Failed to update registration setting')
  },

  async updateAdminUserStatus(userId: string, enabled: boolean): Promise<void> {
    const result = await httpClient.put(`${API_BASE}/admin/users/${userId}/status`, {
      enabled,
    })
    if (!result.success) throw new Error(result.message || 'Failed to update user status')
  },

  async revokeAdminUserSessions(userId: string): Promise<void> {
    const result = await httpClient.post(`${API_BASE}/admin/users/${userId}/revoke-sessions`, {})
    if (!result.success) throw new Error(result.message || 'Failed to revoke user sessions')
  },

  async updateAdminUserArchive(userId: string, archived: boolean): Promise<void> {
    const result = await httpClient.put(`${API_BASE}/admin/users/${userId}/archive`, {
      archived,
    })
    if (!result.success) throw new Error(result.message || 'Failed to update user archive state')
  },

  async resetAdminUserPassword(userId: string, newPassword: string): Promise<void> {
    const result = await httpClient.put(`${API_BASE}/admin/users/${userId}/password`, {
      new_password: newPassword,
    })
    if (!result.success) throw new Error(result.message || 'Failed to reset user password')
  },

  async deleteAdminUser(userId: string): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/admin/users/${userId}`)
    if (!result.success) throw new Error(result.message || 'Failed to delete user')
  },
}
