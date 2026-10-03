import type { UpdateState } from '@/services'
import type { UpdateCardPrefs } from '@/lib/settings-storage'

export type CardMode = 'hidden' | 'card' | 'line'
export type CardVariant = 'ready' | 'available' | 'external' | 'failed' | 'apt' | 'sync'

export interface CardInput {
  state: UpdateState
  downloadAuto: boolean
  prefs: UpdateCardPrefs | null
  nowMs: number
  onboardingDone: boolean
  whatsNewShownThisSession: boolean
  syncUpdateRequired: boolean
}

export interface CardView {
  mode: CardMode
  variant: CardVariant | null
  fresh: boolean
  escalate: boolean
}

export const ESCALATE_AFTER_MS = 7 * 24 * 60 * 60 * 1000

const HIDDEN: CardView = { mode: 'hidden', variant: null, fresh: false, escalate: false }

export function variantOf(s: UpdateState, downloadAuto: boolean): CardVariant | null {
  if (s.reason === 'install_failed' && (s.phase === 'ready' || s.phase === 'error')) return 'failed'
  if (s.phase === 'ready') return 'ready'
  if (s.phase !== 'available') return null
  switch (s.install) {
    case 'in_app': return downloadAuto ? null : 'available'
    case 'unsupported': return 'external'
    case 'apt':
    case 'apt_not_configured': return 'apt'
  }
  return null
}

export function cardView(i: CardInput): CardView {
  const { state, prefs } = i
  if (i.syncUpdateRequired) {
    const mode = prefs?.collapsed && state.phase === 'up_to_date' ? 'line' : 'card'
    return { mode, variant: 'sync', fresh: false, escalate: false }
  }
  if (!i.onboardingDone || i.whatsNewShownThisSession) return HIDDEN
  const variant = variantOf(state, i.downloadAuto)
  if (!variant) return HIDDEN
  if (!prefs || prefs.version !== state.version) return { mode: 'card', variant, fresh: true, escalate: false }
  if (!prefs.escalated && i.nowMs - Date.parse(prefs.shownAt) > ESCALATE_AFTER_MS) {
    return { mode: 'card', variant, fresh: false, escalate: true }
  }
  if (prefs.dismissed) return HIDDEN
  return { mode: prefs.collapsed ? 'line' : 'card', variant, fresh: false, escalate: false }
}
