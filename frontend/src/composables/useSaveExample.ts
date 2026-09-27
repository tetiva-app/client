import { computed, ref } from 'vue'
import type { ExecuteResponse } from '@/types/execute'
import type { CreateExampleInput, ExampleProtocol } from '@/types/example'
import type { HeaderItem } from '@/types/request'
import { useExamplesStore } from '@/stores/examples'
import { useToast } from '@/composables/useToast'
import { getExampleService } from '@/services'
import { grpcStatusName } from '@/constants/grpc-status'
import { exampleBodyTooLarge } from '@/lib/example-limits'

export interface SaveExampleSource {
  requestId: string
  protocol: ExampleProtocol
  isDraft: boolean
  response: ExecuteResponse
}

// Go puts the whole status line ("404 Not Found") into statusText for HTTP.
function reasonPhrase(statusText: string): string {
  return statusText.replace(/^\d+\s*/, '')
}

export function saveExampleBlocker(src: SaveExampleSource): string | null {
  if (src.response.isBinary) return "Binary responses can't be saved as examples"
  if (exampleBodyTooLarge(src.response.body ?? '')) return 'Response is too large for an example (max 256 KB)'
  if (src.isDraft) return 'Save the request first to keep examples'
  return null
}

export function defaultExampleName(protocol: ExampleProtocol, statusCode: number, statusText: string): string {
  if (protocol === 'grpc') return `${grpcStatusName(statusCode)} (${statusCode})`
  const phrase = reasonPhrase(statusText)
  return phrase ? `${statusCode} ${phrase}` : String(statusCode)
}

export function findContentType(headers: HeaderItem[]): string {
  return headers.find(h => h.key.toLowerCase() === 'content-type')?.value ?? ''
}

export function exampleFromResponse(src: SaveExampleSource, name: string): CreateExampleInput {
  const { response, protocol } = src
  const headers = Object.entries(response.headers ?? {}).flatMap(([key, values]) =>
    values.map(value => ({ key, value, enabled: true })),
  )
  return {
    requestId: src.requestId,
    protocol,
    name,
    statusCode: response.statusCode,
    statusText: protocol === 'grpc' ? grpcStatusName(response.statusCode) : reasonPhrase(response.statusText),
    headers,
    body: response.body ?? '',
    contentType: findContentType(headers),
  }
}

// A failed scan never blocks the save; it only decides whether to ask first.
async function suspectedSecrets(input: CreateExampleInput): Promise<string[]> {
  try {
    const res = await (await getExampleService()).scanSecrets({ headers: input.headers, body: input.body })
    return res.error ? [] : res.data
  } catch {
    return []
  }
}

export function useSaveExample(source: () => SaveExampleSource | null) {
  const store = useExamplesStore()
  const toast = useToast()
  const saving = ref(false)
  const secretsOpen = ref(false)
  const secretLabels = ref<string[]>([])
  let held: CreateExampleInput | null = null

  const blocker = computed(() => {
    const src = source()
    return src ? saveExampleBlocker(src) : null
  })

  const defaultName = computed(() => {
    const src = source()
    return src ? defaultExampleName(src.protocol, src.response.statusCode, src.response.statusText) : ''
  })

  async function create(input: CreateExampleInput): Promise<boolean> {
    try {
      await store.create(input)
      toast.success('Saved as example')
      return true
    } catch {
      return false
    }
  }

  // False on a suspected credential: secretsOpen asks, and saveAnyway stores what was held.
  async function save(name: string): Promise<boolean> {
    const src = source()
    if (!src || saving.value || saveExampleBlocker(src)) return false
    const finalName = name.trim() || defaultExampleName(src.protocol, src.response.statusCode, src.response.statusText)
    const input = exampleFromResponse(src, finalName)
    saving.value = true
    try {
      const labels = await suspectedSecrets(input)
      if (labels.length > 0) {
        held = input
        secretLabels.value = labels
        secretsOpen.value = true
        return false
      }
      return await create(input)
    } finally {
      saving.value = false
    }
  }

  async function saveAnyway(): Promise<boolean> {
    const input = held
    secretsOpen.value = false
    if (!input || saving.value) return false
    held = null
    saving.value = true
    try {
      return await create(input)
    } finally {
      saving.value = false
    }
  }

  return { blocker, defaultName, saving, save, secretsOpen, secretLabels, saveAnyway }
}
