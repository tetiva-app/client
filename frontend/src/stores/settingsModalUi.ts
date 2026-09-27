import { ref } from 'vue'
import { defineStore } from 'pinia'
import type { SettingsSectionId } from '@/lib/settings-search'

export const useSettingsModalUi = defineStore('settingsModalUi', () => {
  const open = ref(false)
  const section = ref<SettingsSectionId>('interface')

  function show(target?: SettingsSectionId) {
    if (target) section.value = target
    open.value = true
  }
  function hide() { open.value = false }

  return { open, section, show, hide }
})
