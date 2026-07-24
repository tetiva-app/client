import type { BodyType } from '@/types/request'

// Body content cache per request, survives component re-mounts.
// Keyed by requestId -> bodyType -> content string.
const drafts = new Map<string, Partial<Record<BodyType, string>>>()

const defaultContent: Record<BodyType, string> = {
  none: '',
  json: '',
  xml: '',
  raw: '',
  form: '[]',
  binary: '',
}

// Text-family types share the same string body shape, so switching among them
// preserves the user's text (Postman behavior); form/binary use per-type drafts.
const textFamily: ReadonlySet<BodyType> = new Set(['json', 'xml', 'raw'])

export function isTextFamily(bodyType: BodyType): boolean {
  return textFamily.has(bodyType)
}

export function isTextFamilyTransition(from: BodyType, to: BodyType): boolean {
  return textFamily.has(from) && textFamily.has(to)
}

export function saveDraft(requestId: string, bodyType: BodyType, content: string) {
  let entry = drafts.get(requestId)
  if (!entry) {
    entry = {}
    drafts.set(requestId, entry)
  }
  entry[bodyType] = content
}

export function getDraft(requestId: string, bodyType: BodyType): string {
  return drafts.get(requestId)?.[bodyType] ?? defaultContent[bodyType]
}

export function clearDrafts(requestId: string) {
  drafts.delete(requestId)
}
