<script setup lang="ts">
import { ref, watch } from 'vue'
import KeyValueEditor, { type KeyValueRow } from './KeyValueEditor.vue'

const props = defineProps<{
  url: string
}>()

const emit = defineEmits<{
  (e: 'update:url', value: string): void
}>()

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
</script>

<template>
  <KeyValueEditor
    :rows="rows"
    key-placeholder="Param name"
    highlight-variables
    @update="onUpdate"
    @remove="onRemove"
    @add="onAdd"
    @toggle="onToggle"
  />
</template>
