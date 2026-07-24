import { Decoration, type DecorationSet, EditorView, ViewPlugin, type ViewUpdate, hoverTooltip, type Tooltip } from '@codemirror/view'
import { RangeSetBuilder, StateEffect } from '@codemirror/state'
import { type CompletionContext, type CompletionResult } from '@codemirror/autocomplete'
import { useEnvModalUi } from '@/stores/envModalUi'

export const varsChangedEffect = StateEffect.define<void>()

const VAR_PATTERN = /\{\{([^}]+)\}\}/g

export function extractVarName(raw: string): string {
  return raw.replace(/^\{\{/, '').replace(/\}\}$/, '').trim()
}

const resolvedMark = Decoration.mark({ class: 'cm-var-resolved' })
const unresolvedMark = Decoration.mark({ class: 'cm-var-unresolved' })

function buildDecorations(view: EditorView, vars: Record<string, string>): DecorationSet {
  const builder = new RangeSetBuilder<Decoration>()
  const doc = view.state.doc.toString()

  for (const match of doc.matchAll(VAR_PATTERN)) {
    const from = match.index!
    const to = from + match[0].length
    const varName = extractVarName(match[0])
    const isResolved = varName in vars
    builder.add(from, to, isResolved ? resolvedMark : unresolvedMark)
  }

  return builder.finish()
}

export function variableHighlightPlugin(getVars: () => Record<string, string>) {
  return ViewPlugin.fromClass(
    class {
      decorations: DecorationSet

      constructor(view: EditorView) {
        this.decorations = buildDecorations(view, getVars())
      }

      update(update: ViewUpdate) {
        const varsChanged = update.transactions.some(tr =>
          tr.effects.some(e => e.is(varsChangedEffect))
        )
        if (update.docChanged || update.viewportChanged || varsChanged) {
          this.decorations = buildDecorations(update.view, getVars())
        }
      }
    },
    {
      decorations: (v) => v.decorations,
    }
  )
}

export function variableHoverTooltip(
  getVars: () => Record<string, string>,
  getSecrets?: () => Set<string>,
) {
  return hoverTooltip(
    (view, pos): Tooltip | null => {
      const doc = view.state.doc.toString()

      for (const match of doc.matchAll(VAR_PATTERN)) {
        const from = match.index!
        const to = from + match[0].length
        if (pos < from || pos > to) continue

        const varName = extractVarName(match[0])
        const vars = getVars()
        const isResolved = varName in vars
        const isSecret = getSecrets?.().has(varName) ?? false

        return {
          pos: from,
          end: to,
          above: false,
          create() {
            const dom = document.createElement('div')
            dom.className = 'gc-var-tooltip'
            dom.setAttribute('role', 'tooltip')
            dom.style.cssText =
              'min-width:240px;max-width:360px;' +
              'background:var(--popover);color:var(--popover-foreground);' +
              'border:1px solid var(--border);border-radius:6px;' +
              'padding:8px 10px;font-size:12px;' +
              'box-shadow:0 4px 12px rgba(0,0,0,0.2);'
            dom.addEventListener('mousemove', (e) => e.stopPropagation())

            const row1 = document.createElement('div')
            row1.style.cssText =
              'display:flex;align-items:center;justify-content:space-between;' +
              'gap:8px;margin-bottom:4px;'
            const nameSpan = document.createElement('span')
            nameSpan.style.cssText =
              'font-family:ui-monospace,monospace;font-weight:500;'
            nameSpan.textContent = `{{${varName}}}`
            row1.appendChild(nameSpan)

            const scopePill = document.createElement('span')
            scopePill.style.cssText =
              'display:inline-flex;align-items:center;gap:4px;' +
              'padding:1px 6px;border-radius:3px;font-size:10px;' +
              'background:color-mix(in srgb, var(--primary) 20%, transparent);' +
              'color:var(--primary);font-weight:500;'
            scopePill.textContent = isResolved ? 'Environment' : 'Undefined'
            row1.appendChild(scopePill)
            dom.appendChild(row1)

            const row2 = document.createElement('div')
            row2.style.cssText =
              'font-family:ui-monospace,monospace;font-size:11px;' +
              'white-space:nowrap;overflow:hidden;text-overflow:ellipsis;' +
              'margin-bottom:6px;'
            if (isResolved) {
              row2.style.color = 'var(--gc-success, #10b981)'
              row2.textContent = isSecret ? '••••••••' : vars[varName]
            } else {
              row2.style.color = 'var(--muted-foreground)'
              row2.textContent = 'Not defined in active environment'
            }
            dom.appendChild(row2)

            const action = document.createElement('a')
            action.href = '#'
            action.style.cssText =
              'display:inline-flex;align-items:center;gap:4px;' +
              'color:var(--primary);text-decoration:none;font-size:11px;' +
              'cursor:pointer;'
            action.textContent = isResolved
              ? 'Open in Environment →'
              : '+ Create in Environment'
            action.addEventListener('click', (e) => {
              e.preventDefault()
              e.stopPropagation()
              const ui = useEnvModalUi()
              ui.openForVariable(varName, isResolved ? 'focus' : 'prefill')
            })
            dom.appendChild(action)

            return { dom }
          },
        }
      }
      return null
    },
    { hoverTime: 250, hideOn: (tr) => tr.docChanged },
  )
}

