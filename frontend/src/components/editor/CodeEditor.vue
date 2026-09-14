<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { EditorView, keymap, drawSelection, placeholder as cmPlaceholder, ViewPlugin, Decoration } from '@codemirror/view'
import type { ViewUpdate, DecorationSet } from '@codemirror/view'
import { EditorState, RangeSetBuilder, Compartment } from '@codemirror/state'
import type { ChangeSpec } from '@codemirror/state'
import { json } from '@codemirror/lang-json'
import { xml } from '@codemirror/lang-xml'
import { javascript } from '@codemirror/lang-javascript'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { search, SearchQuery, setSearchQuery, findNext as cmFindNext, findPrevious as cmFindPrev } from '@codemirror/search'
import { syntaxHighlighting, syntaxTree } from '@codemirror/language'
import { linter, lintGutter } from '@codemirror/lint'
import { autocompletion } from '@codemirror/autocomplete'
import { BRAND_ACCENT_SELECTION } from '@/constants/defaults'
import { variableHighlightPlugin, variableHighlightTheme, variableHoverTooltip, variableClickToEdit, variableCompletionSource, varsChangedEffect } from '@/lib/codemirror-variables'
import { scriptHintsSource } from '@/lib/codemirror-script-hints'
import { useSettingsStore } from '@/stores/settings'
import { darkHighlightStyle, lightHighlightStyle } from '@/lib/codemirror-highlight'
import { externalReplace, isExternal } from '@/lib/cm-external'

const props = defineProps<{
  content: string
  language?: 'json' | 'xml' | 'text' | 'javascript'
  resolvedVariables?: Record<string, string>
  secretKeys?: Set<string>
  placeholder?: string
}>()

const emit = defineEmits<{
  (e: 'update:content', value: string): void
}>()

const editorRef = ref<HTMLDivElement>()
let view: EditorView | null = null

const settings = useSettingsStore()
const wrapCompartment = new Compartment()
const highlightCompartment = new Compartment()

function activeHighlight() {
  return syntaxHighlighting(settings.effectiveTheme === 'dark' ? darkHighlightStyle : lightHighlightStyle)
}

// The linter skips errors overlapping {{variable}} or JSONC comments — both are
// stripped on the backend before sending (request.stripJSONC in Go).
const VAR_PATTERN = /\{\{([^}]+)\}\}/g

// Ranges of JSONC comments; comment markers inside string literals don't count.
function jsoncCommentRanges(doc: string): { from: number; to: number }[] {
  const ranges: { from: number; to: number }[] = []
  const n = doc.length
  let i = 0
  while (i < n) {
    const c = doc[i]
    if (c === '"') {
      i++
      while (i < n) {
        const ch = doc[i]
        i++
        if (ch === '\\' && i < n) { i++; continue }
        if (ch === '"') break
      }
      continue
    }
    if (c === '/' && i + 1 < n && doc[i + 1] === '/') {
      const from = i
      i += 2
      while (i < n && doc[i] !== '\n') i++
      ranges.push({ from, to: i })
      continue
    }
    if (c === '/' && i + 1 < n && doc[i + 1] === '*') {
      const from = i
      i += 2
      while (i < n) {
        if (doc[i] === '*' && i + 1 < n && doc[i + 1] === '/') { i += 2; break }
        i++
      }
      ranges.push({ from, to: i })
      continue
    }
    i++
  }
  return ranges
}

const jsonLinter = linter((view) => {
  const doc = view.state.doc.toString()
  if (doc.trim() === '') return []

  const skipRanges: { from: number; to: number }[] = []
  for (const match of doc.matchAll(VAR_PATTERN)) {
    skipRanges.push({ from: match.index!, to: match.index! + match[0].length })
  }
  skipRanges.push(...jsoncCommentRanges(doc))

  const diagnostics: { from: number; to: number; severity: 'error'; message: string }[] = []
  syntaxTree(view.state).iterate({
    enter(node) {
      if (node.type.isError) {
        const nodeTo = node.to > node.from ? node.to : node.from + 1
        const overlaps = skipRanges.some(r => node.from < r.to && nodeTo > r.from)
        if (overlaps) return
        diagnostics.push({
          from: node.from,
          to: nodeTo,
          severity: 'error',
          message: 'Syntax error',
        })
      }
    },
  })
  return diagnostics
})

