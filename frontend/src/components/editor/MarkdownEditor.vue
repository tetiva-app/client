<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { EditorView, keymap, drawSelection, placeholder as cmPlaceholder } from '@codemirror/view'
import { EditorState, Compartment, Prec } from '@codemirror/state'
import { defaultKeymap, history, historyKeymap, indentMore, indentLess } from '@codemirror/commands'
import { markdown, markdownLanguage } from '@codemirror/lang-markdown'
import { syntaxHighlighting, indentUnit } from '@codemirror/language'
import { autocompletion, closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete'
import type { Completion, CompletionContext, CompletionResult } from '@codemirror/autocomplete'
import { BRAND_ACCENT_SELECTION } from '@/constants/defaults'
import { useSettingsStore } from '@/stores/settings'
import { markdownDarkHighlightStyle, markdownLightHighlightStyle } from '@/lib/codemirror-highlight'
import { applyMarkdownAction, blockInsert, SLASH_MENU_ITEMS, tableSkeleton } from '@/lib/markdown-actions'
import type { MarkdownAction } from '@/lib/markdown-actions'
import {
  disabledTableCommands, inTable, isInsideCodeAtLine, normalizeTables, runTableCommand,
  tableEnter, tableShiftTab, tableTab,
} from '@/lib/markdown-table-editor'
import type { TableCommand } from '@/lib/markdown-table-editor'
import { isTsv, tsvToMarkdownTable } from '@/lib/markdown-paste'
import { externalReplace, isExternal } from '@/lib/cm-external'
import MarkdownToolbar from './MarkdownToolbar.vue'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  minHeight?: string
}>(), { placeholder: '', minHeight: '120px' })

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const editorRef = ref<HTMLDivElement>()
const toolbarRef = ref<{ openTablePicker: () => void }>()
const inTableState = ref(false)
const insideFence = ref(false)
const disabledCommands = ref<TableCommand[]>([])
let view: EditorView | null = null

const settings = useSettingsStore()
const highlightCompartment = new Compartment()

function activeHighlight() {
  return syntaxHighlighting(
    settings.effectiveTheme === 'dark' ? markdownDarkHighlightStyle : markdownLightHighlightStyle,
  )
}

// A fence opened inside a fence closes that one and turns the rest of the document into code.
function fenceAtSelection(state: EditorState): boolean {
  const range = state.selection.main
  return isInsideCodeAtLine(state, range.from) || isInsideCodeAtLine(state, range.to)
}

function runAction(action: MarkdownAction) {
  if (!view) return
  const range = view.state.selection.main
  if (action === 'codeBlock' && fenceAtSelection(view.state)) {
    view.focus()
    return
  }
  const edit = applyMarkdownAction(action, view.state.doc.toString(), range.from, range.to)
  view.dispatch({
    changes: { from: edit.from, to: edit.to, insert: edit.insert },
    selection: { anchor: edit.selectionFrom, head: edit.selectionTo },
    userEvent: 'input.markdown',
    scrollIntoView: true,
  })
  view.focus()
}

// Reka returns focus to its trigger when a popup closes; take it back afterwards.
function focusEditor() {
  nextTick(() => view?.focus())
}

function runTable(command: TableCommand) {
  if (!view) return
  runTableCommand(view, command)
  focusEditor()
}

function insertTable(columns: number, rows: number) {
  if (!view) return
  // A table cannot wrap the selection the way bold or quote do, so it goes after it
  // rather than replacing it.
  const { to } = view.state.selection.main
  const skeleton = tableSkeleton(columns, rows)
  const edit = blockInsert(view.state.doc.toString(), to, to, skeleton, 2, 2 + 'Column 1'.length)
  view.dispatch({
    changes: { from: edit.from, to: edit.to, insert: edit.insert },
    selection: { anchor: edit.selectionFrom, head: edit.selectionTo },
    userEvent: 'input.markdown',
    scrollIntoView: true,
  })
  focusEditor()
}

