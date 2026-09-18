import { isWailsEnvironment } from '@/services'

// Only the Wails runtime can dismiss a child window: WebKitGTK ignores window.close().
export async function closeCurrentWindow(): Promise<void> {
  if (isWailsEnvironment()) {
    const { Window } = await import('@wailsio/runtime')
    await Window.Close()
    return
  }
  window.close()
}
