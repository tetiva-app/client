<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import { ExternalLink } from 'lucide-vue-next'
import { isWailsEnvironment, getWindowService } from '@/services'
import { EditorView, drawSelection } from '@codemirror/view'
import { EditorState, Compartment } from '@codemirror/state'
import { StreamLanguage } from '@codemirror/language'
import { protobuf } from '@codemirror/legacy-modes/mode/protobuf'
import { syntaxHighlighting } from '@codemirror/language'
import { BRAND_ACCENT_SELECTION } from '@/constants/defaults'
import { useSettingsStore } from '@/stores/settings'
import { darkHighlightStyle, lightHighlightStyle } from '@/lib/codemirror-highlight'
import { isModShortcut } from '@/lib/shortcut-guards'

const props = withDefaults(defineProps<{
  definition: string
  source: string
  hideDetach?: boolean
}>(), { hideDetach: false })

const editorRef = ref<HTMLDivElement>()
let view: EditorView | null = null

const settings = useSettingsStore()
const wrapCompartment = new Compartment()
const highlightCompartment = new Compartment()

function activeHighlight() {
  return syntaxHighlighting(settings.effectiveTheme === 'dark' ? darkHighlightStyle : lightHighlightStyle)
}

function createEditor() {
  if (!editorRef.value) return

  if (view) {
    view.destroy()
    view = null
  }

  const extensions = [
    EditorView.editable.of(false),
    EditorState.readOnly.of(true),
    wrapCompartment.of(settings.editorWordWrap ? EditorView.lineWrapping : []),
    drawSelection(),
    highlightCompartment.of(activeHighlight()),
    StreamLanguage.define(protobuf),
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
    }),
  ]

  view = new EditorView({
    state: EditorState.create({
      doc: props.definition,
      extensions,
    }),
    parent: editorRef.value,
  })
}

watch(() => props.definition, (newContent) => {
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

function selectAll() {
  if (!view) return
  view.focus()
  view.dispatch({ selection: { anchor: 0, head: view.state.doc.length } })
}

function handleKeydown(e: KeyboardEvent) {
  if (isModShortcut(e, 'KeyA', 'a')) {
    e.preventDefault()
    e.stopPropagation()
    selectAll()
  }
}

const canDetach = computed(() => !props.hideDetach && isWailsEnvironment() && props.definition.length > 0)

async function openInWindow() {
  const svc = await getWindowService()
  if (!svc) return
  const sourceLabel = props.source === 'reflection' ? 'Server Reflection' : 'Proto File'
  await svc.openSchemaViewer(props.definition, props.source, `Schema — ${sourceLabel}`)
}

onMounted(() => {
  createEditor()
})

onUnmounted(() => {
  if (view) {
    view.destroy()
    view = null
  }
})
</script>

<template>
  <div class="flex flex-col h-full">
    <div v-if="canDetach" class="flex items-center justify-end h-7 px-2 border-b border-border shrink-0">
      <button
        class="flex items-center gap-1 px-1.5 py-0.5 text-[10px] text-muted-foreground hover:text-foreground rounded transition-colors cursor-pointer"
        title="Open in Window"
        @click="openInWindow"
      >
        <ExternalLink class="size-3" />
        <span>Open in Window</span>
      </button>
    </div>

    <div class="flex-1 overflow-auto">
      <div
        ref="editorRef"
        tabindex="0"
        class="h-full overflow-auto outline-none"
        @click="view?.focus()"
        @keydown="handleKeydown"
      />
    </div>

    <div class="flex items-center h-7 px-3 border-t border-border text-[10px] text-muted-foreground/60 shrink-0">
      {{ source === 'reflection' ? 'Source: Server reflection' : source === 'proto_file' ? 'Source: Proto file' : source === 'proto_directory' ? 'Source: Proto directory' : 'Source: ' + source }}
    </div>
  </div>
</template>
