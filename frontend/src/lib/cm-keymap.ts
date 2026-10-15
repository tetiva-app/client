import { defaultKeymap } from '@codemirror/commands'
import type { KeyBinding } from '@codemirror/view'

// Mod-Enter sends the request; defaultKeymap would insert a blank line on the way out.
export const editorKeymap: readonly KeyBinding[] = defaultKeymap.filter(b => b.key !== 'Mod-Enter')
