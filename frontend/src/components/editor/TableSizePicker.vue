<script setup lang="ts">
import { nextTick, onMounted, ref, useId } from 'vue'

const props = withDefaults(defineProps<{
  maxColumns?: number
  maxRows?: number
}>(), { maxColumns: 8, maxRows: 6 })

const emit = defineEmits<{
  select: [columns: number, rows: number]
}>()

const uid = useId()
const gridRef = ref<HTMLDivElement>()
const columns = ref(Math.min(3, props.maxColumns))
const rows = ref(Math.min(2, props.maxRows))

const STEPS: Record<string, [number, number]> = {
  ArrowLeft: [-1, 0],
  ArrowRight: [1, 0],
  ArrowUp: [0, -1],
  ArrowDown: [0, 1],
}

function cellId(column: number, row: number): string {
  return `${uid}-cell-${column}-${row}`
}

function highlight(column: number, row: number) {
  columns.value = Math.min(Math.max(column, 1), props.maxColumns)
  rows.value = Math.min(Math.max(row, 1), props.maxRows)
}

function onKeydown(event: KeyboardEvent) {
  const step = STEPS[event.key]
  if (step) {
    event.preventDefault()
    highlight(columns.value + step[0], rows.value + step[1])
    return
  }
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    emit('select', columns.value, rows.value)
  }
}

// The popover keeps its focus scope from grabbing a cell, so the grid takes focus itself.
onMounted(() => {
  nextTick(() => gridRef.value?.focus())
})
</script>

<template>
  <div class="flex flex-col gap-1.5">
    <div
      ref="gridRef"
      role="grid"
      aria-label="Table size"
      tabindex="0"
      :aria-colcount="maxColumns"
      :aria-rowcount="maxRows"
      :aria-activedescendant="cellId(columns, rows)"
      class="flex flex-col gap-1 rounded outline-none focus:ring-1 focus:ring-ring"
      @keydown="onKeydown"
    >
      <div v-for="row in maxRows" :key="row" role="row" class="flex gap-1">
        <button
          v-for="col in maxColumns"
          :id="cellId(col, row)"
          :key="col"
          type="button"
          role="gridcell"
          tabindex="-1"
          :aria-selected="col <= columns && row <= rows"
          class="size-5 rounded-[2px] border transition-colors"
          :class="col <= columns && row <= rows ? 'border-primary bg-primary/30' : 'border-muted-foreground'"
          :aria-label="`${col} × ${row} table`"
          @mouseenter="highlight(col, row)"
          @mousedown.prevent
          @click="emit('select', col, row)"
        />
      </div>
    </div>
    <p class="text-xs text-muted-foreground" aria-live="polite">{{ columns }} × {{ rows }}</p>
  </div>
</template>
