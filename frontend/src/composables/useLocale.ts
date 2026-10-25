import { computed, inject, provide, readonly, ref, type ComputedRef, type InjectionKey, type Ref } from 'vue'
import { currentLocale, type Locale } from '@/lib/locale'

export function useLocale(): Readonly<Ref<Locale>> {
  return currentLocale
}

export function useCopy<T>(dict: Record<Locale, T>): ComputedRef<T> {
  return computed(() => dict[currentLocale.value])
}

const SCREEN_LOCALE: InjectionKey<Readonly<Ref<Locale>>> = Symbol('screen-locale')
const ENGLISH: Readonly<Ref<Locale>> = readonly(ref<Locale>('en'))

// Components shared with screens not yet translated stay English unless their screen opts in.
export function provideScreenLocale(): void {
  provide(SCREEN_LOCALE, currentLocale)
}

export function useScreenLocale(): Readonly<Ref<Locale>> {
  return inject(SCREEN_LOCALE, ENGLISH)
}

export function useScreenCopy<T>(dict: Record<Locale, T>): ComputedRef<T> {
  const locale = useScreenLocale()
  return computed(() => dict[locale.value])
}
