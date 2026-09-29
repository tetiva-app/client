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

export type ClientOS = 'darwin' | 'windows' | 'linux'

// Names match runtime.GOOS: the update server accepts no others.
export function clientOS(): ClientOS | null {
  if (isMac()) return 'darwin'
  if (isLinux()) return 'linux'
  if (typeof navigator === 'undefined') return null
  const hinted = (navigator as UANavigator).userAgentData?.platform
  if (hinted) return hinted === 'Windows' ? 'windows' : null
  return /Win/.test(`${navigator.platform ?? ''} ${navigator.userAgent ?? ''}`) ? 'windows' : null
}
