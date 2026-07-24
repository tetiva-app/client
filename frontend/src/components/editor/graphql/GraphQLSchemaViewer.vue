<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { ExternalLink, ChevronRight, Home, Code, List, Search, ArrowUp, ArrowDown } from 'lucide-vue-next'
import { EditorView, drawSelection } from '@codemirror/view'
import { EditorState, Compartment } from '@codemirror/state'
import { search, SearchQuery, setSearchQuery, findNext as cmFindNext, findPrevious as cmFindPrev } from '@codemirror/search'
import { syntaxHighlighting } from '@codemirror/language'
import { BRAND_ACCENT_SELECTION } from '@/constants/defaults'
import { useSettingsStore } from '@/stores/settings'
import { darkHighlightStyle, lightHighlightStyle } from '@/lib/codemirror-highlight'
import { isWailsEnvironment, getWindowService } from '@/services'
import type { GraphQLSchema, GraphQLType } from '@/types/graphql'

const props = withDefaults(defineProps<{
  schema: GraphQLSchema | null
  selectedOperation: string
  hideDetach?: boolean
  navigateToType?: { name: string; key: number } | null
}>(), { hideDetach: false, navigateToType: null })

const viewMode = ref<'browse' | 'sdl'>('browse')

// Search is shared across both modes.
const searchQuery = ref('')

const navStack = ref<string[]>([])

const currentTypeName = computed(() => {
  if (navStack.value.length === 0) return null
  return navStack.value[navStack.value.length - 1]
})

const currentType = computed<GraphQLType | null>(() => {
  if (!currentTypeName.value || !props.schema) return null
  return props.schema.types.find(t => t.name === currentTypeName.value) ?? null
})

function goToRoot() {
  navStack.value = []
}

function goToType(name: string) {
  const isBuiltin = ['String', 'Boolean', 'Int', 'Float', 'ID'].includes(name) || name.startsWith('__')
  if (isBuiltin) return
  if (!props.schema?.types.find(t => t.name === name)) return
  if (navStack.value[navStack.value.length - 1] === name) return
  navStack.value = [...navStack.value, name]
}

function goBack(index: number) {
  navStack.value = navStack.value.slice(0, index + 1)
}

function baseTypeName(typeStr: string): string {
  return typeStr.replace(/[[\]!]/g, '')
}

function isNavigable(typeStr: string): boolean {
  const base = baseTypeName(typeStr)
  const builtins = ['String', 'Boolean', 'Int', 'Float', 'ID']
  if (builtins.includes(base) || base.startsWith('__')) return false
  return !!(props.schema?.types.find(t => t.name === base))
}

watch(() => props.selectedOperation, () => {
  navStack.value = []
}, { immediate: true })

// Navigate to type when triggered from autocomplete "Show in Docs"
watch(() => props.navigateToType, (nav) => {
  if (!nav || !nav.name || !props.schema) return
  if (props.schema.types.find(t => t.name === nav.name)) {
    navStack.value = [nav.name]
    if (viewMode.value !== 'browse') viewMode.value = 'browse'
  }
})

const activeOp = computed(() => {
  if (!props.selectedOperation || !props.schema) return null
  return (
    props.schema.queries.find(q => q.name === props.selectedOperation) ||
    props.schema.mutations.find(m => m.name === props.selectedOperation) ||
    null
  )
})

const activeOpKind = computed(() => {
  if (!activeOp.value || !props.schema) return null
  if (props.schema.queries.some(q => q.name === activeOp.value!.name)) return 'query'
  return 'mutation'
})

const rootQueries = computed(() => props.schema?.queries ?? [])
const rootMutations = computed(() => props.schema?.mutations ?? [])

function matchesSearch(text: string): boolean {
  if (!searchQuery.value) return true
  return text.toLowerCase().includes(searchQuery.value.toLowerCase())
}

const filteredQueries = computed(() => {
  if (!searchQuery.value || viewMode.value !== 'browse') return rootQueries.value
  return rootQueries.value.filter(op => matchesSearch(op.name) || matchesSearch(op.returnType || ''))
})

