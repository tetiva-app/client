<script setup lang="ts">
import { computed } from 'vue'
import { Info } from 'lucide-vue-next'
import { useCopy } from '@/composables/useLocale'
import { fill } from '@/lib/locale'
import type { RequestRow } from '@/lib/compare/request'
import { changedFragments, type Fragment } from '@/lib/compare/fragments'
import { COMPARE_COPY } from './copy'
import SideMark from './SideMark.vue'

const props = defineProps<{
  rows: RequestRow[]
  labelA: string
  labelB: string
  note?: string
}>()

const copy = useCopy(COMPARE_COPY)

const CHANGED_CLASS = ['rounded-sm bg-[var(--gc-error)]/30', 'rounded-sm bg-[var(--gc-success)]/30']

const fragments = computed(() => {
  const out = new Map<string, [Fragment[], Fragment[]]>()
  for (const row of props.rows) {
    if (row.state !== 'changed' || row.a === null || row.b === null) continue
    const { a, b } = changedFragments(row.a, row.b)
    out.set(row.field, [a, b])
  }
  return out
})

function fieldLabel(row: RequestRow): string {
  const text = copy.value.request
  switch (row.kind) {
    case 'method': return text.method
    case 'url': return text.url
    case 'header': return fill(text.header, { name: row.name ?? '' })
    case 'query': return fill(text.query, { name: row.name ?? '' })
    case 'body': return text.body
    case 'variables': return text.variables
  }
}

function rowNote(row: RequestRow): string | null {
  if (row.state === 'notCompared') return copy.value.request.notCompared
  if (row.kind === 'variables') return copy.value.request.variablesNotKept
  return null
}
</script>

<template>
  <div class="min-h-0 flex-1 overflow-auto" data-testid="request-diff">
    <p v-if="note" class="flex items-center gap-2 px-4 pt-3 pb-1 text-xs text-muted-foreground">
      <Info class="size-3.5 shrink-0 text-primary" />
      {{ note }}
    </p>
    <table class="w-full table-fixed border-collapse text-[12.5px]">
      <thead>
        <tr class="border-b border-border text-left text-xs text-muted-foreground">
          <th class="w-8" />
          <th class="w-[190px] px-2.5 py-2 font-medium">{{ copy.request.field }}</th>
          <th class="px-2.5 py-2 font-medium">
            <span class="flex items-center gap-1.5 truncate"><SideMark side="A" />{{ labelA }}</span>
          </th>
          <th class="px-2.5 py-2 font-medium">
            <span class="flex items-center gap-1.5 truncate"><SideMark side="B" />{{ labelB }}</span>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in rows"
          :key="row.field"
          class="border-b border-border/60 align-top"
          :class="row.state === 'changed' ? 'bg-[var(--gc-warning)]/5' : 'text-muted-foreground'"
          :data-state="row.state"
        >
          <td class="py-1.5 text-center font-bold text-[var(--gc-warning)]">{{ row.state === 'changed' ? '~' : '' }}</td>
          <td class="truncate px-2.5 py-1.5 text-muted-foreground" :title="fieldLabel(row)">{{ fieldLabel(row) }}</td>
          <td v-if="rowNote(row)" colspan="2" class="px-2.5 py-1.5 text-muted-foreground">{{ rowNote(row) }}</td>
          <template v-else>
            <td v-for="(value, i) in [row.a, row.b]" :key="i" class="px-2.5 py-1.5 font-mono">
              <span v-if="value === null" class="font-sans text-muted-foreground">
                {{ row.state === 'unavailable' ? '—' : copy.request.none }}
              </span>
              <div v-else class="max-h-40 overflow-auto whitespace-pre-wrap break-all">
                <template v-if="fragments.has(row.field)"><span
                  v-for="(part, j) in fragments.get(row.field)![i]"
                  :key="j"
                  :class="part.changed ? CHANGED_CLASS[i] : undefined"
                >{{ part.text }}</span></template>
                <template v-else>{{ value }}</template>
              </div>
            </td>
          </template>
        </tr>
      </tbody>
    </table>
  </div>
</template>
