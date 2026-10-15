import type { Ref } from 'vue'
import { isModShortcut } from '@/lib/shortcut-guards'

export function useFindShortcut(input: Ref<HTMLInputElement | null>) {
  return (event: KeyboardEvent) => {
    if (!input.value || !isModShortcut(event, 'KeyF', 'f')) return
    event.preventDefault()
    input.value.focus()
    input.value.select()
  }
}