const filteredMutations = computed(() => {
  if (!searchQuery.value || viewMode.value !== 'browse') return rootMutations.value
  return rootMutations.value.filter(op => matchesSearch(op.name) || matchesSearch(op.returnType || ''))
})

const filteredFields = computed(() => {
  if (!currentType.value || !searchQuery.value || viewMode.value !== 'browse') return currentType.value?.fields ?? []
  return currentType.value.fields.filter(f => matchesSearch(f.name) || matchesSearch(f.type))
})

const editorRef = ref<HTMLDivElement>()
let view: EditorView | null = null

const settings = useSettingsStore()
const wrapCompartment = new Compartment()
const highlightCompartment = new Compartment()

function activeHighlight() {
  return syntaxHighlighting(settings.effectiveTheme === 'dark' ? darkHighlightStyle : lightHighlightStyle)
}

const sdlContent = computed(() => {
  if (!props.schema) return ''
  const parts: string[] = []
  for (const op of props.schema.queries) {
    if (op.definition) parts.push(op.definition)
  }
  for (const op of props.schema.mutations) {
    if (op.definition) parts.push(op.definition)
  }
  for (const type of props.schema.types) {
    if (type.definition) parts.push(type.definition)
  }
  return parts.join('\n\n')
})

function createEditor() {
  if (!editorRef.value) return
  if (view) { view.destroy(); view = null }

  view = new EditorView({
    state: EditorState.create({
      doc: sdlContent.value,
      extensions: [
        EditorView.editable.of(false),
        EditorState.readOnly.of(true),
        wrapCompartment.of(settings.editorWordWrap ? EditorView.lineWrapping : []),
        drawSelection(),
        search({ top: true, createPanel: () => ({ dom: document.createElement('span') }) }),
        highlightCompartment.of(activeHighlight()),
        EditorView.theme({
          '&': {
            fontSize: 'var(--gc-editor-font-size, 13px)',
            fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace',
            backgroundColor: 'transparent',
          },
          '.cm-gutters': { display: 'none' },
          '.cm-content': { padding: '12px', caretColor: 'transparent' },
          '&.cm-focused': { outline: 'none' },
          '.cm-cursor': { display: 'none' },
          '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
            backgroundColor: BRAND_ACCENT_SELECTION + ' !important',
          },
          '.cm-content ::selection': { backgroundColor: BRAND_ACCENT_SELECTION },
          '.cm-panels': { display: 'none' },
          '.cm-searchMatch': {
            backgroundColor: 'color-mix(in srgb, var(--primary) 30%, transparent)',
          },
          '.cm-searchMatch-selected': {
            backgroundColor: 'color-mix(in srgb, var(--primary) 50%, transparent)',
          },
        }),
      ],
    }),
    parent: editorRef.value,
  })
}

watch(sdlContent, (newContent) => {
  if (!view) return
  const current = view.state.doc.toString()
  if (current !== newContent) {
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: newContent } })
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

watch(viewMode, async (mode) => {
  searchQuery.value = ''
  if (mode === 'sdl') {
    await nextTick()
    createEditor()
  } else {
    if (view) { view.destroy(); view = null }
  }
})

function handleSdlSearch() {
  if (!view) return
  view.dispatch({
    effects: setSearchQuery.of(new SearchQuery({ search: searchQuery.value, caseSensitive: false, literal: true })),
  })
}

function sdlSearchNext() {
  if (view) cmFindNext(view)
}

function sdlSearchPrev() {
  if (view) cmFindPrev(view)
}

function selectAll() {
  if (!view) return
  view.focus()
  view.dispatch({ selection: { anchor: 0, head: view.state.doc.length } })
}

function handleEditorKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'a') {
    e.preventDefault()
    e.stopPropagation()
    selectAll()
  }
}

function handleSearchKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') {
    e.preventDefault()
    if (viewMode.value === 'sdl') {
      if (e.shiftKey) sdlSearchPrev()
      else sdlSearchNext()
    }
  }
  if (e.key === 'Escape') {
    searchQuery.value = ''
    if (viewMode.value === 'sdl') handleSdlSearch()
  }
}

const canDetach = computed(() => !props.hideDetach && isWailsEnvironment() && sdlContent.value.length > 0)

