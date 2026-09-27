import { computed, type ComputedRef, type Ref } from 'vue'
import { currentLocale, type Locale } from '@/lib/locale'

export function useLocale(): Readonly<Ref<Locale>> {
  return currentLocale
}

export function useCopy<T>(dict: Record<Locale, T>): ComputedRef<T> {
  return computed(() => dict[currentLocale.value])
}
