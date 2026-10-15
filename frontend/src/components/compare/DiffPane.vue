<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { Decoration, EditorView, drawSelection, keymap, lineNumbers, type DecorationSet } from '@codemirror/view'
import { Compartment, EditorState, RangeSetBuilder, StateEffect, StateField, type Extension, type StateCommand } from '@codemirror/state'
import { selectAll } from '@codemirror/commands'
import { syntaxHighlighting } from '@codemirror/language'
import { MergeView, getChunks, goToNextChunk, goToPreviousChunk, unifiedMergeView, type Chunk } from '@codemirror/merge'
import { BRAND_ACCENT_SELECTION } from '@/constants/defaults'
import { useSettingsStore } from '@/stores/settings'
import { useCopy } from '@/composables/useLocale'
import { darkHighlightStyle, lightHighlightStyle } from '@/lib/codemirror-highlight'
import { languageSupport } from '@/lib/codemirror-languages'
import { DIFF_CONFIG, chunkIndexAt, chunkLineStarts, type DiffPosition } from '@/lib/compare/chunks'
import type { BodyLanguage } from '@/lib/compare/body'
import { COMPARE_COPY } from './copy'

const props = defineProps<{
  a: string
  b: string
  language: BodyLanguage
  mode: 'split' | 'unified'
}>()

const emit = defineEmits<{
  (e: 'position', value: DiffPosition): void
}>()

const MERGE = {
  highlightChanges: true,
  gutter: true,
  collapseUnchanged: { margin: 3, minSize: 4 },
  diffConfig: DIFF_CONFIG,
}

const setCurrent = StateEffect.define<{ from: number; to: number } | null>()
const CURRENT_LINE = Decoration.line({ class: 'cm-currentChunk' })

const currentChunk = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(deco, tr) {
    for (const effect of tr.effects) {
      if (!effect.is(setCurrent)) continue
      if (!effect.value) return Decoration.none
      const lines = new RangeSetBuilder<Decoration>()
      for (const at of chunkLineStarts(tr.state.doc, effect.value.from, effect.value.to)) lines.add(at, at, CURRENT_LINE)
      return lines.finish()
    }
    return deco
  },
  provide: (field) => EditorView.decorations.from(field),
})

const tint = (color: string, percent: number) => `color-mix(in srgb, var(${color}) ${percent}%, transparent)`

const theme = EditorView.theme({
  '&': {
    fontSize: 'var(--gc-editor-font-size, 13px)',
    fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
    backgroundColor: 'transparent',
  },
  '.cm-mergeView &.cm-merge-b': { borderLeft: '1px solid var(--border)' },
  '.cm-content': { caretColor: 'transparent' },
  '&.cm-focused': { outline: 'none' },
  '.cm-cursor': { display: 'none' },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': { backgroundColor: BRAND_ACCENT_SELECTION + ' !important' },
  '.cm-content ::selection': { backgroundColor: BRAND_ACCENT_SELECTION },
  '.cm-gutters': { backgroundColor: 'transparent', border: 'none', color: 'var(--muted-foreground)' },
  '.cm-lineNumbers .cm-gutterElement': { padding: '0 8px 0 12px', fontSize: '0.9em' },
  '&.cm-merge-a .cm-changedLine, .cm-deletedChunk': { backgroundColor: tint('--gc-error', 9) },
  '&.cm-merge-b .cm-changedLine, .cm-inlineChangedLine': { backgroundColor: tint('--gc-success', 9) },
  '&.cm-merge-a .cm-changedText, .cm-deletedChunk .cm-deletedText': { background: tint('--gc-error', 22) },
  '&.cm-merge-b .cm-changedText': { background: tint('--gc-success', 22) },
  '&.cm-merge-a .cm-changedLineGutter, .cm-deletedLineGutter': { background: 'var(--gc-error)' },
  '&.cm-merge-b .cm-changedLineGutter': { background: 'var(--gc-success)' },
  '.cm-currentChunk': { boxShadow: 'inset 3px 0 0 var(--primary)' },
  '.cm-collapsedLines': {
    color: 'var(--muted-foreground)',
    background: 'var(--gc-surface)',
    borderTop: '1px solid var(--border)',
    borderBottom: '1px solid var(--border)',
    fontStyle: 'italic',
    fontSize: '12px',
  },
})

