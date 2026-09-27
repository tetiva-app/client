import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import type { DeepLink, ImportPreview } from '@/services'

// 'opening' is a deep link on its way to the confirmation: there is no link to type.
export type LinkStep = 'url' | 'password' | 'opening'

// oneTimeToken: the deep link's import token, spent by the first download.
export type ImportSource =
  | { kind: 'link'; slug: string; token: string; previewId: string; oneTimeToken: boolean }
  | { kind: 'file'; content: string; parentId: string | null }

// The dialogs live in App.vue; the sidebar menus and deep links start the flow in useImportFlow.
export const useImportUi = defineStore('importUi', () => {
  const flow = ref(0)

  const linkOpen = ref(false)
  const linkStep = ref<LinkStep>('url')
  const slug = ref('')
  const token = ref('')
  const title = ref('')
  const busy = ref('')
  const linkError = ref('')

  const source = ref<ImportSource | null>(null)
  const preview = ref<ImportPreview | null>(null)
  const includeScripts = ref(false)
  const importing = ref(false)
  const confirmError = ref('')
  const notice = ref('')
  const expired = ref(false)

  const queue = ref<DeepLink[]>([])

  const active = computed(() => linkOpen.value || source.value !== null || busy.value !== '')

  function reset() {
    flow.value++
    linkOpen.value = false
    linkStep.value = 'url'
    slug.value = ''
    token.value = ''
    title.value = ''
    busy.value = ''
    linkError.value = ''
    source.value = null
    preview.value = null
    includeScripts.value = false
    importing.value = false
    confirmError.value = ''
    notice.value = ''
    expired.value = false
    return flow.value
  }

  return {
    flow,
    linkOpen,
    linkStep,
    slug,
    token,
    title,
    busy,
    linkError,
    source,
    preview,
    includeScripts,
    importing,
    confirmError,
    notice,
    expired,
    queue,
    active,
    reset,
  }
})
