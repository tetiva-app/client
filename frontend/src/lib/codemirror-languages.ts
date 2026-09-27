import type { Extension } from '@codemirror/state'
import { StreamLanguage } from '@codemirror/language'
import { json } from '@codemirror/lang-json'
import { xml } from '@codemirror/lang-xml'
import { html } from '@codemirror/lang-html'
import { javascript } from '@codemirror/lang-javascript'

export type ViewerLanguage =
  | 'json' | 'xml' | 'html' | 'text' | 'shell' | 'python' | 'go' | 'java' | 'csharp' | 'php' | 'javascript'

// Snippet languages load on demand so the response viewer's chunk does not carry them.
export function languageSupport(lang?: ViewerLanguage): Extension | Promise<Extension> {
  switch (lang) {
    case 'json': return json()
    case 'xml': return xml()
    case 'html': return html()
    case 'javascript': return javascript()
    case 'shell': return import('@codemirror/legacy-modes/mode/shell').then((m) => StreamLanguage.define(m.shell))
    case 'python': return import('@codemirror/legacy-modes/mode/python').then((m) => StreamLanguage.define(m.python))
    case 'go': return import('@codemirror/legacy-modes/mode/go').then((m) => StreamLanguage.define(m.go))
    case 'java': return import('@codemirror/legacy-modes/mode/clike').then((m) => StreamLanguage.define(m.java))
    case 'csharp': return import('@codemirror/legacy-modes/mode/clike').then((m) => StreamLanguage.define(m.csharp))
    default: return []
  }
}