function handlePaste(event: ClipboardEvent, v: EditorView): boolean {
  const text = event.clipboardData?.getData('text/plain') ?? ''
  const pos = v.state.selection.main.from
  if (!isTsv(text) || inTable(v) || isInsideCodeAtLine(v.state, pos)) return false
  event.preventDefault()
  const { from, to } = v.state.selection.main
  const table = tsvToMarkdownTable(text)
  const edit = blockInsert(v.state.doc.toString(), from, to, table, table.length, table.length)
  v.dispatch({
    changes: { from: edit.from, to: edit.to, insert: edit.insert },
    selection: { anchor: edit.selectionFrom },
    userEvent: 'input.paste',
    scrollIntoView: true,
  })
  return true
}

// closeBrackets pairs the third backtick of a fence into a fourth one, because it only
// knows the triple form for languages that list it as a bracket.
const fenceBackticks = Prec.high(EditorView.inputHandler.of((v, from, to, insert) => {
  if (insert !== '`' || from !== to || v.state.sliceDoc(Math.max(0, from - 2), from) !== '``') return false
  v.dispatch({
    changes: { from, insert: '`' },
    selection: { anchor: from + 1 },
    userEvent: 'input.type',
    scrollIntoView: true,
  })
  return true
}))

// GitHub-style: the menu only opens on a slash that starts the line.
function slashMenuSource(context: CompletionContext): CompletionResult | null {
  const match = context.matchBefore(/^\/\w*/)
  if (!match) return null
  const items = SLASH_MENU_ITEMS.filter(item => item.action !== 'codeBlock' || !insideFence.value)
  return {
    from: match.from,
    validFor: /^\/\w*$/,
    options: items.map((item, index): Completion => ({
      label: item.label,
      detail: item.detail,
      type: 'keyword',
      // Equal fuzzy scores are broken alphabetically, which would open on /code.
      boost: items.length - index,
      apply: (target, _completion, from, to) => {
        target.dispatch({ changes: { from, to, insert: '' }, selection: { anchor: from } })
        if (item.action === 'table') toolbarRef.value?.openTablePicker()
        else runAction(item.action)
      },
    })),
  }
}

