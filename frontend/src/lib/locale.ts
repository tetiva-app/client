import { readonly, ref, type Ref } from 'vue'

export type Locale = 'ru' | 'en'
export type LanguagePreference = 'system' | 'en' | 'ru'

const locale = ref<Locale>('en')

export const currentLocale: Readonly<Ref<Locale>> = readonly(locale)

export function setCurrentLocale(l: Locale): void {
  locale.value = l
}

export function systemLocale(navLang: string | undefined): Locale {
  return (navLang ?? '').toLowerCase().startsWith('ru') ? 'ru' : 'en'
}

export function resolveLocale(pref: LanguagePreference, navLang: string | undefined): Locale {
  return pref === 'system' ? systemLocale(navLang) : pref
}

export interface PluralForms {
  one: string
  few?: string
  many?: string
  other: string
}

export function plural(locale: Locale, n: number, forms: PluralForms): string {
  const category = new Intl.PluralRules(locale).select(n) as keyof PluralForms
  const form = forms[category] ?? forms.other
  return form.replace('{n}', formatNumber(locale, n))
}

export function fill(text: string, params: Record<string, string | number>): string {
  return text.replace(/\{(\w+)\}/g, (match, key: string) =>
    Object.prototype.hasOwnProperty.call(params, key) ? String(params[key]) : match)
}

export function formatNumber(locale: Locale, n: number): string {
  return new Intl.NumberFormat(locale).format(n)
}

const JUST_NOW: Record<Locale, string> = { en: 'just now', ru: 'только что' }

export function formatRelative(locale: Locale, iso: string, now: Date = new Date()): string {
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return ''
  const sec = Math.floor((now.getTime() - t) / 1000)
  if (sec < 60) return JUST_NOW[locale]
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'always' })
  if (sec < 3600) return rtf.format(-Math.floor(sec / 60), 'minute')
  if (sec < 86400) return rtf.format(-Math.floor(sec / 3600), 'hour')
  return rtf.format(-Math.floor(sec / 86400), 'day')
}

const BYTE_UNITS: Record<Locale, [string, string, string]> = {
  en: ['B', 'KB', 'MB'],
  ru: ['Б', 'КБ', 'МБ'],
}

export function formatBytes(locale: Locale, bytes: number): string {
  const [b, kb, mb] = BYTE_UNITS[locale]
  if (bytes < 1024) return `${formatNumber(locale, bytes)} ${b}`
  if (bytes < 1024 * 1024) return `${formatNumber(locale, Math.round(bytes / 1024))} ${kb}`
  const n = new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(bytes / (1024 * 1024))
  return `${n} ${mb}`
}
