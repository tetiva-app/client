import { computed, effectScope, onScopeDispose, ref, shallowRef, watch, type Ref } from 'vue'
import { getRequestService } from '@/services'
import { loadSnippets, type Snippets } from '@/lib/snippets/runtime'
import { familyOf, snippetRequest } from '@/lib/snippets/snippet-request'
import type { SnippetTarget } from '@/lib/snippets/types'
import { useSettingsStore } from '@/stores/settings'
import type { Request } from '@/types/request'
import type { SnippetInput } from '@/types/snippet'

const DEBOUNCE_MS = 300

export interface CodeSnippet {
  targets: Ref<SnippetTarget[]>
  selectedKey: Ref<string>
  code: Ref<string>
  warnings: Ref<string[]>
  error: Ref<string | null>
  loading: Ref<boolean>
  stale: Ref<boolean>
  canCopy: Ref<boolean>
  resolveVariables: Ref<boolean>
  includeSecrets: Ref<boolean>
  select(key: string): void
  dispose(): void
}

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

export function useCodeSnippet(opts: {
  request: Ref<Request | undefined>
  workspaceId: Ref<string | undefined>
  envVersion: Ref<unknown>
  executing?: Ref<boolean>
}): CodeSnippet {
  const settings = useSettingsStore()
  const scope = effectScope()

  const snippets = shallowRef<Snippets | null>(null)
  // shallowRef: generate() structuredClones its input, and a reactive proxy cannot be cloned.
  const input = shallowRef<SnippetInput | null>(null)
  const code = ref('')
  const warnings = ref<string[]>([])
  const buildError = ref<string | null>(null)
  const bundleError = ref<string | null>(null)
  const building = ref(false)
  const stale = ref(false)
  const resolveVariables = ref(true)
  const includeSecrets = ref(false)

  let generation = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let disposed = false

  async function refresh() {
    clearTimeout(timer)
    timer = undefined
    const gen = ++generation
    const req = opts.request.value
    const workspaceId = opts.workspaceId.value
    if (!req || !workspaceId) {
      building.value = false
      buildError.value = null
      input.value = null
      stale.value = false
      return
    }
    building.value = true
    let next: SnippetInput | null = null
    let failure: string | null = null
    try {
      const service = await getRequestService()
      const result = await service.buildSnippetInput({
        workspaceId,
        resolveVariables: resolveVariables.value,
        includeSecrets: resolveVariables.value && includeSecrets.value,
        request: snippetRequest(req),
      })
      if (result.error) failure = result.error.message
      else next = result.data
    } catch (e) {
      failure = message(e)
    }
    if (disposed || gen !== generation) return
    building.value = false
    buildError.value = failure
    input.value = next
    stale.value = false
  }

  function invalidate(clear: boolean) {
    generation++
    clearTimeout(timer)
    timer = undefined
    stale.value = true
    if (clear) input.value = null
  }

  function rebuild(clear = false) {
    invalidate(clear)
    void refresh()
  }

  function schedule() {
    invalidate(false)
    timer = setTimeout(() => void refresh(), DEBOUNCE_MS)
  }

  const family = computed(() => familyOf(opts.request.value?.protocol ?? 'http'))

  const targets = computed(() => {
    const protocol = opts.request.value?.protocol
    return snippets.value && protocol ? snippets.value.targetsFor(protocol) : []
  })

  const selectedKey = computed(() => {
    const saved = settings.snippetTargets[family.value]
    return targets.value.some((t) => t.key === saved) ? saved : targets.value[0]?.key ?? ''
  })

  function render() {
    const current = input.value
    const lib = snippets.value
    if (!current || !lib || !selectedKey.value) {
      code.value = ''
      warnings.value = current?.warnings ?? []
      return
    }
    const out = lib.generate(current, selectedKey.value)
    code.value = out.code
    warnings.value = [...new Set([...current.warnings, ...out.warnings])]
  }

  const loading = computed(() => building.value || (!snippets.value && !bundleError.value))

  scope.run(() => {
    // Compared as text: the tabs store swaps in a new object on any change, printed field or not.
    watch(() => (opts.request.value ? JSON.stringify(snippetRequest(opts.request.value)) : ''), schedule)
    watch([opts.envVersion, opts.workspaceId], () => rebuild())
    watch(resolveVariables, (on) => { if (!on) includeSecrets.value = false }, { flush: 'sync' })
    watch([resolveVariables, includeSecrets], () => rebuild(true))
    watch(() => opts.executing?.value ?? false, (now, before) => { if (before && !now) rebuild() })
    watch([input, snippets, selectedKey], render)
    onScopeDispose(() => {
      disposed = true
      clearTimeout(timer)
    })
  })

  void refresh()
  loadSnippets().then(
    (lib) => { if (!disposed) snippets.value = lib },
    (e) => { if (!disposed) bundleError.value = message(e) },
  )

  return {
    targets,
    selectedKey,
    code,
    warnings,
    error: computed(() => buildError.value ?? bundleError.value),
    loading,
    stale,
    canCopy: computed(() => code.value !== '' && !loading.value && !stale.value),
    resolveVariables,
    includeSecrets,
    select(key: string) {
      settings.setSnippetTarget(family.value, key)
    },
    dispose() {
      scope.stop()
    },
  }
}
