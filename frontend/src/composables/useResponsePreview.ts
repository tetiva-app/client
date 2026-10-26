import { computed, onScopeDispose, ref, shallowRef, watch, type Ref } from 'vue'
import { getRequestService } from '@/services'
import { PREVIEW_IMAGE_MAX_BYTES, base64ToBytes, textPreviewKind, type TextPreviewKind } from '@/lib/response-preview'

export type ImageSource =
  | { kind: 'text'; text: string; mimeType: string }
  | { kind: 'file'; path: string; contentType: string }

export type ResponseImageState =
  | { status: 'none' }
  | { status: 'loading' }
  | { status: 'ready'; url: string; mimeType: string }
  | { status: 'tooLarge'; limit: number }

export type BodyView = 'source' | 'preview'

const bodyViews: Record<TextPreviewKind, Ref<BodyView>> = {
  html: ref('source'),
  svg: ref('preview'),
}

export function useTextPreview(source: () => { contentType: string; body: string } | null) {
  const kind = computed<TextPreviewKind | null>(() => {
    const src = source()
    return src ? textPreviewKind(src.contentType, src.body) : null
  })
  const view = computed<BodyView>({
    get: () => (kind.value ? bodyViews[kind.value].value : 'source'),
    set: (next) => {
      if (kind.value) bodyViews[kind.value].value = next
    },
  })
  const previewing = computed(() => kind.value !== null && view.value === 'preview')
  return { kind, view, previewing }
}

function sourceKey(src: ImageSource | null): string {
  if (!src) return ''
  return src.kind === 'file' ? `file:${src.path}:${src.contentType}` : `text:${src.mimeType}:${src.text}`
}

export function useResponseImage(source: () => ImageSource | null): Readonly<Ref<ResponseImageState>> {
  const state = shallowRef<ResponseImageState>({ status: 'none' })
  let objectUrl: string | null = null
  let run = 0

  function release() {
    if (objectUrl) URL.revokeObjectURL(objectUrl)
    objectUrl = null
  }

  function show(bytes: BlobPart, mimeType: string) {
    const blob = new Blob([bytes], { type: mimeType })
    if (blob.size > PREVIEW_IMAGE_MAX_BYTES) {
      state.value = { status: 'tooLarge', limit: PREVIEW_IMAGE_MAX_BYTES }
      return
    }
    objectUrl = URL.createObjectURL(blob)
    state.value = { status: 'ready', url: objectUrl, mimeType }
  }

  async function load(src: ImageSource | null) {
    const current = ++run
    release()
    if (!src) {
      state.value = { status: 'none' }
      return
    }
    if (src.kind === 'text') {
      show(src.text, src.mimeType)
      return
    }

    state.value = { status: 'loading' }
    const res = await getRequestService()
      .then(service => service.readResponseImage(src.path, src.contentType))
      .catch(() => null)
    if (current !== run) return
    if (res?.error?.reason === 'PREVIEW_TOO_LARGE') {
      state.value = { status: 'tooLarge', limit: Number(res.error.fields?.limit) }
    } else if (!res || res.error) {
      state.value = { status: 'none' }
    } else {
      show(base64ToBytes(res.data.data), res.data.mimeType)
    }
  }

  watch(() => sourceKey(source()), () => load(source()), { immediate: true })

  onScopeDispose(() => {
    run++
    release()
  })

  return state
}
