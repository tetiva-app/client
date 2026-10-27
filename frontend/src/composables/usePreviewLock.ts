import { computed } from 'vue'
import { useBuildStore } from '@/stores/build'
import { useCopy } from '@/composables/useLocale'
import { TREE_COPY } from '@/components/sidebar/copy'

export function usePreviewLock() {
  const build = useBuildStore()
  const copy = useCopy(TREE_COPY)
  return {
    locked: computed(() => build.preview),
    reason: computed(() => copy.value.preview.signIn),
  }
}
