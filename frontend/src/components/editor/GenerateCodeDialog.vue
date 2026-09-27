<script setup lang="ts">
import { computed, watch } from 'vue'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import MethodBadge from '@/components/ui/MethodBadge.vue'
import { useCodeDialogUi } from '@/stores/codeDialog'
import { useRequestStore } from '@/stores/tabs'
import CodeSnippetPanel from './CodeSnippetPanel.vue'

const ui = useCodeDialogUi()
const requests = useRequestStore()

const request = computed(() => (ui.requestId ? requests.getById(ui.requestId) : undefined))
const badged = computed(() => request.value?.protocol === 'grpc' || request.value?.protocol === 'websocket')

watch(request, (r) => { if (!r) ui.close() }, { immediate: true })

function onOpenChange(open: boolean) {
  if (!open) ui.close()
}
</script>

<template>
  <Dialog :open="true" @update:open="onOpenChange">
    <DialogContent class="flex h-[min(520px,calc(100vh-2rem))] w-[92vw] flex-col gap-0 p-0 sm:max-w-[800px]">
      <DialogHeader class="min-w-0 flex-row items-center gap-2.5 py-3 pr-12 pl-4">
        <DialogTitle class="shrink-0 text-sm font-semibold">Generate code</DialogTitle>
        <DialogDescription v-if="request" class="flex min-w-0 items-center gap-1.5 text-xs">
          <MethodBadge v-if="badged" :method="request.method" :protocol="request.protocol" />
          <span class="truncate" :title="request.name">
            <template v-if="!badged">{{ request.method }} · </template>{{ request.name }}
          </span>
        </DialogDescription>
      </DialogHeader>
      <CodeSnippetPanel v-if="request" :request="request" class="flex-1" />
    </DialogContent>
  </Dialog>
</template>
