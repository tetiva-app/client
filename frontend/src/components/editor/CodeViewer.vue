<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { EditorView, drawSelection, keymap } from '@codemirror/view'
import { EditorState, Compartment } from '@codemirror/state'
import { selectAll } from '@codemirror/commands'
import { search, SearchQuery, setSearchQuery, findNext as cmFindNext, findPrevious as cmFindPrev } from '@codemirror/search'
import { json } from '@codemirror/lang-json'
import { xml } from '@codemirror/lang-xml'
import { html } from '@codemirror/lang-html'
import { syntaxHighlighting } from '@codemirror/language'
import { BRAND_ACCENT_SELECTION } from '@/constants/defaults'
import { useSettingsStore } from '@/stores/settings'
import { darkHighlightStyle, lightHighlightStyle } from '@/lib/codemirror-highlight'

const props = defineProps<{
  content: string
  language?: 'json' | 'xml' | 'html' | 'text'
}>()

const editorRef = ref<HTMLDivElement>()
let view: EditorView | null = null

const settings = useSettingsStore()
const wrapCompartment = new Compartment()
const highlightCompartment = new Compartment()

function activeHighlight() {
  return syntaxHighlighting(settings.effectiveTheme === 'dark' ? darkHighlightStyle : lightHighlightStyle)
}

function getLanguageExtension(lang?: string) {
  switch (lang) {
    case 'json': return json()
    case 'xml': return xml()
    case 'html': return html()
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
    // readOnly keeps contenteditable so native selection and Cmd/Ctrl+C work.
    // Do not use EditorView.editable.of(false) — it removes contenteditable and breaks copy.
    EditorState.readOnly.of(true),
    // Route Cmd/Ctrl+A through CM6's selectAll so the selection covers the full
    // document, not just the DOM-virtualized viewport — copy then reads state.doc.
    keymap.of([{ key: 'Mod-a', run: selectAll }]),
    wrapCompartment.of(settings.editorWordWrap ? EditorView.lineWrapping : []),
    drawSelection(),
    search({ top: true, createPanel: () => ({ dom: document.createElement('span') }) }),
    highlightCompartment.of(activeHighlight()),
    getLanguageExtension(props.language),
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
      '.cm-panels': {
        display: 'none',
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

defineExpose({ setSearch, findNext, findPrev })

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
