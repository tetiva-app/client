// The webview has no Wails platform API, so the OS is read off the navigator:
// userAgentData exists in WebView2, WebKitGTK and WKWebView only expose platform/userAgent.
type UANavigator = Navigator & { userAgentData?: { platform?: string } }

export function isLinux(): boolean {
  if (typeof navigator === 'undefined') return false
  const hinted = (navigator as UANavigator).userAgentData?.platform
  if (hinted) return hinted === 'Linux'
  const source = `${navigator.platform ?? ''} ${navigator.userAgent ?? ''}`
  return /linux|x11/i.test(source) && !/android/i.test(source)
}
