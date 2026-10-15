import { watch } from 'vue'
import { getWindowService } from '@/services'
import type { ThemePreference } from '@/lib/settings-storage'

interface ThemeSource {
  theme: ThemePreference
  effectiveTheme: 'light' | 'dark'
}

export function reportWindowTheme(settings: ThemeSource) {
  watch(
    () => [settings.theme, settings.effectiveTheme] as const,
    async ([theme, effective]) => {
      const windows = await getWindowService()
      await windows?.setTheme(theme, effective)
    },
    { immediate: true },
  )
}
