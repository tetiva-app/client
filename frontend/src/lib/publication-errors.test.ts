import { describe, it, expect, afterEach } from 'vitest'
import type { ResultError } from '@/types/common'
import { PUBLICATION_ERROR_COPY, isQuotaRefusal, publicationErrorText } from './publication-errors'
import { setCurrentLocale } from './locale'

afterEach(() => { setCurrentLocale('en') })

function serverError(reason: string, desc = 'details'): ResultError {
  return {
    code: 'internal',
    message: `PublicationService.publish: rpc error: code = InvalidArgument desc = [${reason}] ${desc}`,
    reason,
  }
}

describe('publicationErrorText', () => {
  it.each([
    ['PUBLISH_QUOTA_EXCEEDED', 'Free plan includes 1 public collection', 'plans'],
    ['PUBLISH_FEATURE_REQUIRED', 'Unlisted links and passwords are available on Pro', 'plans'],
    ['PUBLISH_CONFIRM_REQUIRED', 'The page will become public and searchable', 'confirm-public'],
    ['PUBLISH_COLLECTION_NOT_SYNCED', "This collection hasn't synced to the cloud yet — wait for sync and try again", undefined],
    ['PUBLISH_COLLECTION_SYNCING', 'Syncing the collection — try again in a moment', undefined],
    ['PUBLISH_CONFLICT', 'Someone else changed this publication — reopen the dialog', undefined],
    ['PUBLISH_BLOCKED', 'Blocked by the platform', undefined],
    ['PUBLISH_SUSPENDED', 'Publishing is suspended for this account — contact support', undefined],
    ['PUBLISH_DISABLED', 'Publishing is turned off on this server', undefined],
    ['PUBLISH_EMAIL_UNVERIFIED', 'Confirm your email to publish', undefined],
    ['PASSWORD_INVALID', 'Password must be 8–72 bytes', undefined],
    ['RATE_LIMITED', 'Too many attempts — try again in a minute', undefined],
  ])('%s', (reason, text, action) => {
    expect(publicationErrorText(serverError(reason))).toEqual(action ? { text, action } : { text })
  })

  it('adds the server detail to a too-large snapshot', () => {
    expect(publicationErrorText(serverError('SNAPSHOT_TOO_LARGE', 'snapshot is 9.1 MiB'))).toEqual({
      text: 'The collection is too large to publish: snapshot is 9.1 MiB',
    })
  })

  it('quotes the server when it rejects the snapshot', () => {
    expect(publicationErrorText(serverError('SNAPSHOT_INVALID', 'items[3].http.method: not allowed'))).toEqual({
      text: 'The server rejected the collection: items[3].http.method: not allowed',
    })
  })

  it('keeps a message that is not a gRPC status as it is', () => {
    expect(publicationErrorText({ code: 'internal', message: 'bad snapshot', reason: 'SNAPSHOT_INVALID' }).text)
      .toBe('The server rejected the collection: bad snapshot')
  })

  it('asks for a new review when the collection changed after the preview', () => {
    expect(publicationErrorText({ code: 'conflict', message: 'publication preview conflict' })).toEqual({
      text: 'The collection changed — review again',
      action: 'review-again',
    })
  })

  it('reads network failures as an unreachable server', () => {
    const text = "Can't reach the server — try again"
    expect(publicationErrorText({
      code: 'internal',
      message: 'PublicationService.publish: publish rpc: rpc error: code = Unavailable desc = connection refused',
      reason: 'SERVER_UNREACHABLE',
    })).toEqual({ text })
    expect(publicationErrorText({ code: 'not_connected', message: 'publish: not connected to sync server' })).toEqual({ text })
    expect(publicationErrorText({ code: 'server_unreachable', message: 'cannot reach the sync server' })).toEqual({ text })
  })

  it('falls back to the formatted error', () => {
    expect(publicationErrorText({ code: 'validation', message: 'validation failed', fields: { password: 'must be 8-72 bytes' } }))
      .toEqual({ text: 'password: must be 8-72 bytes' })
    expect(publicationErrorText({ code: 'internal', message: 'boom', reason: 'SOMETHING_NEW' })).toEqual({ text: 'boom' })
  })
})

describe('publicationErrorText in Russian', () => {
  it('words every reason in both languages', () => {
    expect(Object.keys(PUBLICATION_ERROR_COPY.ru.reasons).sort()).toEqual(Object.keys(PUBLICATION_ERROR_COPY.en.reasons).sort())
    for (const locale of ['en', 'ru'] as const) {
      for (const [reason, text] of Object.entries(PUBLICATION_ERROR_COPY[locale].reasons)) {
        expect(text.trim().length, `${locale}.${reason}`).toBeGreaterThan(0)
      }
    }
    for (const key of ['tooLarge', 'rejected'] as const) {
      expect(PUBLICATION_ERROR_COPY.ru[key]).toContain('{detail}')
      expect(PUBLICATION_ERROR_COPY.en[key]).toContain('{detail}')
    }
  })

  it('keeps the action whatever the language', () => {
    expect(publicationErrorText(serverError('PUBLISH_QUOTA_EXCEEDED'), 'ru')).toEqual({
      text: 'В бесплатном тарифе\u00a0— одна публичная коллекция', action: 'plans',
    })
    expect(publicationErrorText({ code: 'conflict', message: 'publication preview conflict' }, 'ru')).toEqual({
      text: 'Коллекция изменилась\u00a0— проверьте ещё раз', action: 'review-again',
    })
  })

  it('wraps the server detail and says the server is unreachable in Russian', () => {
    expect(publicationErrorText(serverError('SNAPSHOT_TOO_LARGE', 'snapshot is 9.1 MiB'), 'ru').text)
      .toBe('Коллекция слишком большая для публикации: snapshot is 9.1 MiB')
    expect(publicationErrorText({ code: 'not_connected', message: 'x' }, 'ru').text).toBe('Сервер недоступен\u00a0— попробуйте снова')
  })

  it('takes the app language when none is given', () => {
    setCurrentLocale('ru')
    expect(publicationErrorText(serverError('RATE_LIMITED')).text).toBe('Слишком много попыток\u00a0— попробуйте через минуту')
  })
})

describe('isQuotaRefusal', () => {
  it('tells the plan quota apart from every other refusal', () => {
    expect(isQuotaRefusal({ code: 'internal', message: 'x', reason: 'PUBLISH_QUOTA_EXCEEDED' })).toBe(true)
    expect(isQuotaRefusal({ code: 'internal', message: 'x', reason: 'PUBLISH_FEATURE_REQUIRED' })).toBe(false)
    expect(isQuotaRefusal({ code: 'internal', message: 'PUBLISH_QUOTA_EXCEEDED' })).toBe(false)
  })
})
