<script setup lang="ts">
import { ref, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { ChevronDown, ChevronUp } from 'lucide-vue-next'
import { EditorView, drawSelection, keymap } from '@codemirror/view'
import { EditorState, Compartment } from '@codemirror/state'
import { json } from '@codemirror/lang-json'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import { syntaxHighlighting } from '@codemirror/language'
import { useSettingsStore } from '@/stores/settings'
import { darkHighlightStyle, lightHighlightStyle } from '@/lib/codemirror-highlight'
import { graphql, updateSchema } from 'cm6-graphql'
import { autocompletion } from '@codemirror/autocomplete'
import { BRAND_ACCENT_SELECTION } from '@/constants/defaults'
import type { Request } from '@/types/request'
import type { GraphQLSchema as AppGraphQLSchema } from '@/types/graphql'
import { buildGqlJsSchema } from '@/lib/graphql-schema-builder'

const props = defineProps<{
  request: Request
  schema: AppGraphQLSchema | null
}>()

const emit = defineEmits<{
  (e: 'update:query', value: string): void
  (e: 'update:variables', value: string): void
  (e: 'show-in-docs', typeName: string): void
}>()

let gqlJsSchema: ReturnType<typeof import('graphql').buildSchema> | null = null

const queryEditorRef = ref<HTMLDivElement>()
const variablesEditorRef = ref<HTMLDivElement>()
const variablesOpen = ref(true)

let queryView: EditorView | null = null
let variablesView: EditorView | null = null
let ignoreQueryUpdate = false
let ignoreVarsUpdate = false

const settings = useSettingsStore()
const queryWrapCompartment = new Compartment()
const queryHighlightCompartment = new Compartment()
const varsWrapCompartment = new Compartment()
const varsHighlightCompartment = new Compartment()

function activeHighlight() {
  return syntaxHighlighting(settings.effectiveTheme === 'dark' ? darkHighlightStyle : lightHighlightStyle)
}

const baseTheme = EditorView.theme({
  '&': {
    fontSize: 'var(--gc-editor-font-size, 13px)',
    fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
    backgroundColor: 'transparent',
  },
  '.cm-gutters': { display: 'none' },
  '.cm-content': { padding: '12px', cursor: 'text' },
  '.cm-scroller': { cursor: 'text' },
  '&.cm-focused': { outline: 'none' },
  '.cm-cursor, .cm-cursor-primary': { borderLeftColor: '#528bff', borderLeftWidth: '2px' },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
    backgroundColor: BRAND_ACCENT_SELECTION + ' !important',
  },
  '.cm-content ::selection': { backgroundColor: BRAND_ACCENT_SELECTION },
  '.cm-tooltip': {
    backgroundColor: 'var(--popover)',
    color: 'var(--popover-foreground)',
    border: '1px solid var(--border)',
    borderRadius: '6px',
    fontSize: '12px',
    fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
  },
  '.cm-tooltip-autocomplete > ul': {
    fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
    fontSize: '12px',
  },
  '.cm-tooltip-autocomplete > ul > li': {
    cursor: 'pointer',
    padding: '4px 8px',
    borderRadius: '4px',
    margin: '1px 4px',
  },
  '.cm-tooltip-autocomplete > ul > li:hover': {
    backgroundColor: 'rgba(255, 255, 255, 0.08)',
  },
  '.cm-tooltip-autocomplete > ul > li[aria-selected="true"]': {
    backgroundColor: '#6C5CE7 !important',
    color: '#ffffff !important',
  },
  '.cm-tooltip-lint': {
    backgroundColor: 'var(--popover)',
    color: 'var(--popover-foreground)',
    border: '1px solid var(--border)',
    borderRadius: '6px',
  },
  // Cmd/Ctrl held: show pointer + underline hint on tokens
  '&.cm-meta-held .cm-content': {
    cursor: 'pointer',
  },
  '&.cm-meta-held .cm-content span': {
    textDecoration: 'underline',
    textDecorationColor: '#A78BFA',
  },
})

function createQueryEditor() {
  if (!queryEditorRef.value) return
  if (queryView) { queryView.destroy(); queryView = null }

  gqlJsSchema = props.schema ? buildGqlJsSchema(props.schema) : null

  queryView = new EditorView({
    state: EditorState.create({
      doc: props.request.graphqlQuery || '',
      extensions: [
        queryWrapCompartment.of(settings.editorWordWrap ? EditorView.lineWrapping : []),
        drawSelection(),
        history(),
        keymap.of([...defaultKeymap, ...historyKeymap]),
        queryHighlightCompartment.of(activeHighlight()),
        autocompletion(),
        graphql(gqlJsSchema ?? undefined, {
          onShowInDocs(_field, type, _parentType) {
            if (!type) return
            const baseName = type.replace(/[[\]!]/g, '')
            if (baseName) emit('show-in-docs', baseName)
          },
        }),
        // Cmd/Ctrl held visual indicator for jump-to-docs
        EditorView.domEventHandlers({
          keydown(e, view) {
            if (e.metaKey || e.ctrlKey) view.dom.classList.add('cm-meta-held')
          },
          keyup(e, view) {
            if (!e.metaKey && !e.ctrlKey) view.dom.classList.remove('cm-meta-held')
          },
          blur(_e, view) {
            view.dom.classList.remove('cm-meta-held')
          },
        }),
        baseTheme,
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            const val = update.state.doc.toString()
            ignoreQueryUpdate = true
            emit('update:query', val)
          }
        }),
      ],
    }),
    parent: queryEditorRef.value,
  })
}

