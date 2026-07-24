import { isWailsEnvironment } from '@/services'

// Opens a URL in the user's default system browser. In Wails we must use the
// runtime Browser API — window.open / target=_blank are unreliable in WKWebView.
export async function openExternal(url: string): Promise<void> {
  if (isWailsEnvironment()) {
    const { Browser } = await import('@wailsio/runtime')
    await Browser.OpenURL(url)
    return
  }
  window.open(url, '_blank', 'noopener')
}
