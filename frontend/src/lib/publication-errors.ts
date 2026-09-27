import type { ResultError } from '@/types/common'
import { formatResultError, isUnreachable } from '@/lib/result-error'
import { currentLocale, fill, type Locale } from '@/lib/locale'

export type PublicationErrorAction = 'plans' | 'review-again' | 'confirm-public'

export interface PublicationErrorText {
  text: string
  action?: PublicationErrorAction
}

type Reason =
  | 'PUBLISH_QUOTA_EXCEEDED'
  | 'PUBLISH_FEATURE_REQUIRED'
  | 'PUBLISH_CONFIRM_REQUIRED'
  | 'PUBLISH_COLLECTION_NOT_SYNCED'
  | 'PUBLISH_COLLECTION_SYNCING'
  | 'PUBLISH_CONFLICT'
  | 'PUBLISH_BLOCKED'
  | 'PUBLISH_SUSPENDED'
  | 'PUBLISH_DISABLED'
  | 'PUBLISH_EMAIL_UNVERIFIED'
  | 'PASSWORD_INVALID'
  | 'RATE_LIMITED'

interface PublicationErrorCopy {
  reasons: Record<Reason, string>
  tooLarge: string
  rejected: string
  changed: string
  unreachable: string
}

export const PUBLICATION_ERROR_COPY: Record<Locale, PublicationErrorCopy> = {
  en: {
    reasons: {
      PUBLISH_QUOTA_EXCEEDED: 'Free plan includes 1 public collection',
      PUBLISH_FEATURE_REQUIRED: 'Unlisted links and passwords are available on Pro',
      PUBLISH_CONFIRM_REQUIRED: 'The page will become public and searchable',
      PUBLISH_COLLECTION_NOT_SYNCED: "This collection hasn't synced to the cloud yet — wait for sync and try again",
      PUBLISH_COLLECTION_SYNCING: 'Syncing the collection — try again in a moment',
      PUBLISH_CONFLICT: 'Someone else changed this publication — reopen the dialog',
      PUBLISH_BLOCKED: 'Blocked by the platform',
      PUBLISH_SUSPENDED: 'Publishing is suspended for this account — contact support',
      PUBLISH_DISABLED: 'Publishing is turned off on this server',
      PUBLISH_EMAIL_UNVERIFIED: 'Confirm your email to publish',
      PASSWORD_INVALID: 'Password must be 8–72 bytes',
      RATE_LIMITED: 'Too many attempts — try again in a minute',
    },
    tooLarge: 'The collection is too large to publish: {detail}',
    rejected: 'The server rejected the collection: {detail}',
    changed: 'The collection changed — review again',
    unreachable: "Can't reach the server — try again",
  },
  ru: {
    reasons: {
      PUBLISH_QUOTA_EXCEEDED: 'В бесплатном тарифе\u00a0— одна публичная коллекция',
      PUBLISH_FEATURE_REQUIRED: 'Доступ по ссылке и пароль есть в тарифе Pro',
      PUBLISH_CONFIRM_REQUIRED: 'Страница станет публичной и попадёт в поиск',
      PUBLISH_COLLECTION_NOT_SYNCED: 'Коллекция ещё не синхронизирована с облаком\u00a0— дождитесь синхронизации и попробуйте снова',
      PUBLISH_COLLECTION_SYNCING: 'Коллекция синхронизируется\u00a0— попробуйте чуть позже',
      PUBLISH_CONFLICT: 'Публикацию изменил кто-то другой\u00a0— откройте окно заново',
      PUBLISH_BLOCKED: 'Заблокировано платформой',
      PUBLISH_SUSPENDED: 'Публикация для этого аккаунта приостановлена\u00a0— напишите в поддержку',
      PUBLISH_DISABLED: 'На этом сервере публикация выключена',
      PUBLISH_EMAIL_UNVERIFIED: 'Подтвердите почту, чтобы публиковать',
      PASSWORD_INVALID: 'Пароль должен быть от 8 до 72 байт',
      RATE_LIMITED: 'Слишком много попыток\u00a0— попробуйте через минуту',
    },
    tooLarge: 'Коллекция слишком большая для публикации: {detail}',
    rejected: 'Сервер не принял коллекцию: {detail}',
    changed: 'Коллекция изменилась\u00a0— проверьте ещё раз',
    unreachable: 'Сервер недоступен\u00a0— попробуйте снова',
  },
}

const REASON_ACTION: Partial<Record<Reason, PublicationErrorAction>> = {
  PUBLISH_QUOTA_EXCEEDED: 'plans',
  PUBLISH_FEATURE_REQUIRED: 'plans',
  PUBLISH_CONFIRM_REQUIRED: 'confirm-public',
}

function isReason(reason: string | undefined): reason is Reason {
  return !!reason && Object.prototype.hasOwnProperty.call(PUBLICATION_ERROR_COPY.en.reasons, reason)
}

// The Go side wraps the gRPC status ("…: rpc error: code = X desc = [REASON] text"); only "text" is for people.
function serverDetail(message: string): string {
  const desc = message.lastIndexOf('desc = ')
  const tail = desc >= 0 ? message.slice(desc + 'desc = '.length) : message
  return tail.replace(/^\[[A-Z_]+\]\s*/, '')
}

export function isQuotaRefusal(e: ResultError): boolean {
  return e.reason === 'PUBLISH_QUOTA_EXCEEDED'
}

export function publicationErrorText(e: ResultError, locale: Locale = currentLocale.value): PublicationErrorText {
  const copy = PUBLICATION_ERROR_COPY[locale]
  if (isUnreachable(e)) return { text: copy.unreachable }
  if (e.reason === 'SNAPSHOT_TOO_LARGE') return { text: fill(copy.tooLarge, { detail: serverDetail(e.message) }) }
  if (e.reason === 'SNAPSHOT_INVALID') return { text: fill(copy.rejected, { detail: serverDetail(e.message) }) }
  if (isReason(e.reason)) {
    const action = REASON_ACTION[e.reason]
    return action ? { text: copy.reasons[e.reason], action } : { text: copy.reasons[e.reason] }
  }
  if (e.code === 'conflict' && !e.reason) return { text: copy.changed, action: 'review-again' }
  return { text: formatResultError(e) }
}
