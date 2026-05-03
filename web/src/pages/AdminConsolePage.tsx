import useSWR from 'swr'
import { useMemo, useState } from 'react'
import { toast } from 'sonner'
import {
  Activity,
  Archive,
  Bot,
  Building2,
  Clock3,
  KeyRound,
  LayoutDashboard,
  Lock,
  LogOut,
  RefreshCw,
  ScrollText,
  Search,
  Settings2,
  ShieldCheck,
  Trash2,
  UserCog,
  Users,
} from 'lucide-react'
import { api } from '../lib/api'
import { invalidateSystemConfig } from '../lib/config'
import { useLanguage } from '../contexts/LanguageContext'
import type { AdminAuditLog, AdminOverview, AdminUserSummary } from '../types'

const formatTime = (value?: string, locale = 'en-US') => {
  if (!value) return '--'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

type AdminSection = 'overview' | 'access' | 'users' | 'audit'

export function AdminConsolePage() {
  const { language } = useLanguage()
  const isZh = language === 'zh'
  const locale = isZh ? 'zh-CN' : 'en-US'

  const [activeSection, setActiveSection] = useState<AdminSection>('overview')
  const [registrationSaving, setRegistrationSaving] = useState(false)
  const [userActionLoading, setUserActionLoading] = useState<Record<string, boolean>>({})
  const [passwordDrafts, setPasswordDrafts] = useState<Record<string, string>>({})
  const [auditActionFilter, setAuditActionFilter] = useState('')
  const [auditQuery, setAuditQuery] = useState('')

  const {
    data: overview,
    mutate: mutateOverview,
    isLoading: overviewLoading,
  } = useSWR<AdminOverview>('admin-overview', api.getAdminOverview, {
    refreshInterval: 15000,
    revalidateOnFocus: true,
  })

  const {
    data: usersPayload,
    mutate: mutateUsers,
    isLoading: usersLoading,
  } = useSWR<{ users: AdminUserSummary[] }>('admin-users', api.getAdminUsers, {
    refreshInterval: 20000,
    revalidateOnFocus: true,
  })

  const auditKey = `admin-audit-logs:${auditActionFilter}:${auditQuery}`
  const {
    data: auditPayload,
    mutate: mutateAudit,
    isLoading: auditLoading,
  } = useSWR<{ logs: AdminAuditLog[] }>(auditKey, () =>
    api.getAdminAuditLogs({
      action: auditActionFilter || undefined,
      query: auditQuery || undefined,
      limit: 100,
    }), {
      refreshInterval: 15000,
      revalidateOnFocus: true,
    }
  )

  const users = usersPayload?.users ?? []
  const auditLogs = auditPayload?.logs ?? []
  const activeUsers = users.filter((entry) => !entry.is_archived)
  const archivedUsers = users.filter((entry) => entry.is_archived)

  const healthTone = useMemo(() => {
    if (!overview) return 'idle'
    if (
      overview.traders_running > 0 ||
      overview.models_enabled > 0 ||
      overview.exchanges_enabled > 0
    ) {
      return 'healthy'
    }
    return 'attention'
  }, [overview])

  const runtimeLabel = {
    idle: isZh ? '加载中' : 'Loading',
    healthy: isZh ? '运行正常' : 'Operational',
    attention: isZh ? '待配置' : 'Needs attention',
  }[healthTone]

  const sections = [
    {
      id: 'overview' as const,
      label: isZh ? '总览' : 'Overview',
      icon: LayoutDashboard,
    },
    {
      id: 'access' as const,
      label: isZh ? '访问控制' : 'Access',
      icon: Settings2,
    },
    {
      id: 'users' as const,
      label: isZh ? '用户管理' : 'Users',
      icon: UserCog,
    },
    {
      id: 'audit' as const,
      label: isZh ? '审计记录' : 'Audit',
      icon: ScrollText,
    },
  ]

  const statCards = overview
    ? [
        {
          title: isZh ? '总用户数' : 'Total users',
          value: overview.users_total,
          detail: isZh
            ? `近 7 天新增 ${overview.new_users_7d}`
            : `${overview.new_users_7d} new in 7d`,
          icon: Users,
        },
        {
          title: isZh ? '交易员实例' : 'Trader instances',
          value: overview.traders_total,
          detail: isZh
            ? `${overview.traders_running} 个运行中`
            : `${overview.traders_running} running`,
          icon: Activity,
        },
        {
          title: isZh ? 'AI 模型配置' : 'AI model configs',
          value: overview.models_total,
          detail: isZh
            ? `${overview.models_enabled} 个已启用`
            : `${overview.models_enabled} enabled`,
          icon: Bot,
        },
        {
          title: isZh ? '交易所接入' : 'Exchange connections',
          value: overview.exchanges_total,
          detail: isZh
            ? `${overview.exchanges_enabled} 个已启用`
            : `${overview.exchanges_enabled} enabled`,
          icon: Building2,
        },
      ]
    : []

  const runtimeCards = [
    {
      label: isZh ? '数据库' : 'Database',
      value: overview?.db_type?.toUpperCase() || '--',
      icon: Building2,
    },
    {
      label: isZh ? '传输加密' : 'Transport encryption',
      value: overview?.transport_encryption
        ? isZh ? '已开启' : 'Enabled'
        : isZh ? '已关闭' : 'Disabled',
      icon: Lock,
    },
    {
      label: isZh ? '外部量化源' : 'External quant source',
      value: overview?.nofxos_enabled
        ? isZh ? '仍启用' : 'Enabled'
        : isZh ? '已切断' : 'Disconnected',
      icon: RefreshCw,
    },
    {
      label: isZh ? 'CORS 域名数' : 'CORS origins',
      value: String(overview?.cors_origins ?? '--'),
      icon: ShieldCheck,
    },
  ]

  const auditActionOptions = useMemo(() => {
    const values = Array.from(new Set((auditPayload?.logs ?? []).map((entry) => entry.action)))
    return values.sort()
  }, [auditPayload?.logs])

  const setRowLoading = (userId: string, loading: boolean) => {
    setUserActionLoading((current) => ({ ...current, [userId]: loading }))
  }

  const refreshAdminData = async () => {
    await Promise.all([mutateOverview(), mutateUsers(), mutateAudit()])
  }

  const handleRegistrationToggle = async () => {
    if (!overview) return
    setRegistrationSaving(true)
    const nextValue = !overview.registration_enabled
    try {
      await api.updateAdminRegistration(nextValue)
      await refreshAdminData()
      invalidateSystemConfig()
      toast.success(
        nextValue
          ? isZh ? '新用户注册已开启' : 'User registration enabled'
          : isZh ? '新用户注册已关闭' : 'User registration disabled'
      )
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : isZh ? '更新注册设置失败' : 'Failed to update registration setting'
      )
    } finally {
      setRegistrationSaving(false)
    }
  }

  const handleUserStatusToggle = async (entry: AdminUserSummary) => {
    if (entry.is_admin) return
    const nextEnabled = !entry.is_active
    setRowLoading(entry.id, true)
    try {
      await api.updateAdminUserStatus(entry.id, nextEnabled)
      await refreshAdminData()
      toast.success(
        nextEnabled
          ? isZh ? `已启用 ${entry.email}` : `${entry.email} enabled`
          : isZh ? `已停用 ${entry.email} 并强制下线` : `${entry.email} disabled and signed out`
      )
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : isZh ? '更新用户状态失败' : 'Failed to update user status'
      )
    } finally {
      setRowLoading(entry.id, false)
    }
  }

  const handleRevokeSessions = async (entry: AdminUserSummary) => {
    if (entry.is_admin) return
    setRowLoading(entry.id, true)
    try {
      await api.revokeAdminUserSessions(entry.id)
      await refreshAdminData()
      toast.success(isZh ? `已强制 ${entry.email} 下线` : `${entry.email} signed out`)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : isZh ? '强制下线失败' : 'Failed to revoke sessions'
      )
    } finally {
      setRowLoading(entry.id, false)
    }
  }

  const handleArchiveToggle = async (entry: AdminUserSummary) => {
    if (entry.is_admin) return
    const nextArchived = !entry.is_archived
    setRowLoading(entry.id, true)
    try {
      await api.updateAdminUserArchive(entry.id, nextArchived)
      await refreshAdminData()
      toast.success(
        nextArchived
          ? isZh ? `已归档 ${entry.email}` : `${entry.email} archived`
          : isZh ? `已恢复 ${entry.email}` : `${entry.email} restored`
      )
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : isZh ? '归档操作失败' : 'Failed to update archive state'
      )
    } finally {
      setRowLoading(entry.id, false)
    }
  }

  const handleDeleteUser = async (entry: AdminUserSummary) => {
    if (entry.is_admin) return
    const confirmed = window.confirm(
      isZh
        ? `确认永久删除 ${entry.email} 吗？仅当该用户已无交易员、模型和交易所配置时才允许删除。`
        : `Permanently delete ${entry.email}? This only works when the user has no traders, models, or exchanges left.`
    )
    if (!confirmed) return

    setRowLoading(entry.id, true)
    try {
      await api.deleteAdminUser(entry.id)
      await refreshAdminData()
      toast.success(isZh ? `已删除 ${entry.email}` : `${entry.email} deleted`)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : isZh ? '删除用户失败' : 'Failed to delete user'
      )
    } finally {
      setRowLoading(entry.id, false)
    }
  }

  const handleResetPassword = async (entry: AdminUserSummary) => {
    const nextPassword = (passwordDrafts[entry.id] || '').trim()
    if (nextPassword.length < 8) {
      toast.error(isZh ? '新密码至少 8 位' : 'Password must be at least 8 characters')
      return
    }

    setRowLoading(entry.id, true)
    try {
      await api.resetAdminUserPassword(entry.id, nextPassword)
      setPasswordDrafts((current) => ({ ...current, [entry.id]: '' }))
      await refreshAdminData()
      toast.success(
        isZh ? `已重置 ${entry.email} 的密码并撤销旧会话` : `Password reset for ${entry.email}`
      )
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : isZh ? '重置密码失败' : 'Failed to reset password'
      )
    } finally {
      setRowLoading(entry.id, false)
    }
  }

  const renderUserRow = (entry: AdminUserSummary) => {
    const rowLoading = Boolean(userActionLoading[entry.id])
    return (
      <tr key={entry.id} className="border-t align-top" style={{ borderColor: 'var(--panel-border)' }}>
        <td className="px-4 py-4">
          <div className="font-medium" style={{ color: 'var(--text-primary)' }}>
            {entry.email}
          </div>
          <div className="mt-1 text-xs" style={{ color: 'var(--text-tertiary)' }}>
            {entry.id}
          </div>
          <div className="mt-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
            {isZh ? '最近撤销会话：' : 'Last forced sign-out: '}
            {formatTime(entry.session_revoked_at, locale)}
          </div>
        </td>
        <td className="px-4 py-4">
          <div className="flex flex-wrap gap-2">
            <span
              className="inline-flex rounded-full px-2.5 py-1 text-xs font-semibold"
              style={{
                background: entry.is_admin
                  ? 'color-mix(in srgb, var(--accent-primary) 12%, transparent)'
                  : 'rgba(148, 163, 184, 0.12)',
                color: entry.is_admin ? 'var(--accent-primary)' : 'var(--text-secondary)',
              }}
            >
              {entry.is_admin ? (isZh ? '管理员' : 'Admin') : isZh ? '用户' : 'User'}
            </span>
            <span
              className="inline-flex rounded-full px-2.5 py-1 text-xs font-semibold"
              style={{
                background: entry.is_archived
                  ? 'rgba(99, 102, 241, 0.12)'
                  : entry.is_active
                    ? 'rgba(34, 197, 94, 0.12)'
                    : 'rgba(239, 68, 68, 0.12)',
                color: entry.is_archived
                  ? 'rgb(79, 70, 229)'
                  : entry.is_active
                    ? 'rgb(22, 163, 74)'
                    : 'rgb(220, 38, 38)',
              }}
            >
              {entry.is_archived
                ? isZh ? '已归档' : 'Archived'
                : entry.is_active
                  ? isZh ? '已启用' : 'Enabled'
                  : isZh ? '已停用' : 'Disabled'}
            </span>
          </div>
        </td>
        <td className="px-4 py-4" style={{ color: 'var(--text-secondary)' }}>
          {entry.trader_count}
          <span className="ml-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
            {isZh ? `运行中 ${entry.running_traders}` : `${entry.running_traders} running`}
          </span>
        </td>
        <td className="px-4 py-4" style={{ color: 'var(--text-secondary)' }}>
          {entry.model_count}
          <span className="ml-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
            {isZh ? `启用 ${entry.enabled_models}` : `${entry.enabled_models} enabled`}
          </span>
        </td>
        <td className="px-4 py-4" style={{ color: 'var(--text-secondary)' }}>
          {entry.exchange_count}
          <span className="ml-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
            {isZh ? `启用 ${entry.enabled_exchanges}` : `${entry.enabled_exchanges} enabled`}
          </span>
        </td>
        <td className="px-4 py-4" style={{ color: 'var(--text-secondary)' }}>
          {formatTime(entry.created_at, locale)}
        </td>
        <td className="px-4 py-4 min-w-[360px]">
          {entry.is_admin ? (
            <div className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
              {isZh ? '管理员账户受保护' : 'Protected admin account'}
            </div>
          ) : (
            <div className="space-y-3">
              <div className="flex flex-wrap gap-2">
                <button
                  onClick={() => handleUserStatusToggle(entry)}
                  disabled={rowLoading || entry.is_archived}
                  className="inline-flex items-center justify-center rounded-xl px-3 py-2 text-xs font-semibold transition disabled:opacity-60"
                  style={{
                    background: entry.is_active
                      ? 'rgba(239, 68, 68, 0.12)'
                      : 'rgba(34, 197, 94, 0.12)',
                    color: entry.is_active ? 'rgb(220, 38, 38)' : 'rgb(22, 163, 74)',
                  }}
                >
                  {rowLoading
                    ? isZh ? '处理中...' : 'Saving...'
                    : entry.is_active
                      ? isZh ? '停用账户' : 'Disable'
                      : isZh ? '启用账户' : 'Enable'}
                </button>
                <button
                  onClick={() => handleRevokeSessions(entry)}
                  disabled={rowLoading}
                  className="inline-flex items-center gap-2 rounded-xl px-3 py-2 text-xs font-semibold transition disabled:opacity-60"
                  style={{
                    background: 'rgba(245, 158, 11, 0.12)',
                    color: 'rgb(217, 119, 6)',
                  }}
                >
                  <LogOut className="h-3.5 w-3.5" />
                  {isZh ? '强制下线' : 'Sign out'}
                </button>
                <button
                  onClick={() => handleArchiveToggle(entry)}
                  disabled={rowLoading}
                  className="inline-flex items-center gap-2 rounded-xl px-3 py-2 text-xs font-semibold transition disabled:opacity-60"
                  style={{
                    background: 'rgba(99, 102, 241, 0.12)',
                    color: 'rgb(79, 70, 229)',
                  }}
                >
                  <Archive className="h-3.5 w-3.5" />
                  {entry.is_archived
                    ? isZh ? '恢复账户' : 'Restore'
                    : isZh ? '归档账户' : 'Archive'}
                </button>
                <button
                  onClick={() => handleDeleteUser(entry)}
                  disabled={rowLoading}
                  className="inline-flex items-center gap-2 rounded-xl px-3 py-2 text-xs font-semibold transition disabled:opacity-60"
                  style={{
                    background: 'rgba(239, 68, 68, 0.12)',
                    color: 'rgb(220, 38, 38)',
                  }}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                  {isZh ? '删除用户' : 'Delete'}
                </button>
              </div>

              <div className="flex flex-col gap-2">
                <input
                  type="password"
                  value={passwordDrafts[entry.id] || ''}
                  onChange={(e) =>
                    setPasswordDrafts((current) => ({
                      ...current,
                      [entry.id]: e.target.value,
                    }))
                  }
                  placeholder={isZh ? '输入新密码（至少 8 位）' : 'New password (min 8 chars)'}
                  className="rounded-xl border px-3 py-2 text-sm outline-none"
                  style={{
                    background: 'var(--panel-bg)',
                    borderColor: 'var(--panel-border)',
                    color: 'var(--text-primary)',
                  }}
                />
                <button
                  onClick={() => handleResetPassword(entry)}
                  disabled={rowLoading}
                  className="inline-flex items-center gap-2 rounded-xl px-3 py-2 text-xs font-semibold transition disabled:opacity-60"
                  style={{
                    background: 'color-mix(in srgb, var(--accent-primary) 14%, transparent)',
                    color: 'var(--accent-primary)',
                  }}
                >
                  <KeyRound className="h-3.5 w-3.5" />
                  {isZh ? '重置密码并撤销旧会话' : 'Reset password'}
                </button>
              </div>
            </div>
          )}
        </td>
      </tr>
    )
  }

  return (
    <main className="admin-console-page px-4 pb-10 pt-6 sm:px-6 lg:px-8 lg:pt-20">
      <div className="mx-auto max-w-[1680px] space-y-6">
        <section
          className="rounded-[28px] border p-6 sm:p-8"
          style={{
            background:
              'linear-gradient(140deg, var(--panel-bg-solid), color-mix(in srgb, var(--panel-bg) 78%, transparent))',
            borderColor: 'var(--panel-border)',
            boxShadow: '0 24px 70px rgba(15, 23, 42, 0.08)',
          }}
        >
          <div className="flex flex-col gap-6 xl:flex-row xl:items-end xl:justify-between">
            <div className="space-y-3">
              <div
                className="inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-semibold uppercase tracking-[0.16em]"
                style={{
                  borderColor:
                    'color-mix(in srgb, var(--accent-primary) 30%, var(--panel-border))',
                  color: 'var(--accent-primary)',
                  background: 'color-mix(in srgb, var(--accent-primary) 10%, transparent)',
                }}
              >
                <ShieldCheck className="h-3.5 w-3.5" />
                {isZh ? '管理员控制台' : 'Admin Console'}
              </div>
              <div>
                <h1
                  className="text-3xl font-semibold tracking-tight sm:text-4xl"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {isZh ? '平台治理与运行总览' : 'Platform governance and runtime overview'}
                </h1>
                <p className="mt-3 max-w-3xl text-sm sm:text-base" style={{ color: 'var(--text-secondary)' }}>
                  {isZh
                    ? '管理员内部工作台已与普通用户界面隔离，用于治理访问、控制用户、追踪操作与监控平台运行状态。'
                    : 'The administrative control plane is isolated from standard users and focuses on access, operations, user control, and traceability.'}
                </p>
              </div>
            </div>

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:min-w-[420px]">
              <div
                className="rounded-2xl border p-4"
                style={{ background: 'var(--panel-bg)', borderColor: 'var(--panel-border)' }}
              >
                <div className="flex items-center gap-3">
                  <div
                    className="flex h-10 w-10 items-center justify-center rounded-2xl"
                    style={{
                      background:
                        healthTone === 'healthy'
                          ? 'rgba(34, 197, 94, 0.12)'
                          : 'rgba(245, 158, 11, 0.12)',
                      color:
                        healthTone === 'healthy'
                          ? 'rgb(22, 163, 74)'
                          : 'rgb(217, 119, 6)',
                    }}
                  >
                    <Activity className="h-5 w-5" />
                  </div>
                  <div>
                    <div className="text-xs uppercase tracking-[0.14em]" style={{ color: 'var(--text-tertiary)' }}>
                      {isZh ? '平台状态' : 'Platform status'}
                    </div>
                    <div className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
                      {runtimeLabel}
                    </div>
                  </div>
                </div>
              </div>
              <div
                className="rounded-2xl border p-4"
                style={{ background: 'var(--panel-bg)', borderColor: 'var(--panel-border)' }}
              >
                <div className="flex items-center gap-3">
                  <div
                    className="flex h-10 w-10 items-center justify-center rounded-2xl"
                    style={{
                      background: 'color-mix(in srgb, var(--accent-primary) 12%, transparent)',
                      color: 'var(--accent-primary)',
                    }}
                  >
                    <Clock3 className="h-5 w-5" />
                  </div>
                  <div>
                    <div className="text-xs uppercase tracking-[0.14em]" style={{ color: 'var(--text-tertiary)' }}>
                      {isZh ? '服务器时间' : 'Server time'}
                    </div>
                    <div className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                      {overview ? formatTime(overview.server_time, locale) : '--'}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className="grid gap-4 xl:grid-cols-[220px_minmax(0,1fr)]">
          <aside
            className="rounded-3xl border p-3"
            style={{ background: 'var(--panel-bg-solid)', borderColor: 'var(--panel-border)' }}
          >
            <div className="mb-3 px-3 pt-2 text-xs font-semibold uppercase tracking-[0.14em]" style={{ color: 'var(--text-tertiary)' }}>
              {isZh ? '控制平面' : 'Control plane'}
            </div>
            <div className="space-y-2">
              {sections.map((section) => {
                const Icon = section.icon
                const active = activeSection === section.id
                return (
                  <button
                    key={section.id}
                    onClick={() => setActiveSection(section.id)}
                    className="flex w-full items-center gap-3 rounded-2xl px-4 py-3 text-left text-sm font-medium transition"
                    style={{
                      background: active
                        ? 'color-mix(in srgb, var(--accent-primary) 12%, transparent)'
                        : 'transparent',
                      color: active ? 'var(--accent-primary)' : 'var(--text-secondary)',
                    }}
                  >
                    <Icon className="h-4 w-4" />
                    {section.label}
                  </button>
                )
              })}
            </div>
          </aside>

          <div className="space-y-6">
            {(activeSection === 'overview' || activeSection === 'access') && (
              <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                {statCards.map((card) => {
                  const Icon = card.icon
                  return (
                    <div
                      key={card.title}
                      className="rounded-3xl border p-5"
                      style={{ background: 'var(--panel-bg-solid)', borderColor: 'var(--panel-border)' }}
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div>
                          <div className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                            {card.title}
                          </div>
                          <div className="mt-3 text-3xl font-semibold" style={{ color: 'var(--text-primary)' }}>
                            {overviewLoading ? '--' : card.value}
                          </div>
                          <div className="mt-2 text-sm" style={{ color: 'var(--text-tertiary)' }}>
                            {card.detail}
                          </div>
                        </div>
                        <div
                          className="flex h-11 w-11 items-center justify-center rounded-2xl"
                          style={{
                            background: 'color-mix(in srgb, var(--accent-primary) 12%, transparent)',
                            color: 'var(--accent-primary)',
                          }}
                        >
                          <Icon className="h-5 w-5" />
                        </div>
                      </div>
                    </div>
                  )
                })}
              </section>
            )}

            {activeSection === 'overview' && (
              <section
                className="rounded-3xl border p-6"
                style={{ background: 'var(--panel-bg-solid)', borderColor: 'var(--panel-border)' }}
              >
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h2 className="text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>
                      {isZh ? '平台运行监控' : 'Platform runtime monitor'}
                    </h2>
                    <p className="mt-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
                      {isZh
                        ? '用于快速判断部署环境、加密链路和外部依赖状态。'
                        : 'A compact view of deployment posture, transport security, and external dependency isolation.'}
                    </p>
                  </div>
                  <div className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
                    {usersLoading
                      ? isZh ? '加载中...' : 'Loading...'
                      : isZh
                        ? `活跃 ${activeUsers.length} / 归档 ${archivedUsers.length}`
                        : `${activeUsers.length} active / ${archivedUsers.length} archived`}
                  </div>
                </div>

                <div className="mt-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                  {runtimeCards.map((item) => {
                    const Icon = item.icon
                    return (
                      <div
                        key={item.label}
                        className="rounded-2xl border p-4"
                        style={{ background: 'var(--panel-bg)', borderColor: 'var(--panel-border)' }}
                      >
                        <div className="flex items-center gap-3">
                          <div
                            className="flex h-10 w-10 items-center justify-center rounded-2xl"
                            style={{
                              background: 'color-mix(in srgb, var(--accent-primary) 12%, transparent)',
                              color: 'var(--accent-primary)',
                            }}
                          >
                            <Icon className="h-5 w-5" />
                          </div>
                          <div>
                            <div className="text-xs uppercase tracking-[0.12em]" style={{ color: 'var(--text-tertiary)' }}>
                              {item.label}
                            </div>
                            <div className="mt-1 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                              {item.value}
                            </div>
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>

                <div
                  className="mt-4 rounded-2xl border p-4"
                  style={{ background: 'var(--panel-bg)', borderColor: 'var(--panel-border)' }}
                >
                  <div className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                    {isZh ? '管理员安全基线' : 'Administrator security baseline'}
                  </div>
                  <div className="mt-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
                    {overview?.admin_password_configured
                      ? isZh
                        ? '管理员独立密码已配置，后台入口与普通用户入口已隔离。'
                        : 'A dedicated admin password is configured and the control-plane entry is isolated from standard users.'
                      : isZh
                        ? '管理员密码未配置，这会削弱后台安全边界。'
                        : 'Admin password is not configured, which weakens the control-plane boundary.'}
                  </div>
                </div>
              </section>
            )}

            {activeSection === 'access' && (
              <section
                className="rounded-3xl border p-6"
                style={{ background: 'var(--panel-bg-solid)', borderColor: 'var(--panel-border)' }}
              >
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h2 className="text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>
                      {isZh ? '注册与访问控制' : 'Registration and access control'}
                    </h2>
                    <p className="mt-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
                      {isZh
                        ? '管理员可在这里控制新用户注册开关，并核对整体访问边界。'
                        : 'Control self-service registration and verify the platform access boundary.'}
                    </p>
                  </div>
                  <UserCog className="h-5 w-5" style={{ color: 'var(--accent-primary)' }} />
                </div>

                <div
                  className="mt-6 rounded-2xl border p-4"
                  style={{ background: 'var(--panel-bg)', borderColor: 'var(--panel-border)' }}
                >
                  <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <div className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                        {isZh ? '新用户注册' : 'New user registration'}
                      </div>
                      <div className="mt-1 text-sm" style={{ color: 'var(--text-secondary)' }}>
                        {overview?.registration_enabled
                          ? isZh
                            ? '当前开放。新用户可以通过注册页创建账号。'
                            : 'Currently open. New users can create accounts from the register page.'
                          : isZh
                            ? '当前关闭。仅管理员或已有账号可进入平台。'
                            : 'Currently closed. Only administrators or existing users can access the platform.'}
                      </div>
                    </div>
                    <button
                      onClick={handleRegistrationToggle}
                      disabled={registrationSaving || !overview}
                      className="inline-flex items-center justify-center rounded-2xl px-4 py-2 text-sm font-semibold transition disabled:opacity-60"
                      style={{
                        background: overview?.registration_enabled
                          ? 'rgba(239, 68, 68, 0.12)'
                          : 'color-mix(in srgb, var(--accent-primary) 14%, transparent)',
                        color: overview?.registration_enabled
                          ? 'rgb(220, 38, 38)'
                          : 'var(--accent-primary)',
                        border: '1px solid',
                        borderColor: overview?.registration_enabled
                          ? 'rgba(239, 68, 68, 0.18)'
                          : 'color-mix(in srgb, var(--accent-primary) 28%, var(--panel-border))',
                      }}
                    >
                      {registrationSaving
                        ? isZh ? '处理中...' : 'Saving...'
                        : overview?.registration_enabled
                          ? isZh ? '关闭注册' : 'Disable signup'
                          : isZh ? '开放注册' : 'Enable signup'}
                    </button>
                  </div>
                </div>
              </section>
            )}

            {activeSection === 'users' && (
              <section
                className="rounded-3xl border p-6"
                style={{ background: 'var(--panel-bg-solid)', borderColor: 'var(--panel-border)' }}
              >
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <h2 className="text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>
                      {isZh ? '用户运营管理' : 'User operations'}
                    </h2>
                    <p className="mt-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
                      {isZh
                        ? '支持启停账户、强制下线、归档、密码重置和安全删除。'
                        : 'Enable, disable, sign out, archive, reset passwords, and safely remove accounts.'}
                    </p>
                  </div>
                  <div className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
                    {usersLoading ? (isZh ? '加载中...' : 'Loading...') : `${users.length} ${isZh ? '个账户' : 'accounts'}`}
                  </div>
                </div>

                <div className="mt-6 overflow-hidden rounded-2xl border" style={{ borderColor: 'var(--panel-border)' }}>
                  <div className="overflow-x-auto">
                    <table className="min-w-full text-sm">
                      <thead style={{ background: 'var(--panel-bg)' }}>
                        <tr>
                          {[
                            isZh ? '用户' : 'User',
                            isZh ? '状态' : 'Status',
                            isZh ? '交易资源' : 'Trading resources',
                            isZh ? '模型资源' : 'Model resources',
                            isZh ? '交易所资源' : 'Exchange resources',
                            isZh ? '创建时间' : 'Created',
                            isZh ? '操作' : 'Actions',
                          ].map((label) => (
                            <th
                              key={label}
                              className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-[0.12em]"
                              style={{ color: 'var(--text-tertiary)' }}
                            >
                              {label}
                            </th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {users.map(renderUserRow)}
                        {!usersLoading && users.length === 0 && (
                          <tr>
                            <td colSpan={7} className="px-4 py-12 text-center" style={{ color: 'var(--text-tertiary)' }}>
                              {isZh ? '当前没有可显示的用户记录。' : 'No user records are available.'}
                            </td>
                          </tr>
                        )}
                      </tbody>
                    </table>
                  </div>
                </div>
              </section>
            )}

            {activeSection === 'audit' && (
              <section
                className="rounded-3xl border p-6"
                style={{ background: 'var(--panel-bg-solid)', borderColor: 'var(--panel-border)' }}
              >
                <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
                  <div>
                    <h2 className="text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>
                      {isZh ? '最近审计记录' : 'Recent audit trail'}
                    </h2>
                    <p className="mt-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
                      {isZh
                        ? '管理员关键操作会留痕，可按动作类型和关键词筛选。'
                        : 'Administrative control-plane actions are traceable and filterable by action and keyword.'}
                    </p>
                  </div>
                  <div className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
                    {auditLoading ? (isZh ? '加载中...' : 'Loading...') : `${auditLogs.length} ${isZh ? '条记录' : 'records'}`}
                  </div>
                </div>

                <div className="mt-6 grid gap-3 lg:grid-cols-[220px_minmax(0,1fr)]">
                  <select
                    value={auditActionFilter}
                    onChange={(e) => setAuditActionFilter(e.target.value)}
                    className="rounded-2xl border px-4 py-3 text-sm outline-none"
                    style={{
                      background: 'var(--panel-bg)',
                      borderColor: 'var(--panel-border)',
                      color: 'var(--text-primary)',
                    }}
                  >
                    <option value="">{isZh ? '全部动作' : 'All actions'}</option>
                    {auditActionOptions.map((action) => (
                      <option key={action} value={action}>
                        {action}
                      </option>
                    ))}
                  </select>
                  <label
                    className="flex items-center gap-3 rounded-2xl border px-4 py-3"
                    style={{ background: 'var(--panel-bg)', borderColor: 'var(--panel-border)' }}
                  >
                    <Search className="h-4 w-4" style={{ color: 'var(--text-tertiary)' }} />
                    <input
                      value={auditQuery}
                      onChange={(e) => setAuditQuery(e.target.value)}
                      placeholder={isZh ? '搜索操作者、目标、摘要' : 'Search actor, target, or summary'}
                      className="w-full bg-transparent text-sm outline-none"
                      style={{ color: 'var(--text-primary)' }}
                    />
                  </label>
                </div>

                <div className="mt-6 overflow-hidden rounded-2xl border" style={{ borderColor: 'var(--panel-border)' }}>
                  <div className="overflow-x-auto">
                    <table className="min-w-full text-sm">
                      <thead style={{ background: 'var(--panel-bg)' }}>
                        <tr>
                          {[isZh ? '时间' : 'Time', isZh ? '操作者' : 'Actor', isZh ? '动作' : 'Action', isZh ? '目标' : 'Target', isZh ? '摘要' : 'Summary'].map((label) => (
                            <th
                              key={label}
                              className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-[0.12em]"
                              style={{ color: 'var(--text-tertiary)' }}
                            >
                              {label}
                            </th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {auditLogs.map((entry) => (
                          <tr key={entry.id} className="border-t" style={{ borderColor: 'var(--panel-border)' }}>
                            <td className="px-4 py-4" style={{ color: 'var(--text-secondary)' }}>
                              {formatTime(entry.created_at, locale)}
                            </td>
                            <td className="px-4 py-4" style={{ color: 'var(--text-primary)' }}>
                              {entry.actor_email}
                            </td>
                            <td className="px-4 py-4">
                              <span
                                className="inline-flex rounded-full px-2.5 py-1 text-xs font-semibold"
                                style={{
                                  background: 'color-mix(in srgb, var(--accent-primary) 12%, transparent)',
                                  color: 'var(--accent-primary)',
                                }}
                              >
                                {entry.action}
                              </span>
                            </td>
                            <td className="px-4 py-4" style={{ color: 'var(--text-secondary)' }}>
                              {entry.target_type}:{entry.target_id}
                            </td>
                            <td className="px-4 py-4" style={{ color: 'var(--text-secondary)' }}>
                              {entry.summary}
                            </td>
                          </tr>
                        ))}
                        {!auditLoading && auditLogs.length === 0 && (
                          <tr>
                            <td colSpan={5} className="px-4 py-10 text-center" style={{ color: 'var(--text-tertiary)' }}>
                              {isZh ? '当前筛选条件下没有审计记录。' : 'No audit records match the current filters.'}
                            </td>
                          </tr>
                        )}
                      </tbody>
                    </table>
                  </div>
                </div>
              </section>
            )}
          </div>
        </section>
      </div>
    </main>
  )
}
