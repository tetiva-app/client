import { EditorView } from '@codemirror/view'

// CodeMirror's base theme pins the scroller to `monospace`, which WebKit draws as Courier.
export const inheritFont = EditorView.theme({ '.cm-scroller': { fontFamily: 'inherit' } })
