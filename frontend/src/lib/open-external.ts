import { isWailsEnvironment } from '@/services'

// Opens a URL in the user's default system browser. In Wails we must use the
// runtime Browser API — window.open / target=_blank are unreliable in WKWebView.
export async function openExternal(url: string): Promise<void> {
  // Checked before the Wails branch: the e2e specs load the app with ?wails=1,
  // which makes isWailsEnvironment() true and would launch a real browser.
  if (window.__TETIVA_NO_EXTERNAL_OPEN__) {
    window.__lastExternalUrl = url
    return
  }
  let scheme = ''
  try {
    scheme = new URL(url).protocol
  } catch {
    throw new Error('cannot open a malformed URL')
  }
  if (scheme !== 'http:' && scheme !== 'https:') {
    throw new Error(`refusing to open a ${scheme} URL`)
  }
  if (isWailsEnvironment()) {
    const { Browser } = await import('@wailsio/runtime')
    await Browser.OpenURL(url)
    return
  }
  window.open(url, '_blank', 'noopener')
}
