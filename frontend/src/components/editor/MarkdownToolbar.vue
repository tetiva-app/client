<script setup lang="ts">
import type { Component } from 'vue'
import { ref, watch } from 'vue'
import {
  Bold, ChevronDown, Code, Heading, Italic, Link2, List, ListOrdered, SquareCode, Table, TextQuote,
} from 'lucide-vue-next'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import type { MarkdownAction } from '@/lib/markdown-actions'
import { TABLE_COMMANDS } from '@/lib/markdown-table-editor'
import type { TableCommand } from '@/lib/markdown-table-editor'
import TableSizePicker from './TableSizePicker.vue'
import { isMac } from '@/lib/platform'

const props = withDefaults(defineProps<{
  inTable?: boolean
  insideFence?: boolean
  disabledCommands?: TableCommand[]
}>(), { inTable: false, insideFence: false, disabledCommands: () => [] })

const emit = defineEmits<{
  action: [MarkdownAction]
  tableCommand: [TableCommand]
  insertTable: [columns: number, rows: number]
  closed: []
}>()

const mod = isMac() ? '⌘' : 'Ctrl+'

interface ToolbarButton {
  action: MarkdownAction
  icon: Component
  label: string
  hint?: string
}

const groups: ToolbarButton[][] = [
  [
    { action: 'bold', icon: Bold, label: 'Bold', hint: `${mod}B` },
    { action: 'italic', icon: Italic, label: 'Italic', hint: `${mod}I` },
    { action: 'code', icon: Code, label: 'Inline code' },
    { action: 'link', icon: Link2, label: 'Link', hint: `${mod}K` },
  ],
  [
    { action: 'heading', icon: Heading, label: 'Heading' },
    { action: 'bulletList', icon: List, label: 'Bullet list' },
    { action: 'numberedList', icon: ListOrdered, label: 'Numbered list' },
    { action: 'quote', icon: TextQuote, label: 'Quote' },
  ],
  [
    { action: 'codeBlock', icon: SquareCode, label: 'Code block' },
  ],
]

const buttonClass = 'flex items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-40'

const tableGroups = [...new Set(TABLE_COMMANDS.map(item => item.group))]
  .map(group => TABLE_COMMANDS.filter(item => item.group === group))

const pickerOpen = ref(false)
const menuOpen = ref(false)

// Reka's own refocus is prevented below, so the editor is told to take focus back.
watch(pickerOpen, (open) => { if (!open) emit('closed') })
watch(menuOpen, (open) => { if (!open) emit('closed') })

function keepEditorFocus(event: Event) {
  event.preventDefault()
}

function isDisabled(command: TableCommand): boolean {
  return props.disabledCommands.includes(command)
}

function isActionDisabled(action: MarkdownAction): boolean {
  return action === 'codeBlock' && props.insideFence
}

function onPickerSelect(columns: number, rows: number) {
  pickerOpen.value = false
  emit('insertTable', columns, rows)
}

defineExpose({
  openTablePicker() {
    pickerOpen.value = true
  },
})
</script>

<template>
  <!-- Sticky: long descriptions grow the panel, and the toolbar has to stay reachable. -->
  <div class="sticky top-0 z-10 flex flex-wrap items-center gap-0.5 rounded-t-md border-b border-border bg-background px-1 py-1">
    <template v-for="(group, index) in groups" :key="index">
      <span v-if="index > 0" class="mx-1 h-4 w-px bg-border" aria-hidden="true" />
      <button
        v-for="button in group"
        :key="button.action"
        type="button"
        class="size-7"
        :class="buttonClass"
        :disabled="isActionDisabled(button.action)"
        :title="button.hint ? `${button.label} (${button.hint})` : button.label"
        :aria-label="button.label"
        @mousedown.prevent
        @click="emit('action', button.action)"
      >
        <component :is="button.icon" class="size-3.5" />
      </button>
    </template>

    <!-- Right edge: both popups open there, so they hang off the table instead of
         covering the row the command is about to change. -->
    <div class="ml-auto flex items-center gap-1">
      <span class="hidden text-[10px] text-muted-foreground/60 sm:inline">
        Type / for commands
      </span>

      <Popover v-if="!inTable" v-model:open="pickerOpen">
        <PopoverTrigger as-child>
          <button
            type="button"
            class="size-7"
            :class="buttonClass"
            title="Insert table"
            aria-label="Insert table"
            @mousedown.prevent
          >
            <Table class="size-3.5" />
          </button>
        </PopoverTrigger>
        <PopoverContent
          class="w-auto p-2"
          align="end"
          @open-auto-focus="keepEditorFocus"
          @close-auto-focus="keepEditorFocus"
        >
          <TableSizePicker @select="onPickerSelect" />
        </PopoverContent>
      </Popover>

      <DropdownMenu v-else v-model:open="menuOpen">
        <DropdownMenuTrigger as-child>
          <button
            type="button"
            class="h-7 gap-0.5 px-1"
            :class="buttonClass"
            title="Table actions"
            aria-label="Table actions"
            @mousedown.prevent
          >
            <Table class="size-3.5" />
            <ChevronDown class="size-3" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="min-w-44" @close-auto-focus="keepEditorFocus">
          <template v-for="(commands, index) in tableGroups" :key="index">
            <DropdownMenuSeparator v-if="index > 0" />
            <DropdownMenuItem
              v-for="item in commands"
              :key="item.command"
              class="py-1 text-xs"
              :disabled="isDisabled(item.command)"
              @select="emit('tableCommand', item.command)"
            >
              {{ item.label }}
            </DropdownMenuItem>
          </template>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  </div>
</template>