async function openInWindow() {
  const svc = await getWindowService()
  if (!svc) return
  const label = props.selectedOperation
    ? `Schema — ${props.selectedOperation}`
    : 'GraphQL Schema'
  const schemaJSON = props.schema ? JSON.stringify({ schema: props.schema, selectedOperation: props.selectedOperation }) : ''
  await svc.openSchemaViewer(sdlContent.value, 'graphql', label, 'graphql', schemaJSON)
}

onMounted(() => {
  if (viewMode.value === 'sdl') createEditor()
})
onUnmounted(() => { if (view) { view.destroy(); view = null } })
</script>

<template>
  <div class="flex flex-col h-full">
    <div class="flex items-center h-8 px-3 border-b border-border shrink-0 gap-2">
      <div class="flex items-center bg-muted/30 rounded-md p-0.5 gap-0.5">
        <button
          class="flex items-center gap-1 px-2 py-0.5 text-[10px] rounded transition-colors cursor-pointer"
          :class="viewMode === 'browse'
            ? 'bg-background text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'"
          @click="viewMode = 'browse'"
        >
          <List class="size-3" />
          Browse
        </button>
        <button
          class="flex items-center gap-1 px-2 py-0.5 text-[10px] rounded transition-colors cursor-pointer"
          :class="viewMode === 'sdl'
            ? 'bg-background text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'"
          @click="viewMode = 'sdl'"
        >
          <Code class="size-3" />
          SDL
        </button>
      </div>

      <div class="flex-1" />

      <div v-if="schema" class="flex items-center gap-0.5">
        <div class="relative">
          <Search class="absolute left-2 top-1/2 -translate-y-1/2 size-3 text-muted-foreground pointer-events-none" />
          <input
            v-model="searchQuery"
            class="h-6 w-32 pl-7 pr-2 text-[11px] bg-background border border-border rounded-md outline-none focus:ring-1 focus:ring-primary placeholder:text-muted-foreground"
            placeholder="Search..."
            @input="viewMode === 'sdl' && handleSdlSearch()"
            @keydown="handleSearchKeydown"
          />
        </div>
        <template v-if="searchQuery && viewMode === 'sdl'">
          <button
            class="size-5 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            title="Previous match (Shift+Enter)"
            @click="sdlSearchPrev"
          >
            <ArrowUp class="size-3" />
          </button>
          <button
            class="size-5 flex items-center justify-center text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            title="Next match (Enter)"
            @click="sdlSearchNext"
          >
            <ArrowDown class="size-3" />
          </button>
        </template>
      </div>

      <button
        v-if="canDetach"
        class="flex items-center gap-1 px-1.5 py-0.5 text-[10px] text-muted-foreground hover:text-foreground rounded transition-colors cursor-pointer"
        title="Open in Window"
        @click="openInWindow"
      >
        <ExternalLink class="size-3" />
        <span>Open in Window</span>
      </button>
    </div>

    <div v-if="!schema" class="flex-1 flex items-center justify-center">
      <p class="text-sm text-muted-foreground">Load schema to browse documentation</p>
    </div>

    <template v-else-if="viewMode === 'browse'">
      <div v-if="navStack.length > 0" class="flex items-center gap-1 px-3 py-1 border-b border-border/50 flex-wrap min-h-[24px] shrink-0">
        <button
          class="flex items-center gap-0.5 text-[10px] text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
          @click="goToRoot"
        >
          <Home class="size-2.5" />
          Root
        </button>
        <template v-for="(name, i) in navStack" :key="name + i">
          <ChevronRight class="size-2.5 text-muted-foreground/50 shrink-0" />
          <button
            class="text-[10px] text-muted-foreground hover:text-foreground transition-colors cursor-pointer font-mono truncate max-w-[120px]"
            :class="{ 'text-foreground font-medium': i === navStack.length - 1 }"
            :title="name"
            @click="i < navStack.length - 1 && goBack(i)"
          >
            {{ name }}
          </button>
        </template>
      </div>

      <div class="flex-1 overflow-y-auto text-xs">
        <template v-if="navStack.length === 0 && activeOp">
          <div class="flex items-center gap-2 px-3 py-2 border-b border-border/50">
            <span
              class="text-[9px] font-bold px-1 py-0.5 rounded uppercase"
              :class="activeOpKind === 'query' ? 'bg-green-500/10 text-green-400' : 'bg-orange-500/10 text-orange-400'"
            >{{ activeOpKind === 'query' ? 'Q' : 'M' }}</span>
            <span class="font-mono font-medium text-foreground">{{ activeOp.name }}</span>
          </div>

          <div v-if="activeOp.args.length > 0" class="py-1">
            <div class="px-3 py-1 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
              Arguments
            </div>
            <div
              v-for="arg in activeOp.args"
              :key="arg.name"
              class="px-3 py-1.5 border-b border-border/30"
            >
              <div class="flex items-baseline gap-1">
                <span class="font-mono text-foreground font-medium">{{ arg.name }}</span>
                <span class="text-muted-foreground/50">:</span>
                <span
                  :class="isNavigable(arg.type) ? 'font-mono text-[#A78BFA] hover:underline cursor-pointer' : 'font-mono text-muted-foreground'"
                  @click="isNavigable(arg.type) && goToType(baseTypeName(arg.type))"
                >{{ arg.type }}</span>
              </div>
            </div>
          </div>

          <div v-if="activeOp.returnType" class="py-1">
            <div class="px-3 py-1 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
              Returns
            </div>
            <button
              class="flex w-full items-center gap-1.5 px-3 py-1.5 hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer text-left"
              @click="isNavigable(activeOp.returnType) && goToType(baseTypeName(activeOp.returnType))"
            >
              <span
                :class="isNavigable(activeOp.returnType) ? 'font-mono text-[#A78BFA] hover:underline' : 'font-mono text-muted-foreground'"
              >{{ activeOp.returnType }}</span>
              <ChevronRight v-if="isNavigable(activeOp.returnType)" class="size-3 text-muted-foreground/50" />
            </button>
          </div>
        </template>

        <template v-else-if="navStack.length === 0">
          <div v-if="filteredQueries.length > 0" class="py-1">
            <div class="px-3 py-1 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
              Queries
            </div>
            <button
              v-for="op in filteredQueries"
              :key="'q-' + op.name"
              class="flex w-full items-start gap-2 px-3 py-1.5 hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer text-left"
              @click="goToType(op.returnType ? baseTypeName(op.returnType) : '')"
            >
              <span class="shrink-0 mt-0.5 text-[9px] font-bold px-1 py-0.5 rounded bg-green-500/10 text-green-400">Q</span>
              <div class="min-w-0">
                <span class="font-mono text-foreground">{{ op.name }}</span>
                <span v-if="op.args.length" class="text-muted-foreground/60 font-mono text-[10px] ml-0.5">({{ op.args.map(a => `${a.name}: ${a.type}`).join(', ') }})</span>
                <div v-if="op.returnType" class="text-[10px] mt-0.5">
                  <span class="text-muted-foreground/50">→ </span>
                  <span
                    :class="isNavigable(op.returnType) ? 'text-[#A78BFA] hover:underline cursor-pointer' : 'text-muted-foreground'"
                    @click.stop="isNavigable(op.returnType) && goToType(baseTypeName(op.returnType))"
                  >{{ op.returnType }}</span>
                </div>
              </div>
            </button>
          </div>

          <div v-if="filteredMutations.length > 0" class="py-1 border-t border-border/50">
            <div class="px-3 py-1 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
              Mutations
            </div>
            <button
              v-for="op in filteredMutations"
              :key="'m-' + op.name"
              class="flex w-full items-start gap-2 px-3 py-1.5 hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer text-left"
              @click="goToType(op.returnType ? baseTypeName(op.returnType) : '')"
            >
              <span class="shrink-0 mt-0.5 text-[9px] font-bold px-1 py-0.5 rounded bg-orange-500/10 text-orange-400">M</span>
              <div class="min-w-0">
                <span class="font-mono text-foreground">{{ op.name }}</span>
                <span v-if="op.args.length" class="text-muted-foreground/60 font-mono text-[10px] ml-0.5">({{ op.args.map(a => `${a.name}: ${a.type}`).join(', ') }})</span>
                <div v-if="op.returnType" class="text-[10px] mt-0.5">
                  <span class="text-muted-foreground/50">→ </span>
                  <span
                    :class="isNavigable(op.returnType) ? 'text-[#A78BFA] hover:underline cursor-pointer' : 'text-muted-foreground'"
                    @click.stop="isNavigable(op.returnType) && goToType(baseTypeName(op.returnType))"
                  >{{ op.returnType }}</span>
                </div>
              </div>
            </button>
          </div>

          <div v-if="filteredQueries.length === 0 && filteredMutations.length === 0" class="px-3 py-4 text-center text-muted-foreground">
            {{ searchQuery ? 'No matches found' : 'No operations found' }}
          </div>
        </template>

        <template v-else-if="currentType">
          <div class="flex items-center gap-2 px-3 py-2 border-b border-border/50">
            <span class="text-[9px] font-bold px-1 py-0.5 rounded bg-[#A78BFA]/10 text-[#A78BFA] uppercase">
              {{ currentType.kind }}
            </span>
            <span class="font-mono font-medium text-foreground">{{ currentType.name }}</span>
          </div>

          <div v-if="currentType.enumValues.length > 0" class="py-1">
            <div class="px-3 py-1 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
              Values
            </div>
            <div
              v-for="val in currentType.enumValues"
              :key="val"
              class="px-3 py-1 font-mono text-foreground"
            >
              {{ val }}
            </div>
          </div>

          <div v-if="currentType.possibleTypes.length > 0" class="py-1">
            <div class="px-3 py-1 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
              Possible Types
            </div>
            <div
              v-for="pt in currentType.possibleTypes"
              :key="pt"
              class="px-3 py-1"
            >
              <span
                :class="isNavigable(pt) ? 'font-mono text-[#A78BFA] hover:underline cursor-pointer' : 'font-mono text-muted-foreground'"
                @click="isNavigable(pt) && goToType(pt)"
              >{{ pt }}</span>
            </div>
          </div>

          <div v-if="filteredFields.length > 0" class="py-1">
            <div class="px-3 py-1 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
              Fields
            </div>
            <div
              v-for="field in filteredFields"
              :key="field.name"
              class="px-3 py-1.5 border-b border-border/30"
            >
              <div class="flex items-baseline gap-1">
                <span class="font-mono text-foreground font-medium">{{ field.name }}</span>
                <span class="text-muted-foreground/50">:</span>
                <span
                  :class="isNavigable(field.type) ? 'font-mono text-[#A78BFA] hover:underline cursor-pointer' : 'font-mono text-muted-foreground'"
                  @click="isNavigable(field.type) && goToType(baseTypeName(field.type))"
                >{{ field.type }}</span>
              </div>
              <div v-if="field.args.length" class="mt-0.5 text-[10px] text-muted-foreground/60 font-mono">
                ({{ field.args.map(a => `${a.name}: ${a.type}`).join(', ') }})
              </div>
            </div>
          </div>

          <div v-if="filteredFields.length === 0 && currentType.enumValues.length === 0 && currentType.possibleTypes.length === 0" class="px-3 py-4 text-center text-muted-foreground">
            {{ searchQuery ? 'No matches found' : 'No fields' }}
          </div>
        </template>

        <div v-else class="px-3 py-4 text-center text-muted-foreground">
          Type not found
        </div>
      </div>
    </template>

    <template v-else>
      <div v-if="!sdlContent" class="flex-1 flex items-center justify-center">
        <p class="text-sm text-muted-foreground">No schema definitions available</p>
      </div>

      <div v-else class="flex-1 overflow-auto">
        <div
          ref="editorRef"
          tabindex="0"
          class="h-full overflow-auto outline-none"
          @click="view?.focus()"
          @keydown="handleEditorKeydown"
        />
      </div>
    </template>

    <div v-if="schema" class="flex items-center h-7 px-3 border-t border-border text-[10px] text-muted-foreground/60 shrink-0">
      {{ schema.source === 'introspection' ? 'Source: Server introspection' : schema.source === 'schema_file' ? 'Source: Schema file' : 'Source: ' + schema.source }}
      <span v-if="selectedOperation" class="ml-2">• {{ selectedOperation }}</span>
    </div>
  </div>
</template>
