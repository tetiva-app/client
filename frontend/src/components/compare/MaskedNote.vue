<script setup lang="ts">
import { computed } from 'vue'
import { useCopy } from '@/composables/useLocale'
import { COMPARE_COPY } from './copy'

const props = defineProps<{
  keys: readonly string[]
  headers: readonly string[]
}>()

const copy = useCopy(COMPARE_COPY)

const text = computed(() => {
  const [before, after = ''] = copy.value.masked.split('{list}')
  return { before, after }
})
const items = computed(() => [
  ...props.keys.map((name) => ({ name, code: true })),
  ...props.headers.map((name) => ({ name, code: false })),
])
</script>

<template>
  <span v-if="items.length" class="ml-auto truncate pl-3 text-xs text-muted-foreground">
    {{ text.before }}<template v-for="(item, i) in items" :key="item.name"><template v-if="i > 0">, </template><code v-if="item.code" class="font-mono">{{ item.name }}</code><template v-else>{{ item.name }}</template></template>{{ text.after }}
  </span>
</template>