function createVariablesEditor() {
  if (!variablesEditorRef.value) return
  if (variablesView) { variablesView.destroy(); variablesView = null }

  variablesView = new EditorView({
    state: EditorState.create({
      doc: props.request.graphqlVariables || '',
      extensions: [
        varsWrapCompartment.of(settings.editorWordWrap ? EditorView.lineWrapping : []),
        drawSelection(),
        history(),
        keymap.of([...defaultKeymap, ...historyKeymap]),
        json(),
        varsHighlightCompartment.of(activeHighlight()),
        baseTheme,
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            const val = update.state.doc.toString()
            ignoreVarsUpdate = true
            emit('update:variables', val)
          }
        }),
      ],
    }),
    parent: variablesEditorRef.value,
  })
}

watch(() => props.request.graphqlQuery, (val) => {
  if (ignoreQueryUpdate) { ignoreQueryUpdate = false; return }
  if (!queryView) return
  const current = queryView.state.doc.toString()
  if (current !== val) {
    queryView.dispatch({ changes: { from: 0, to: queryView.state.doc.length, insert: val || '' } })
  }
})

watch(() => props.request.graphqlVariables, (val) => {
  if (ignoreVarsUpdate) { ignoreVarsUpdate = false; return }
  if (!variablesView) return
  const current = variablesView.state.doc.toString()
  if (current !== val) {
    variablesView.dispatch({ changes: { from: 0, to: variablesView.state.doc.length, insert: val || '' } })
  }
})

watch(variablesOpen, (open) => {
  if (open) {
    nextTick(() => createVariablesEditor())
  }
})

watch(() => props.schema, (newSchema) => {
  if (!queryView) return
  gqlJsSchema = newSchema ? buildGqlJsSchema(newSchema) : null
  updateSchema(queryView, gqlJsSchema ?? undefined)
})

// React to settings changes without recreating either editor.
watch(() => settings.editorWordWrap, (wrap) => {
  const ext = wrap ? EditorView.lineWrapping : []
  queryView?.dispatch({ effects: queryWrapCompartment.reconfigure(ext) })
  variablesView?.dispatch({ effects: varsWrapCompartment.reconfigure(ext) })
})
watch(() => settings.effectiveTheme, () => {
  queryView?.dispatch({ effects: queryHighlightCompartment.reconfigure(activeHighlight()) })
  variablesView?.dispatch({ effects: varsHighlightCompartment.reconfigure(activeHighlight()) })
})
watch(() => settings.editorFontSize, () => {
  queryView?.requestMeasure()
  variablesView?.requestMeasure()
})

onMounted(() => {
  createQueryEditor()
  createVariablesEditor()
})

onUnmounted(() => {
  if (queryView) { queryView.destroy(); queryView = null }
  if (variablesView) { variablesView.destroy(); variablesView = null }
})
</script>

<template>
  <div class="flex h-full overflow-hidden">
    <div class="flex-1 flex flex-col min-w-0 overflow-hidden">
      <div class="flex-1 min-h-0 flex flex-col overflow-hidden">
        <div class="flex items-center h-7 px-3 border-b border-border shrink-0">
          <span class="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">Query</span>
        </div>
        <div class="flex-1 min-h-0 overflow-auto">
          <div ref="queryEditorRef" class="h-full overflow-auto outline-none" />
        </div>
      </div>

      <div class="shrink-0 border-t border-border" :class="variablesOpen ? 'h-[160px] flex flex-col' : ''">
        <button
          class="flex w-full items-center gap-2 h-7 px-3 hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer border-b border-border"
          :class="variablesOpen ? 'border-b border-border' : ''"
          @click="variablesOpen = !variablesOpen"
        >
          <span class="text-[10px] font-medium text-muted-foreground uppercase tracking-wider">Variables</span>
          <ChevronDown v-if="!variablesOpen" class="size-3 text-muted-foreground ml-auto" />
          <ChevronUp v-else class="size-3 text-muted-foreground ml-auto" />
        </button>
        <div v-if="variablesOpen" class="flex-1 min-h-0 overflow-auto">
          <div ref="variablesEditorRef" class="h-full overflow-auto outline-none" />
        </div>
      </div>
    </div>
  </div>
</template>
