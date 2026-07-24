// Guards for window-level shortcut handlers.

// Text-editing surface: CodeMirror, native inputs, contenteditable.
export function isEditingTarget(event: KeyboardEvent): boolean {
  const el = event.target as HTMLElement | null
  if (!el || typeof el.closest !== 'function') return false
  if (el.closest('.cm-editor')) return true
  const tag = el.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  return el.isContentEditable === true
}

// Open modal layer (Reka dialogs render with role="dialog"/"alertdialog").
export function isInsideOverlay(event: KeyboardEvent): boolean {
  const el = event.target as HTMLElement | null
  if (!el || typeof el.closest !== 'function') return false
  return el.closest('[role="dialog"], [role="alertdialog"]') !== null
}
