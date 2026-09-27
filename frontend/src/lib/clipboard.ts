// Wails first: navigator.clipboard fails in WebView once an await spends the user gesture.
export async function copyText(text: string): Promise<void> {
  try {
    const { Clipboard } = await import('@wailsio/runtime')
    await Clipboard.SetText(text)
  } catch {
    await navigator.clipboard.writeText(text)
  }
}