function createEditor() {
  if (!editorRef.value) return

  const extensions = [
    EditorView.lineWrapping,
    drawSelection(),
    history(),
    indentUnit.of('  '),
    Prec.high(keymap.of([
      { key: 'Mod-b', run: () => { runAction('bold'); return true } },
      { key: 'Mod-i', run: () => { runAction('italic'); return true } },
      { key: 'Mod-k', run: () => { runAction('link'); return true } },
      // Mod-Enter sends the request; swallow it so defaultKeymap's
      // insertBlankLine does not fire on the way out.
      { key: 'Mod-Enter', run: () => true },
      // Table bindings fall through outside a table, so the list ones below still run.
      { key: 'Tab', run: tableTab, shift: tableShiftTab },
      { key: 'Enter', run: tableEnter },
      { key: 'Tab', run: indentMore, shift: indentLess },
      { key: 'Escape', run: (v) => { v.contentDOM.blur(); return true } },
    ])),
    keymap.of([...closeBracketsKeymap, ...defaultKeymap, ...historyKeymap]),
    fenceBackticks,
    closeBrackets(),
    markdownLanguage.data.of({ closeBrackets: { brackets: ['(', '[', '{', "'", '"', '`'] } }),
    markdown({ base: markdownLanguage }),
    highlightCompartment.of(activeHighlight()),
    autocompletion({ override: [slashMenuSource], icons: false }),
    cmPlaceholder(props.placeholder),
    // Prec.high: lang-markdown's own paste handler must not get first refusal.
    Prec.high(EditorView.domEventHandlers({ paste: handlePaste })),
    EditorView.domEventHandlers({ blur: (_event, v) => { normalizeTables(v); return false } }),
    EditorView.updateListener.of((update) => {
      if (update.docChanged && !update.transactions.some(isExternal)) {
        emit('update:modelValue', update.state.doc.toString())
      }
      if (update.docChanged || update.selectionSet) {
        inTableState.value = inTable(update.view)
        disabledCommands.value = inTableState.value ? disabledTableCommands(update.view) : []
        insideFence.value = fenceAtSelection(update.state)
      }
    }),
    EditorView.theme({
      '&': {
        fontSize: 'var(--gc-editor-font-size, 13px)',
        fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
        backgroundColor: 'transparent',
      },
      '&.cm-focused': { outline: 'none' },
      '.cm-content': {
        padding: '10px 12px',
        cursor: 'text',
        minHeight: props.minHeight,
      },
      '.cm-scroller': { cursor: 'text', lineHeight: '1.6' },
      '.cm-cursor, .cm-cursor-primary': {
        borderLeftColor: 'var(--foreground)',
        borderLeftWidth: '2px',
      },
      '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
        backgroundColor: BRAND_ACCENT_SELECTION + ' !important',
      },
      '.cm-content ::selection': { backgroundColor: BRAND_ACCENT_SELECTION },
      '.cm-tooltip-autocomplete': {
        backgroundColor: 'var(--popover)',
        border: '1px solid var(--border)',
        borderRadius: '6px',
        fontFamily: 'ui-sans-serif, system-ui, sans-serif',
        fontSize: '12px',
      },
      '.cm-tooltip.cm-tooltip-autocomplete > ul': {
        fontFamily: 'ui-sans-serif, system-ui, sans-serif',
      },
      '.cm-tooltip.cm-tooltip-autocomplete > ul > li': {
        padding: '3px 8px',
        color: 'var(--foreground)',
        cursor: 'pointer',
      },
      // Weaker than the selected state, and declared first so the two never tie.
      '.cm-tooltip.cm-tooltip-autocomplete > ul > li:hover': {
        backgroundColor: 'color-mix(in srgb, var(--primary) 15%, transparent)',
      },
      // --accent is a near-popover grey in both themes; the brand token is the
      // only one that reads as a selection on top of --popover.
      '.cm-tooltip.cm-tooltip-autocomplete > ul > li[aria-selected]': {
        backgroundColor: 'var(--primary)',
        color: 'var(--primary-foreground)',
      },
      '.cm-completionLabel': { fontWeight: '500' },
      '.cm-completionDetail': {
        color: 'var(--muted-foreground)',
        fontStyle: 'normal',
        marginLeft: '8px',
      },
      // Muted grey drops to 1.2-1.6:1 once the row is filled.
      '.cm-tooltip.cm-tooltip-autocomplete > ul > li[aria-selected] .cm-completionDetail': {
        color: 'var(--primary-foreground)',
      },
    }),
  ]

  view = new EditorView({
    state: EditorState.create({ doc: props.modelValue, extensions }),
    parent: editorRef.value,
  })
  inTableState.value = inTable(view)
  disabledCommands.value = inTableState.value ? disabledTableCommands(view) : []
  insideFence.value = fenceAtSelection(view.state)
}

watch(() => props.modelValue, (next) => {
  if (!view) return
  const spec = externalReplace(view.state, next)
  if (spec) view.dispatch(spec)
})

watch(() => settings.effectiveTheme, () => {
  view?.dispatch({ effects: highlightCompartment.reconfigure(activeHighlight()) })
})

onMounted(createEditor)

onUnmounted(() => {
  view?.destroy()
  view = null
})

defineExpose({
  focus() {
    // The editor is mounted while hidden, so its layout is stale on first show.
    view?.requestMeasure()
    view?.focus()
  },
  normalizeTables: () => { if (view) normalizeTables(view) },
})
</script>

<template>
  <div class="rounded-md border border-input bg-background focus-within:ring-1 focus-within:ring-ring">
    <MarkdownToolbar
      ref="toolbarRef"
      :in-table="inTableState"
      :inside-fence="insideFence"
      :disabled-commands="disabledCommands"
      @action="runAction"
      @table-command="runTable"
      @insert-table="insertTable"
      @closed="focusEditor"
    />
    <div ref="editorRef" class="overflow-auto" />
  </div>
</template>
