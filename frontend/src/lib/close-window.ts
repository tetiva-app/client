import { isWailsEnvironment } from '@/services'

// WebKitGTK ignores window.close() for a window the script did not open, so on
// Linux only the Wails runtime can dismiss a child window.
export async function closeCurrentWindow(): Promise<void> {
  if (isWailsEnvironment()) {
    const { Window } = await import('@wailsio/runtime')
    await Window.Close()
    return
  }
  window.close()
}
