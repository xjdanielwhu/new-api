import { Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { AnimateInView } from '@/components/animate-in-view'

interface CTAProps {
  className?: string
  isAuthenticated?: boolean
}

export function CTA(props: CTAProps) {
  const { t } = useTranslation()

  if (props.isAuthenticated) {
    return null
  }

  return (
    <section className='relative z-10 overflow-hidden px-6 py-28 md:py-36'>
      <div
        aria-hidden
        className='pointer-events-none absolute inset-0 -z-10'
        style={{
          background:
            'radial-gradient(ellipse 60% 50% at 30% 50%, rgba(219, 234, 254, 0.5) 0%, transparent 70%), radial-gradient(ellipse 50% 40% at 70% 40%, rgba(167, 139, 250, 0.25) 0%, transparent 70%)',
        }}
      />

      <div className='mx-auto w-full max-w-6xl'>
        <AnimateInView className='text-center' animation='scale-in'>
          <h2 className='text-2xl leading-tight font-bold tracking-tight md:text-3xl'>
            {t('Ready to simplify')}
            <br />
            <span className='bg-gradient-to-r from-blue-500 via-violet-500 to-purple-500 bg-clip-text text-transparent'>
              {t('your AI integration?')}
            </span>
          </h2>
          <p className='mx-auto mt-5 max-w-xl text-base leading-relaxed text-muted-foreground/80 md:text-[15px]'>
            {t(
              'Deploy your own gateway and start routing requests through your configured upstream services.'
            )}
          </p>
          <div className='mt-8 flex flex-wrap items-center justify-center gap-3'>
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
          </div>
        </AnimateInView>
      </div>
    </section>
  )
}