// Decoration for JSONC comments — lezer-json doesn't recognize them as
// comment tokens, so we apply a styling pass on top of the parser.
const jsoncCommentMark = Decoration.mark({ class: 'cm-jsonc-comment' })

const jsoncCommentHighlighter = ViewPlugin.fromClass(class {
  decorations: DecorationSet
  constructor(view: EditorView) { this.decorations = this.build(view) }
  update(update: ViewUpdate) {
    if (update.docChanged || update.viewportChanged) {
      this.decorations = this.build(update.view)
    }
  }
  build(view: EditorView): DecorationSet {
    const builder = new RangeSetBuilder<Decoration>()
    const ranges = jsoncCommentRanges(view.state.doc.toString())
    for (const r of ranges) builder.add(r.from, r.to, jsoncCommentMark)
    return builder.finish()
  }
}, { decorations: v => v.decorations })

// Mod-/ toggle: if every targeted line is already commented, uncomment all;
// otherwise insert "// " after each line's leading indent.
function jsoncToggleLineComment(view: EditorView): boolean {
  const { state } = view
  const lineNumbers = new Set<number>()
  for (const range of state.selection.ranges) {
    let line = state.doc.lineAt(range.from)
    while (true) {
      lineNumbers.add(line.number)
      if (line.to >= range.to) break
      if (line.number >= state.doc.lines) break
      line = state.doc.line(line.number + 1)
    }
  }
  const lines = Array.from(lineNumbers).sort((a, b) => a - b).map(n => state.doc.line(n))
  if (lines.length === 0) return false

  const allCommented = lines.every(l => /^\s*\/\//.test(l.text))
  const changes: ChangeSpec[] = []
  for (const line of lines) {
    if (allCommented) {
      const m = line.text.match(/^(\s*)(\/\/ ?)/)
      if (m) {
        changes.push({ from: line.from + m[1].length, to: line.from + m[1].length + m[2].length })
      }
    } else {
      const indent = (line.text.match(/^\s*/) ?? [''])[0].length
      changes.push({ from: line.from + indent, insert: '// ' })
    }
  }
  if (changes.length === 0) return false
  view.dispatch({ changes, userEvent: 'input.comment' })
  return true
}

const jsoncCommentKeymap = keymap.of([
  { key: 'Mod-/', run: jsoncToggleLineComment },
])

function getLanguageExtension(lang?: string) {
  const varSource = variableCompletionSource(() => props.resolvedVariables ?? {}, () => props.secretKeys ?? new Set<string>())
  switch (lang) {
    case 'json': return [
      json(),
      jsonLinter,
      lintGutter(),
      jsoncCommentHighlighter,
      jsoncCommentKeymap,
      autocompletion({ override: [varSource] }),
    ]
    case 'xml': return [xml(), autocompletion({ override: [varSource] })]
    case 'javascript': return [javascript(), autocompletion({ override: [scriptHintsSource, varSource] })]
    default: return []
  }
}

function createEditor() {
  if (!editorRef.value) return

  if (view) {
    view.destroy()
    view = null
  }

  const extensions = [
    wrapCompartment.of(settings.editorWordWrap ? EditorView.lineWrapping : []),
    drawSelection(),
    history(),
    keymap.of([...defaultKeymap, ...historyKeymap]),
    search({ top: true, createPanel: () => ({ dom: document.createElement('span') }) }),
    highlightCompartment.of(activeHighlight()),
    props.placeholder ? cmPlaceholder(props.placeholder) : [],
    getLanguageExtension(props.language),
    variableHighlightPlugin(() => props.resolvedVariables ?? {}),
    variableHoverTooltip(() => props.resolvedVariables ?? {}, () => props.secretKeys ?? new Set<string>()),
    variableHighlightTheme,
    variableClickToEdit(() => props.resolvedVariables ?? {}),
    EditorView.updateListener.of((update) => {
      if (update.docChanged && !update.transactions.some(isExternal)) {
        emit('update:content', update.state.doc.toString())
      }
    }),
    EditorView.theme({
      '&': {
        fontSize: 'var(--gc-editor-font-size, 13px)',
        fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
        backgroundColor: 'transparent',
      },
      '.cm-gutters': {
        display: 'none',
      },
      '.cm-gutter-lint': {
        display: 'block',
        width: '12px',
      },
      '.cm-lint-marker-error': {
        content: '"●"',
        color: '#F04438',
      },
      '.cm-tooltip-lint': {
        backgroundColor: 'var(--popover)',
        border: '1px solid var(--border)',
        borderRadius: '4px',
        color: 'var(--foreground)',
        fontSize: '12px',
      },
      '.cm-lintRange-error': {
        backgroundImage: 'none',
        textDecoration: 'wavy underline #F04438',
        textUnderlineOffset: '3px',
      },
      '.cm-content': {
        padding: '12px',
        cursor: 'text',
      },
      '.cm-scroller': {
        cursor: 'text',
      },
      '&.cm-focused': {
        outline: 'none',
      },
      '.cm-cursor, .cm-cursor-primary': {
        borderLeftColor: '#528bff',
        borderLeftWidth: '2px',
      },
      '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
        backgroundColor: BRAND_ACCENT_SELECTION + ' !important',
      },
      '.cm-content ::selection': {
        backgroundColor: BRAND_ACCENT_SELECTION,
      },
      '.cm-panels': {
        display: 'none',
      },
      '.cm-searchMatch': {
        backgroundColor: 'color-mix(in srgb, var(--primary) 30%, transparent)',
      },
      '.cm-searchMatch-selected': {
        backgroundColor: 'color-mix(in srgb, var(--primary) 50%, transparent)',
      },
      '.cm-jsonc-comment, .cm-jsonc-comment span': {
        color: '#5c6370 !important',
        fontStyle: 'italic',
      },
    }),
  ]

  view = new EditorView({
    state: EditorState.create({
      doc: props.content,
      extensions,
    }),
    parent: editorRef.value,
  })
}

