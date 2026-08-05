import { useSystemConfigStore } from '@/stores/system-config-store'
import { useTranslation } from 'react-i18next'

interface AppSupportBadgeProps {
  className?: string
}

export function AppSupportBadge(props: AppSupportBadgeProps) {
  const { t } = useTranslation()
  const { config } = useSystemConfigStore()

  const chats = (config as unknown as { chats?: Array<Record<string, string>> }).chats || []
  
  const displayApps = chats.slice(0, 4)

  return (
    <div className={`mt-8 ${props.className || ''}`}>
      <div className='flex items-center gap-2 mb-4'>
        <span className='inline-flex items-center gap-1.5 rounded-full bg-blue-500/10 px-2.5 py-1 text-xs font-medium text-blue-600 dark:bg-blue-400/10 dark:text-blue-400'>
          <span className='relative flex h-1.5 w-1.5'>
            <span className='animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75'></span>
            <span className='relative inline-flex rounded-full h-1.5 w-1.5 bg-blue-500'></span>
          </span>
          {t('AI Application Support')}
        </span>
      </div>
      <div className='flex flex-wrap items-center gap-2'>
        {displayApps.map((app, index) => {
          const name = Object.keys(app)[0]
          return (
            <span
              key={index}
              className='rounded-lg border border-border/40 px-3 py-1.5 text-xs font-medium text-foreground/70 transition-colors hover:bg-muted/50 hover:text-foreground'
            >
              {name}
            </span>
          )
        })}
        {chats.length > 4 && (
          <span className='rounded-lg border border-border/40 px-3 py-1.5 text-xs font-medium text-foreground/50'>
            +{chats.length - 4} {t('more')}
          </span>
        )}
      </div>
    </div>
  )
}