const settings = useSettingsStore()
const copy = useCopy(COMPARE_COPY)
const host = ref<HTMLDivElement>()
const wrap = new Compartment()
const highlight = new Compartment()

let merge: MergeView | null = null
let unified: EditorView | null = null

function activeHighlight(): Extension {
  return syntaxHighlighting(settings.effectiveTheme === 'dark' ? darkHighlightStyle : lightHighlightStyle)
}

function extensions(): Extension[] {
  return [
    EditorState.readOnly.of(true),
    keymap.of([
      { key: 'Mod-a', run: selectAll },
      { key: 'Alt-ArrowDown', run: () => { next(); return true } },
      { key: 'Alt-ArrowUp', run: () => { prev(); return true } },
    ]),
    currentChunk,
    EditorView.updateListener.of((update) => {
      if (update.selectionSet && update.view === target()) queueMicrotask(report)
    }),
    EditorState.phrases.of({ '$ unchanged lines': copy.value.cm.unchanged }),
    lineNumbers(),
    drawSelection(),
    wrap.of(settings.editorWordWrap ? EditorView.lineWrapping : []),
    highlight.of(activeHighlight()),
    language(),
    theme,
  ]
}

function language(): Extension {
  const support = languageSupport(props.language)
  return support instanceof Promise ? [] : support
}

function views(): EditorView[] {
  if (merge) return [merge.a, merge.b]
  return unified ? [unified] : []
}

function target(): EditorView | null {
  return merge?.b ?? unified
}

function chunks() {
  if (merge) return merge.chunks
  return unified ? getChunks(unified.state)?.chunks ?? [] : []
}

function markCurrent(chunk: Chunk | undefined) {
  const mark = (view: EditorView, from = 0, to = 0, keepEmpty = false) => {
    view.dispatch({ effects: setCurrent.of(chunk && (from < to || keepEmpty) ? { from, to } : null) })
  }
  if (merge) {
    mark(merge.a, chunk?.fromA, chunk?.toA)
    mark(merge.b, chunk?.fromB, chunk?.toB)
  } else if (unified) {
    mark(unified, chunk?.fromB, chunk?.toB, true)
  }
}

function report() {
  const view = target()
  if (!view) return
  const list = chunks()
  const index = chunkIndexAt(list, view.state.selection.main.head)
  markCurrent(list[index])
  emit('position', { index, total: list.length })
}

function destroy() {
  merge?.destroy()
  unified?.destroy()
  merge = null
  unified = null
}

function create() {
  destroy()
  if (!host.value) return
  if (props.mode === 'split') {
    merge = new MergeView({
      a: { doc: props.a, extensions: extensions() },
      b: { doc: props.b, extensions: extensions() },
      parent: host.value,
      ...MERGE,
    })
  } else {
    unified = new EditorView({
      state: EditorState.create({
        doc: props.b,
        extensions: [...extensions(), unifiedMergeView({ original: props.a, mergeControls: false, ...MERGE })],
      }),
      parent: host.value,
    })
  }
  const first = chunks()[0]
  if (first) target()?.dispatch({ selection: { anchor: first.fromB } })
  report()
}

function move(command: StateCommand) {
  const view = target()
  if (view) command(view)
}

const next = () => move(goToNextChunk)
const prev = () => move(goToPreviousChunk)

defineExpose({ next, prev })

watch([() => props.a, () => props.b, () => props.language, () => props.mode, () => copy.value.cm.unchanged], create)

watch(() => settings.editorWordWrap, (on) => {
  for (const view of views()) view.dispatch({ effects: wrap.reconfigure(on ? EditorView.lineWrapping : []) })
})
watch(() => settings.effectiveTheme, () => {
  for (const view of views()) view.dispatch({ effects: highlight.reconfigure(activeHighlight()) })
})
watch(() => settings.editorFontSize, () => {
  for (const view of views()) view.requestMeasure()
})

onMounted(create)
onUnmounted(destroy)
</script>

<template>
  <div ref="host" class="h-full overflow-auto" data-testid="diff-pane" />
</template>
