import { ref, watch, type Ref } from 'vue'
import { cabinetPublishedUrl } from '@/lib/cabinet'

export function useCabinetLink(visible: () => boolean, offline: () => boolean): Ref<string | null> {
  const url = ref<string | null>(null)
  let asked = 0
  async function discover(shown: boolean) {
    const ask = ++asked
    if (!shown) return
    const found = await cabinetPublishedUrl().catch(() => null)
    if (ask === asked) url.value = found
  }
  watch(visible, shown => {
    url.value = null
    void discover(shown)
  }, { immediate: true })
  watch(offline, (now, was) => {
    if (was && !now && visible() && url.value === null) void discover(true)
  })
  return url
}
