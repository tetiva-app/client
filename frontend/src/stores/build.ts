import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { getBuildService, type BuildFlavor } from '@/services'

const LOAD_TIMEOUT_MS = 1500

// main.ts awaits load() so locked controls never flash as available; Go refuses either way.
export const useBuildStore = defineStore('build', () => {
  const flavor = ref<BuildFlavor>('stable')
  const preview = computed(() => flavor.value === 'preview')
  const productName = computed(() => (preview.value ? 'Tetiva Preview' : 'Tetiva'))

  async function read() {
    try {
      const res = await (await getBuildService()).info()
      if (!res.error) flavor.value = res.data.flavor
    } catch (err) {
      console.error('Failed to read the build flavor:', err)
    }
  }

  async function load() {
    let timer: ReturnType<typeof setTimeout> | undefined
    const gaveUp = new Promise<void>(resolve => { timer = setTimeout(resolve, LOAD_TIMEOUT_MS) })
    await Promise.race([read(), gaveUp])
    clearTimeout(timer)
  }

  return { flavor, preview, productName, load }
})
