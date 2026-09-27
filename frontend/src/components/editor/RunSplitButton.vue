<script setup lang="ts">
import { computed, onDeactivated, ref } from 'vue'
import { ChevronDown, Code, FileCode, Square, Terminal } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { copyMenuModel } from '@/lib/snippets/menu'
import type { SnippetTargetMeta } from '@/lib/snippets/targets'

const props = defineProps<{
  label: string
  loading?: boolean
  disabled?: boolean
  targets: SnippetTargetMeta[]
  toggleVariant?: 'default' | 'destructive'
}>()

const emit = defineEmits<{
  (e: 'run'): void
  (e: 'cancel'): void
  (e: 'copy', key: string): void
  (e: 'generate'): void
}>()

const menu = computed(() => copyMenuModel(props.targets))
const variant = computed(() => (props.loading ? 'destructive' : props.toggleVariant ?? 'default'))
const open = ref(false)

// Native Close Tab bypasses the overlay guard, and KeepAlive would leave the portaled menu over the next tab.
onDeactivated(() => {
  open.value = false
})

// The menu hands focus back to the chevron after its close animation, which would pull it out of the dialog.
let generating = false

function generate() {
  generating = true
  emit('generate')
}

function onCloseAutoFocus(event: Event) {
  if (generating) event.preventDefault()
  generating = false
}
</script>

<template>
  <div class="flex items-center">
    <Button
      v-if="loading"
      size="sm"
      variant="destructive"
      class="h-7 px-4 cursor-pointer rounded-r-none"
      @click="emit('cancel')"
    >
      <Square class="size-3 mr-1.5 fill-current" />
      Cancel
    </Button>
    <Button
      v-else
      size="sm"
      :variant="variant"
      class="h-7 px-5 cursor-pointer rounded-r-none"
      :disabled="disabled"
      @click="emit('run')"
    >
      {{ label }}
    </Button>
    <div class="w-px h-7 bg-primary-foreground/30" />
    <DropdownMenu v-model:open="open">
      <DropdownMenuTrigger as-child>
        <Button
          size="sm"
          :variant="variant"
          aria-label="More actions"
          class="h-7 w-7 px-0 cursor-pointer rounded-l-none"
        >
          <ChevronDown class="size-3" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" class="min-w-[180px]" @close-auto-focus="onCloseAutoFocus">
        <DropdownMenuItem
          v-for="target in menu.top"
          :key="target.key"
          class="text-xs"
          @select="emit('copy', target.key)"
        >
          <Terminal v-if="target.language === 'shell'" class="size-3.5" />
          <Code v-else class="size-3.5" />
          Copy as {{ target.label }}
        </DropdownMenuItem>
        <DropdownMenuSub v-if="menu.submenu.length">
          <DropdownMenuSubTrigger class="text-xs data-[state=open]:bg-muted-foreground/10">
            <Code class="size-3.5" />
            Copy as
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent class="min-w-[160px]">
            <DropdownMenuItem
              v-for="target in menu.submenu"
              :key="target.key"
              class="text-xs"
              @select="emit('copy', target.key)"
            >
              {{ target.label }}
            </DropdownMenuItem>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuSeparator />
        <DropdownMenuItem class="text-xs" @select="generate">
          <FileCode class="size-3.5" />
          Generate code…
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  </div>
</template>
