import type { Locale } from '@/lib/locale'

export interface UiCopy {
  close: string
  dismiss: string
  docs: string
  docsTopic: string
}

export const UI_COPY: Record<Locale, UiCopy> = {
  en: {
    close: 'Close',
    dismiss: 'Dismiss',
    docs: 'Documentation',
    docsTopic: 'Documentation: {topic}',
  },
  ru: {
    close: 'Закрыть',
    dismiss: 'Закрыть',
    docs: 'Документация',
    docsTopic: 'Документация: {topic}',
  },
}
