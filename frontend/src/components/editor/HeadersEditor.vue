<script setup lang="ts">
import { computed } from 'vue'
import KeyValueEditor, { type KeyValueRow } from './KeyValueEditor.vue'
import type { HeaderItem } from '@/types/request'

const props = defineProps<{
  headers: HeaderItem[]
}>()

const emit = defineEmits<{
  (e: 'update:headers', value: HeaderItem[]): void
}>()

const rows = computed<KeyValueRow[]>(() =>
  props.headers.map((h, i) => ({
    id: `header-${i}-${h.key}`,
    key: h.key,
    value: h.value,
    enabled: h.enabled,
  }))
)

function onUpdate({ index, field, value }: { index: number; field: 'key' | 'value'; value: string }) {
  const updated = props.headers.map((h, i) =>
    i === index ? { ...h, [field]: value } : { ...h }
  )
  emit('update:headers', updated)
}

function onRemove(index: number) {
  emit('update:headers', props.headers.filter((_, i) => i !== index))
}

function onAdd({ key, value }: { key: string; value: string }) {
  emit('update:headers', [...props.headers, { key, value, enabled: true }])
}

function onToggle(index: number) {
  const updated = props.headers.map((h, i) =>
    i === index ? { ...h, enabled: !h.enabled } : { ...h }
  )
  emit('update:headers', updated)
}
</script>

<template>
  <KeyValueEditor
    :rows="rows"
    key-placeholder="Header name"
    highlight-variables
    @update="onUpdate"
    @remove="onRemove"
    @add="onAdd"
    @toggle="onToggle"
  />
</template>