// Update document content without recreating the editor
watch(() => props.content, (newContent) => {
  if (!view) return
  const spec = externalReplace(view.state, newContent)
  if (spec) view.dispatch(spec)
})

watch(() => props.resolvedVariables, () => {
  if (view) {
    view.dispatch({ effects: varsChangedEffect.of(undefined) })
  }
})

// React to settings changes without recreating the editor.
watch(() => settings.editorWordWrap, (wrap) => {
  view?.dispatch({ effects: wrapCompartment.reconfigure(wrap ? EditorView.lineWrapping : []) })
})
watch(() => settings.effectiveTheme, () => {
  view?.dispatch({ effects: highlightCompartment.reconfigure(activeHighlight()) })
})
watch(() => settings.editorFontSize, () => {
  // CSS handles the visual resize; re-measure so cursor coordinates stay correct.
  view?.requestMeasure()
})

watch(() => props.language, () => {
  createEditor()
})

onMounted(() => {
  createEditor()
})

onUnmounted(() => {
  if (view) {
    view.destroy()
    view = null
  }
})

function format() {
  if (!view || props.language !== 'json') return
  try {
    const parsed = JSON.parse(view.state.doc.toString())
    const formatted = JSON.stringify(parsed, null, 2)
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: formatted },
    })
  } catch {
    // Invalid JSON — do nothing
  }
}

function setSearch(query: string) {
  if (!view) return
  view.dispatch({
    effects: setSearchQuery.of(new SearchQuery({ search: query, caseSensitive: false, literal: true })),
  })
}

function findNext() {
  if (view) cmFindNext(view)
}

function findPrev() {
  if (view) cmFindPrev(view)
}

defineExpose({ format, setSearch, findNext, findPrev })
</script>

<template>
  <div
    ref="editorRef"
    class="h-full overflow-auto outline-none"
  />
</template>
