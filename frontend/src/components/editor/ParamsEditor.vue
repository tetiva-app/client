<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import KeyValueEditor, { type KeyValueRow } from './KeyValueEditor.vue'
import { useCopy } from '@/composables/useLocale'
import { EDITOR_COPY } from './copy'
import type { PathVariable } from '@/types/request'
import { isPathVariableName, renamePathVariable, syncPathVariables } from '@/lib/path-variables'

const props = defineProps<{
  url: string
  pathVariables: PathVariable[] | null
}>()

const emit = defineEmits<{
  (e: 'update:url', value: string): void
  (e: 'update:pathVariables', value: PathVariable[]): void
}>()

const copy = useCopy(EDITOR_COPY)

const rows = ref<KeyValueRow[]>([])

// Guard against infinite update loops
let lastEmittedUrl = ''
let rowIdCounter = 0

function nextId(): string {
  return `param-${++rowIdCounter}`
}

function parseParams(url: string): KeyValueRow[] {
  try {
    const normalized = url.includes('://') ? url : `http://${url}`
    const parsed = new URL(normalized)
    const result: KeyValueRow[] = []
    parsed.searchParams.forEach((value, key) => {
      result.push({ id: nextId(), key, value, enabled: true })
    })
    return result
  } catch {
    return []
  }
}

function rebuildUrl() {
  const enabledParams = rows.value.filter(r => r.enabled && r.key.trim())
  const searchParams = new URLSearchParams()
  for (const param of enabledParams) {
    searchParams.append(param.key, param.value)
  }

  const queryString = searchParams.toString()
  let baseUrl = props.url

  const qIndex = baseUrl.indexOf('?')
  if (qIndex !== -1) {
    baseUrl = baseUrl.substring(0, qIndex)
  }

  const hashIndex = baseUrl.indexOf('#')
  let hash = ''
  if (hashIndex !== -1) {
    hash = baseUrl.substring(hashIndex)
    baseUrl = baseUrl.substring(0, hashIndex)
  }

  const newUrl = queryString ? `${baseUrl}?${queryString}${hash}` : `${baseUrl}${hash}`

  lastEmittedUrl = newUrl
  emit('update:url', newUrl)
}

watch(() => props.url, (newUrl) => {
  if (newUrl === lastEmittedUrl) return
  rows.value = parseParams(newUrl)
}, { immediate: true })

function onUpdate({ index, field, value }: { index: number; field: 'key' | 'value'; value: string }) {
  rows.value[index][field] = value
  rebuildUrl()
}

function onToggle(index: number) {
  rows.value[index].enabled = !rows.value[index].enabled
  rebuildUrl()
}

function onRemove(index: number) {
  rows.value.splice(index, 1)
  rebuildUrl()
}

function onAdd({ key, value }: { key: string; value: string }) {
  rows.value.push({ id: nextId(), key, value, enabled: true })
  rebuildUrl()
}

const pathVariables = computed(() => syncPathVariables(props.url, props.pathVariables))

// Ids follow the position, so renaming a key keeps the row and its focus.
const pathRows = computed<KeyValueRow[]>(() =>
  pathVariables.value.map((v, i) => ({ id: `path-${i}`, key: v.key, value: v.value, enabled: true })),
)

function onPathValue({ index, value }: { index: number; value: string }) {
  emit('update:pathVariables', pathVariables.value.map((v, i) => (i === index ? { ...v, value } : v)))
}

function onPathRename({ index, value }: { index: number; value: string }) {
  const from = pathVariables.value[index]
  const to = value.trim()
  if (!from || to === from.key || !isPathVariableName(to)) return
  const url = renamePathVariable(props.url, from.key, to)
  emit('update:url', url)
  emit('update:pathVariables', syncPathVariables(url, pathVariables.value.map((v, i) => (i === index ? { ...v, key: to } : v))))
}
</script>

<template>
  <div class="pb-3">
    <h3 class="mx-3 mt-3 text-xs font-medium text-muted-foreground">{{ copy.params.query }}</h3>
    <KeyValueEditor
      :rows="rows"
      key-placeholder="Param name"
      highlight-variables
      class="mt-1.5!"
      @update="onUpdate"
      @remove="onRemove"
      @add="onAdd"
      @toggle="onToggle"
    />
    <template v-if="pathRows.length > 0">
      <h3 class="mx-3 mt-4 text-xs font-medium text-muted-foreground" data-testid="path-variables-title">
        {{ copy.params.path }}
      </h3>
      <KeyValueEditor
        :rows="pathRows"
        :value-placeholder="copy.params.value"
        fixed-rows
        highlight-variables
        class="mt-1.5!"
        data-testid="path-variables"
        @update="onPathValue"
        @rename="onPathRename"
      />
    </template>
    <p v-else class="mx-3 mt-2 text-xs text-muted-foreground">{{ copy.params.pathHint }}</p>
  </div>
</template>
