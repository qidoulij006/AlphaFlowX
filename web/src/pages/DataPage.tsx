import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'

export function DataPage() {
  const { language } = useLanguage()

  return (
    <div className="w-full min-h-[calc(100vh-64px)] px-6 py-10 flex items-center justify-center">
      <div
        className="w-full max-w-3xl rounded-3xl p-8 md:p-10 text-center"
        style={{
          background: 'var(--panel-bg-solid)',
          border: '1px solid var(--panel-border)',
          boxShadow: 'var(--shadow-soft)',
        }}
      >
        <div className="text-sm font-semibold mb-3" style={{ color: 'var(--text-primary)' }}>
          {t('dataCenter', language)}
        </div>
        <h1 className="text-2xl md:text-3xl font-semibold mb-4" style={{ color: 'var(--text-primary)' }}>
          {language === 'zh' ? '外部数据看板已停用' : 'External data terminal disabled'}
        </h1>
        <p className="text-sm md:text-base leading-7" style={{ color: 'var(--text-secondary)' }}>
          {language === 'zh'
            ? '当前部署已切断全部外部量化数据调用。此页面不再加载第三方数据看板。'
            : 'This deployment has disabled all outbound quant data calls. This page no longer loads the third-party data terminal.'}
        </p>
      </div>
    </div>
  )
}
