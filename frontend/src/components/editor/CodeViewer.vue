<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, onUnmounted, watch } from 'vue'
import { EditorView, drawSelection, keymap } from '@codemirror/view'
import { EditorState, Compartment } from '@codemirror/state'
import { selectAll } from '@codemirror/commands'
import { syntaxHighlighting } from '@codemirror/language'
import { BRAND_ACCENT_SELECTION } from '@/constants/defaults'
import { useSettingsStore } from '@/stores/settings'
import { darkHighlightStyle, lightHighlightStyle } from '@/lib/codemirror-highlight'
import { languageSupport, type ViewerLanguage } from '@/lib/codemirror-languages'
import { applyFind, findInDocument, stepFind, type FindStatus } from '@/lib/cm-find'

const props = defineProps<{
  content: string
  language?: ViewerLanguage
  label?: string
  search?: string
}>()

const emit = defineEmits<{
  (e: 'find-status', status: FindStatus | null): void
}>()

const editorRef = ref<HTMLDivElement>()
let view: EditorView | null = null

const settings = useSettingsStore()
const wrapCompartment = new Compartment()
const highlightCompartment = new Compartment()
const languageCompartment = new Compartment()

function activeHighlight() {
  return syntaxHighlighting(settings.effectiveTheme === 'dark' ? darkHighlightStyle : lightHighlightStyle)
}

function createEditor() {
  if (!editorRef.value) return

  if (view) {
    view.destroy()
    view = null
  }

  const lang = props.language
  const support = languageSupport(lang)

  const extensions = [
    // readOnly keeps contenteditable so native selection and Cmd/Ctrl+C work.
    // Do not use EditorView.editable.of(false) — it removes contenteditable and breaks copy.
    EditorState.readOnly.of(true),
    // Route Cmd/Ctrl+A through CM6's selectAll so the selection covers the full
    // document, not just the DOM-virtualized viewport — copy then reads state.doc.
    keymap.of([{ key: 'Mod-a', run: selectAll }]),
    wrapCompartment.of(settings.editorWordWrap ? EditorView.lineWrapping : []),
    drawSelection(),
    findInDocument(status => emit('find-status', status)),
    highlightCompartment.of(activeHighlight()),
    languageCompartment.of(support instanceof Promise ? [] : support),
    EditorView.contentAttributes.of(props.label ? { 'aria-label': props.label } : {}),
    EditorView.theme({
      '&': {
        fontSize: 'var(--gc-editor-font-size, 13px)',
        fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
        backgroundColor: 'transparent',
      },
      '.cm-gutters': {
        display: 'none',
      },
      '.cm-content': {
        padding: '12px',
        caretColor: 'transparent',
      },
      '&.cm-focused': {
        outline: 'none',
      },
      '.cm-cursor': {
        display: 'none',
      },
      '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
        backgroundColor: BRAND_ACCENT_SELECTION + ' !important',
      },
      '.cm-content ::selection': {
        backgroundColor: BRAND_ACCENT_SELECTION,
      },
      // display: none would zero the panel's rect, which CodeMirror reads as scroll margin.
      '.cm-panels': {
        height: '0',
        overflow: 'hidden',
        border: 'none',
      },
      '.cm-searchMatch': {
        backgroundColor: 'color-mix(in srgb, var(--primary) 30%, transparent)',
      },
      '.cm-searchMatch-selected': {
        backgroundColor: 'color-mix(in srgb, var(--primary) 50%, transparent)',
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
  if (props.search) applyFind(view, props.search)
  if (support instanceof Promise) {
    const created = view
    support.then((ext) => {
      if (view === created && props.language === lang) created.dispatch({ effects: languageCompartment.reconfigure(ext) })
    }, () => {})
  }
}

// Update document content without recreating the editor
watch(() => props.content, (newContent) => {
  if (!view) return
  const currentDoc = view.state.doc.toString()
  if (currentDoc !== newContent) {
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: newContent },
    })
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
  view?.requestMeasure()
})

watch(() => props.language, () => {
  createEditor()
})

onMounted(() => {
  createEditor()
})

watch(() => props.search, (query) => {
  if (view) applyFind(view, query ?? '')
})

function findNext() {
  if (view) stepFind(view, true)
}

function findPrev() {
  if (view) stepFind(view, false)
}

function focus() {
  view?.focus()
}

defineExpose({ findNext, findPrev, focus })

onBeforeUnmount(() => emit('find-status', null))

onUnmounted(() => {
  if (view) {
    view.destroy()
    view = null
  }
})
</script>

<template>
  <div
    ref="editorRef"
    class="h-full overflow-auto outline-none"
    @click="view?.focus()"
  />
</template>