// Wildcard selectors override inner syntax highlighting spans
export const variableHighlightTheme = EditorView.theme({
  '.cm-var-resolved, .cm-var-resolved *': {
    color: 'var(--primary) !important',
    backgroundColor: 'color-mix(in srgb, var(--primary) 20%, transparent)',
    borderRadius: '2px',
  },
  '.cm-var-unresolved, .cm-var-unresolved *': {
    color: '#f97316 !important',
    backgroundColor: 'rgba(249, 115, 22, 0.2)',
    borderRadius: '2px',
  },
  '.cm-tooltip': {
    border: 'none !important',
    backgroundColor: 'transparent !important',
  },
  '.cm-tooltip-autocomplete': {
    backgroundColor: 'var(--popover) !important',
    border: '1px solid var(--border) !important',
    borderRadius: '6px !important',
  },
  '.cm-tooltip-autocomplete ul li': {
    color: 'var(--popover-foreground)',
  },
  '.cm-tooltip-autocomplete ul li[aria-selected]': {
    backgroundColor: 'color-mix(in srgb, var(--primary) 30%, transparent) !important',
    color: 'var(--popover-foreground) !important',
  },
  '.cm-completionLabel': {
    color: 'var(--primary) !important',
    fontFamily: 'ui-monospace, monospace',
  },
  '.cm-completionDetail': {
    color: 'var(--muted-foreground) !important',
    fontStyle: 'normal !important',
  },
})

// When getAvailableVars is provided, uses full variable list (includes vars with empty values).
// Otherwise falls back to resolved vars only (backward compat for CodeEditor).
export function variableCompletionSource(
  getVars: () => Record<string, string>,
  getSecrets?: () => Set<string>,
  getAvailableVars?: () => Array<{ key: string; value: string; isSecret: boolean }>
) {
  return (context: CompletionContext): CompletionResult | null => {
    const before = context.matchBefore(/\{\{[\w]*/)
    if (!before) return null

    let options: Array<{ label: string; detail: string; apply: string }>

    if (getAvailableVars) {
      const available = getAvailableVars()
      if (available.length === 0) return null
      options = available.map(v => ({
        label: v.key,
        detail: v.isSecret ? '••••••••' : (v.value.length > 30 ? v.value.slice(0, 30) + '…' : v.value || '(empty)'),
        apply: v.key + '}}',
      }))
    } else {
      const vars = getVars()
      const secrets = getSecrets?.() ?? new Set<string>()
      options = Object.entries(vars).map(([key, value]) => ({
        label: key,
        detail: secrets.has(key) ? '••••••••' : (value.length > 30 ? value.slice(0, 30) + '…' : value),
        apply: key + '}}',
      }))
      if (options.length === 0) return null
    }

    return {
      from: before.from + 2, // after {{
      options,
    }
  }
}

export function variableClickToEdit(getVars: () => Record<string, string>) {
  return EditorView.domEventHandlers({
    mousedown(event, view) {
      if (!(event.metaKey || event.ctrlKey)) return false
      const pos = view.posAtCoords({ x: event.clientX, y: event.clientY })
      if (pos === null) return false

      const doc = view.state.doc.toString()
      for (const match of doc.matchAll(VAR_PATTERN)) {
        const from = match.index!
        const to = from + match[0].length
        if (pos < from || pos > to) continue

        event.preventDefault()
        const varName = extractVarName(match[0])
        const isResolved = varName in getVars()
        const ui = useEnvModalUi()
        ui.openForVariable(varName, isResolved ? 'focus' : 'prefill')
        return true
      }
      return false
    },
  })
}
