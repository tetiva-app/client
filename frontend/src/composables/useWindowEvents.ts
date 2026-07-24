import { onMounted, onUnmounted } from 'vue'
import { isWailsEnvironment } from '@/services'

interface WindowEventsConfig {
  mode: 'main' | 'detached-request' | 'schema-viewer'
  requestId?: string
  onEnvChanged?: () => void
  onCollectionUpdated?: () => void
  onWorkspaceSwitched?: () => void
  onRequestDeleted?: () => void
  onRequestUpdated?: () => void
  onSyncChanged?: () => void
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

    if (config.mode === 'detached-request' && config.requestId) {
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
