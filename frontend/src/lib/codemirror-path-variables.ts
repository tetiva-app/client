import { Decoration, type DecorationSet, EditorView, ViewPlugin, type ViewUpdate } from '@codemirror/view'
import { RangeSetBuilder } from '@codemirror/state'
import { pathVariableSpans } from '@/lib/path-variables'

const pathVariableMark = Decoration.mark({ class: 'cm-path-var' })

function decorations(view: EditorView): DecorationSet {
  const builder = new RangeSetBuilder<Decoration>()
  for (const [from, to] of pathVariableSpans(view.state.doc.toString())) {
    builder.add(from, to, pathVariableMark)
  }
  return builder.finish()
}

export const pathVariableHighlight = [
  ViewPlugin.fromClass(
    class {
      decorations: DecorationSet

      constructor(view: EditorView) {
        this.decorations = decorations(view)
      }

      update(update: ViewUpdate) {
        if (update.docChanged) this.decorations = decorations(update.view)
      }
    },
    { decorations: (v) => v.decorations },
  ),
  EditorView.theme({
    '.cm-path-var': { color: 'var(--primary)', fontWeight: '500' },
  }),
]
