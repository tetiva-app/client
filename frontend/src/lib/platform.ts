// The webview has no Wails platform API; only WebView2 exposes userAgentData.
type UANavigator = Navigator & { userAgentData?: { platform?: string } }

export function isLinux(): boolean {
  if (typeof navigator === 'undefined') return false
  const hinted = (navigator as UANavigator).userAgentData?.platform
  if (hinted) return hinted === 'Linux'
  const source = `${navigator.platform ?? ''} ${navigator.userAgent ?? ''}`
  return /linux|x11/i.test(source) && !/android/i.test(source)
}

export function isMac(): boolean {
  if (typeof navigator === 'undefined') return false
  const hinted = (navigator as UANavigator).userAgentData?.platform
  if (hinted) return hinted === 'macOS'
  return /Mac|iPhone|iPad/i.test(`${navigator.platform ?? ''} ${navigator.userAgent ?? ''}`)
}
