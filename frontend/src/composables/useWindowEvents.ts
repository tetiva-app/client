import { onMounted, onUnmounted } from 'vue'
import { isWailsEnvironment } from '@/services'
import { onContentSaved } from '@/lib/content-saved'

interface WindowEventsConfig {
  mode: 'main' | 'detached-request' | 'schema-viewer'
  requestId?: string
  onEnvChanged?: () => void
  onCollectionUpdated?: () => void
  onWorkspaceSwitched?: () => void
  onRequestDeleted?: () => void
  onRequestUpdated?: () => void
  onSaveAndClose?: () => void
  onSyncChanged?: () => void
  onExamplesChanged?: (requestId: string) => void
  onSavedInOtherWindow?: () => void
}

// The Wails runtime may wrap the emitted map in `data` depending on version.
export function eventPayload(evt: unknown): Record<string, unknown> {
  const wrapped = (evt as { data?: unknown })?.data
  return ((wrapped ?? evt) as Record<string, unknown>) ?? {}
}

export function useWindowEvents(config: WindowEventsConfig) {
  if (!isWailsEnvironment()) return

  const unsubscribers: (() => void)[] = []
  let disposed = false

  onMounted(async () => {
    const { Events } = await import('@wailsio/runtime')
    if (disposed) return // component unmounted before the dynamic import resolved

    if (config.onEnvChanged) {
      unsubscribers.push(Events.On('env:changed', config.onEnvChanged))
    }

    if (config.onCollectionUpdated) {
      unsubscribers.push(Events.On('collection:updated', config.onCollectionUpdated))
    }

    if (config.onWorkspaceSwitched) {
      unsubscribers.push(Events.On('workspace:switched', config.onWorkspaceSwitched))
    }

    if (config.onSyncChanged) {
      unsubscribers.push(Events.On('sync:changed', config.onSyncChanged))
      unsubscribers.push(Events.On('sync:entity_updated', config.onSyncChanged))
    }

    const onExamplesChanged = config.onExamplesChanged
    if (onExamplesChanged) {
      unsubscribers.push(Events.On('examples:changed', (evt: unknown) => {
        onExamplesChanged(String(eventPayload(evt).requestId ?? ''))
      }))
    }

    if (config.onSavedInOtherWindow) {
      unsubscribers.push(Events.On('content:saved', config.onSavedInOtherWindow))
    }

    if (config.mode === 'detached-request' && config.requestId) {
      unsubscribers.push(onContentSaved(() => { void Events.Emit('content:saved') }))
      if (config.onRequestDeleted) {
        unsubscribers.push(
          Events.On(`request:deleted:${config.requestId}`, config.onRequestDeleted),
        )
      }
      if (config.onRequestUpdated) {
        unsubscribers.push(
          Events.On(`request:updated:${config.requestId}`, config.onRequestUpdated),
        )
      }
      if (config.onSaveAndClose) {
        unsubscribers.push(
          Events.On(`window:save-and-close:${config.requestId}`, config.onSaveAndClose),
        )
      }
    }
  })

  onUnmounted(() => {
    disposed = true
    for (const unsub of unsubscribers) {
      unsub()
    }
  })
}

/**
 * Emit a Wails event (no-op in browser mode).
 */
export async function emitWailsEvent(name: string, data?: any) {
  if (!isWailsEnvironment()) return
  const { Events } = await import('@wailsio/runtime')
  await Events.Emit(name, data)
}
