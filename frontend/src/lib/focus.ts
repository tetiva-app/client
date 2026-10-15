export function focusIsLost(): boolean {
  const el = document.activeElement
  return !el || el === document.body || el.getClientRects().length === 0
}

export function refocus(target: () => HTMLElement | null | undefined) {
  requestAnimationFrame(() => {
    if (focusIsLost()) target()?.focus()
  })
}
