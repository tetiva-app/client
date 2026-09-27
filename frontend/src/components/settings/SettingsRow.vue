<script setup lang="ts">
// Search hides rows with v-show, so the divider depends on the previous visible row.
withDefaults(defineProps<{ label: string; description?: string; shown?: boolean }>(), { shown: true })
</script>

<template>
  <div
    v-show="shown"
    :data-hidden="shown ? undefined : ''"
    class="border-t border-border py-3 [&:not([data-row]:not([data-hidden])~&)]:border-t-0"
  >
    <div class="flex min-h-8 items-center justify-between gap-4">
      <div class="flex min-w-0 items-center gap-2 text-[13px] font-medium">
        <span class="min-w-0">{{ label }}</span>
        <slot name="badge" />
      </div>
      <div v-if="$slots.control" class="flex min-w-0 items-center justify-end gap-1.5">
        <slot name="control" />
      </div>
    </div>
    <div v-if="$slots.description || description" class="mt-0.5 text-xs text-muted-foreground">
      <slot name="description">{{ description }}</slot>
    </div>
    <div v-if="$slots.default" class="mt-2.5 flex min-w-0 flex-col gap-2">
      <slot />
    </div>
  </div>
</template>
