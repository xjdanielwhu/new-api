import { Link } from '@tanstack/react-router'
import { ArrowRight, BookOpen } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { HeroTerminalDemo } from '../hero-terminal-demo'

interface HeroProps {
  className?: string
  isAuthenticated?: boolean
}

function MoreIcon() {
  return (
    <svg viewBox='0 0 24 24' className='size-4' fill='currentColor'>
      <circle cx='5' cy='12' r='2' />
      <circle cx='12' cy='12' r='2' />
      <circle cx='19' cy='12' r='2' />
    </svg>
  )
}

export function Hero(props: HeroProps) {
  const { t } = useTranslation()

  return (
    <section className='relative z-10 flex flex-col items-center overflow-hidden px-6 pt-24 pb-16 md:pt-32 md:pb-24 lg:pt-36 lg:pb-28'>
      {/* Colorful ambient blobs */}
      <div
        aria-hidden
        className='pointer-events-none absolute inset-0 -z-10 opacity-25 dark:opacity-[0.12]'
        style={{
          background:
            'radial-gradient(60% 50% at 20% 20%, oklch(0.72 0.18 250 / 0.8) 0%, rgba(0, 0, 0, 0) 70%), radial-gradient(50% 40% at 80% 15%, oklch(0.65 0.15 200 / 0.6) 0%, rgba(0, 0, 0, 0) 70%), radial-gradient(40% 35% at 40% 80%, oklch(0.7 0.12 280 / 0.4) 0%, rgba(0, 0, 0, 0) 70%)',
        }}
      />

      {/* Subtle grid overlay */}
      <div
        aria-hidden
        className='absolute inset-0 -z-10 bg-[linear-gradient(to_right,var(--border)_1px,transparent_1px),linear-gradient(to_bottom,var(--border)_1px,transparent_1px)] bg-[size:4rem_4rem] opacity-[0.08] [mask-image:radial-gradient(ellipse_60%_50%_at_50%_30%,black_20%,transparent_100%)]'
      />

      <div className='mx-auto grid w-full max-w-6xl grid-cols-1 items-start gap-12 md:grid-cols-2 md:gap-8'>
        <div className='flex w-full min-w-0 flex-col items-center text-center md:items-start md:text-left'>
          <div className='landing-animate-fade-up mb-5 inline-flex items-center gap-1.5 rounded-full border border-blue-500/20 bg-blue-500/5 px-3 py-1.5 text-[11px] font-medium text-blue-600 shadow-xs dark:border-blue-400/20 dark:bg-blue-400/5 dark:text-blue-400'>
            <span className='relative flex h-2 w-2'>
              <span className='animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75'></span>
              <span className='relative inline-flex rounded-full h-2 w-2 bg-blue-500'></span>
            </span>
            {t('AI Application Infrastructure Foundation')}
          </div>

          <h1 className='text-[clamp(2.25rem,4.5vw,3.25rem)] leading-[1.15] font-bold tracking-tight text-foreground'>
            <span>{t('Unified API Gateway for')}</span>
            <br />
            <span className='bg-gradient-to-r from-blue-500 via-violet-500 to-purple-500 bg-clip-text text-transparent'>
              {t('Massive AI Models')}
            </span>
          </h1>

          <p className='mt-5 max-w-xl text-base leading-relaxed text-muted-foreground/80 md:text-[15px]'>
            {t(
              'Connect to massive models through unified, standard interface protocols. Power AI applications, manage digital assets efficiently, and connect to the future.'
            )}
          </p>

          <div className='mt-8 flex flex-wrap items-center justify-center gap-3 md:justify-start'>
            {props.isAuthenticated ? (
              <Button
                size='lg'
                className='group h-11 rounded-[1rem] px-5 text-sm font-medium'
                render={<Link to='/dashboard' />}
              >
                {t('Go to Dashboard')}
                <ArrowRight className='ml-2 size-4 transition-transform duration-200 group-hover:translate-x-1' />
              </Button>
            ) : (
              <>
                <Button
                  size='lg'
                  className='group h-11 rounded-[1rem] px-5 text-sm font-medium'
                  render={<Link to='/sign-up' />}
                >
                  {t('Get Started')}
                  <ArrowRight className='ml-2 size-4 transition-transform duration-200 group-hover:translate-x-1' />
                </Button>
                <Button
                  size='lg'
                  variant='outline'
                  className='h-11 rounded-[1rem] border-border/50 bg-background px-5 text-sm font-medium hover:bg-muted/50'
                  render={<Link to='/pricing' />}
                >
                  {t('View Pricing')}
                </Button>
                <a
                  href='https://coai-api.apifox.cn/'
                  target='_blank'
                  rel='noopener noreferrer'
                  className='inline-flex h-11 items-center justify-center gap-2 rounded-[1rem] border border-border/50 bg-background px-5 text-sm font-medium transition-colors hover:bg-muted/50'
                >
                  <BookOpen className='size-4' />
                  {t('Documentation')}
                </a>
              </>
            )}
          </div>

          <div className='mt-10 w-full max-w-xl'>
            <div className='mb-4 flex flex-col gap-1'>
              <span className='text-[10px] font-bold tracking-[0.15em] uppercase text-muted-foreground/50'>
                {t('Common App Support')}
              </span>
              <p className='text-xs leading-relaxed text-muted-foreground/60'>
                {t('Supports one-click configuration and perfectly adapts to NewAPI multi-protocol configuration')}
              </p>
            </div>
            <div className='flex flex-wrap items-center justify-center gap-3 md:justify-start'>
              <a
                href='https://cherry-ai.com/'
                target='_blank'
                rel='noopener noreferrer'
                className='group flex items-center gap-3 rounded-full border border-border/40 bg-muted/15 px-5 py-2.5 text-sm font-medium text-foreground/80 shadow-[0_1px_2.5px_rgba(0,0,0,0.01)] backdrop-blur-xs transition-all duration-300 hover:scale-[1.02] hover:border-border hover:bg-muted/30 hover:text-foreground'
              >
                <img
                  src='/cherry-studio-icon.png'
                  alt='Cherry Studio'
                  className='size-5 rounded-md object-contain'
                />
                Cherry Studio
              </a>
              <a
                href='https://ccswitch.io/'
                target='_blank'
                rel='noopener noreferrer'
                className='group flex items-center gap-3 rounded-full border border-border/40 bg-muted/15 px-5 py-2.5 text-sm font-medium text-foreground/80 shadow-[0_1px_2.5px_rgba(0,0,0,0.01)] backdrop-blur-xs transition-all duration-300 hover:scale-[1.02] hover:border-border hover:bg-muted/30 hover:text-foreground'
              >
                <img
                  src='/cc-switch-icon.ico'
                  alt='CC Switch'
                  className='size-5 rounded-md object-contain'
                />
                CC Switch
              </a>
              <button className='group flex cursor-default items-center gap-2.5 rounded-full border border-border/40 bg-muted/15 px-5 py-2.5 text-sm font-medium text-foreground/55 shadow-[0_1px_2.5px_rgba(0,0,0,0.01)] backdrop-blur-xs transition-all duration-300 hover:scale-[1.02] hover:border-border hover:bg-muted/30 hover:text-foreground'>
                <MoreIcon />
                {t('More')}
              </button>
            </div>
          </div>
        </div>

        <div className='flex w-full min-w-0 justify-center'>
          <HeroTerminalDemo />
        </div>
      </div>
    </section>
  )
}
