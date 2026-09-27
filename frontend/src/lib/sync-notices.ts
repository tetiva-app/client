import { plural, type Locale, type PluralForms } from '@/lib/locale'

export interface SyncNotice {
  message: string
  showPlans: boolean
}

// Rejections the engine reports through `sync:quota_exceeded`; unknown kinds stay
// silent so a newer server cannot spam the UI with untranslated text.
export function quotaNotice(kind: string): SyncNotice | null {
  switch (kind) {
    case 'cloud_collections':
      return {
        message: 'Cloud collection limit reached (20 on Free). Saved locally, not synced.',
        showPlans: true,
      }
    case 'members':
      return {
        message: "Sync paused: the team exceeds its plan's member limit. Ask the owner to update the plan or remove members.",
        showPlans: true,
      }
    default:
      return null
  }
}

export function rejectNotice(reason: string): SyncNotice | null {
  switch (reason) {
    case 'id_conflict':
      return {
        message: 'Some items could not be synced: they already exist in another cloud workspace this workspace was linked to before. Re-link to the original workspace.',
        showPlans: false,
      }
    case 'too_large':
      return {
        message: 'Too large to sync: one item is bigger than the server accepts. It stays local and is retried later; shorten its body, scripts or description.',
        showPlans: false,
      }
    default:
      return null
  }
}

export const PARKED_COPY: Record<Locale, { quota: PluralForms; tooLarge: PluralForms }> = {
  en: {
    quota: { one: '{n} change not synced — plan limit', other: '{n} changes not synced — plan limit' },
    tooLarge: { one: '{n} item too large for the server', other: '{n} items too large for the server' },
  },
  ru: {
    quota: {
      one: '{n} изменение не синхронизировано\u00a0— лимит тарифа',
      few: '{n} изменения не синхронизированы\u00a0— лимит тарифа',
      many: '{n} изменений не синхронизировано\u00a0— лимит тарифа',
      other: '{n} изменения не синхронизированы\u00a0— лимит тарифа',
    },
    tooLarge: {
      one: '{n} элемент слишком велик для сервера',
      few: '{n} элемента слишком велики для сервера',
      many: '{n} элементов слишком велики для сервера',
      other: '{n} элемента слишком велики для сервера',
    },
  },
}

export function parkedSummary(quota: number, tooLarge: number, locale: Locale = 'en'): string {
  const copy = PARKED_COPY[locale]
  const parts: string[] = []
  if (quota > 0) parts.push(plural(locale, quota, copy.quota))
  if (tooLarge > 0) parts.push(plural(locale, tooLarge, copy.tooLarge))
  return parts.join(' · ')
}
