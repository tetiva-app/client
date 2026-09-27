<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChevronRight, Download, Loader2, TriangleAlert } from 'lucide-vue-next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { importCounts, useImportFlow } from '@/composables/useImportFlow'

const flow = useImportFlow()
const ui = flow.ui
const destination = flow.destination

const showScripts = ref(false)

const preview = computed(() => ui.preview!)
const scriptCount = computed(() => preview.value.scripts.length)
const kind = computed(() => (preview.value.format === 'tetiva' ? 'Tetiva collection' : 'Postman collection'))
const canImport = computed(() => !ui.importing && !ui.expired && destination.value.workspaceName !== '')

function onOpenChange(open: boolean) {
  if (!open) flow.cancel()
}
</script>

<template>
  <Dialog :open="true" @update:open="onOpenChange">
    <DialogContent
      class="flex max-h-[calc(100vh-2rem)] w-[92vw] flex-col gap-0 border-border/50 bg-background p-0 sm:max-w-[560px]"
      data-testid="import-confirm"
    >
      <DialogHeader class="border-b border-border px-4 py-3">
        <DialogTitle class="flex items-center gap-2 text-sm font-medium">
          <Download class="size-4 text-primary" />
          Import “{{ preview.title || 'Untitled' }}”
        </DialogTitle>
        <DialogDescription class="text-xs text-muted-foreground">
          {{ kind }} · {{ importCounts(preview) }}
        </DialogDescription>
      </DialogHeader>

      <div class="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-3 text-[13px]">
        <section class="space-y-1">
          <h3 class="text-xs font-medium text-muted-foreground">Destination</h3>
          <p>
            Workspace “{{ destination.workspaceName }}”<template v-if="destination.folderName">, folder “{{ destination.folderName }}”</template>
          </p>
          <p v-if="destination.alwaysTopLevel" class="text-xs text-muted-foreground">
            Tetiva collections are always imported as a new top-level collection.
          </p>
          <p
            v-if="destination.inheritedAuth"
            class="flex items-center gap-1.5 text-xs text-[var(--gc-warning)]"
            data-testid="import-inherited-auth"
          >
            <TriangleAlert class="size-3.5 shrink-0" />
            Requests without their own authorization will use {{ destination.inheritedAuth }}'s authorization.
          </p>
          <p v-if="destination.cloud" class="flex items-center gap-1.5 text-xs text-[var(--gc-warning)]" data-testid="import-cloud-note">
            <TriangleAlert class="size-3.5 shrink-0" />
            This collection will be shared with everyone in {{ destination.workspaceName }}.
          </p>
        </section>

        <section v-if="preview.environmentName" class="space-y-1">
          <h3 class="text-xs font-medium text-muted-foreground">Environment</h3>
          <p>Adds the environment “{{ preview.environmentName }}”. Secret values stay empty.</p>
        </section>

        <section v-if="preview.hosts.length > 0" class="space-y-1">
          <h3 class="text-xs font-medium text-muted-foreground">Sends requests to</h3>
          <ul class="max-h-28 space-y-0.5 overflow-y-auto font-mono text-xs" data-testid="import-hosts">
            <li v-for="host in preview.hosts" :key="host" class="break-all">{{ host }}</li>
          </ul>
        </section>

        <section v-if="preview.warnings.length > 0" class="space-y-1">
          <h3 class="text-xs font-medium text-muted-foreground">Changed on import · {{ preview.warnings.length }}</h3>
          <ul class="max-h-28 list-disc space-y-0.5 overflow-y-auto pl-4 text-xs text-muted-foreground">
            <li v-for="(warning, i) in preview.warnings" :key="i">{{ warning }}</li>
          </ul>
        </section>

        <section v-if="scriptCount > 0" class="space-y-2">
          <h3 class="text-xs font-medium text-muted-foreground">Scripts</h3>
          <button
            type="button"
            class="flex items-center gap-1 cursor-pointer hover:text-foreground"
            :aria-expanded="showScripts"
            @click="showScripts = !showScripts"
          >
            <ChevronRight class="size-3.5 transition-transform" :class="{ 'rotate-90': showScripts }" />
            This collection contains {{ scriptCount === 1 ? '1 script' : `${scriptCount} scripts` }}
          </button>
          <div v-if="showScripts" class="max-h-64 space-y-2 overflow-y-auto" data-testid="import-scripts-list">
            <div v-for="(script, i) in preview.scripts" :key="i" class="rounded-md border border-border">
              <div class="border-b border-border px-2 py-1 text-xs text-muted-foreground">
                {{ script.path }} · {{ script.phase === 'pre' ? 'Pre-request' : 'Post-response' }}
              </div>
              <pre class="max-h-40 overflow-auto px-2 py-1.5 font-mono text-xs whitespace-pre-wrap break-all">{{ script.text }}</pre>
            </div>
          </div>
          <label class="flex items-start gap-2 cursor-pointer">
            <input
              v-model="ui.includeScripts"
              type="checkbox"
              class="mt-0.5 cursor-pointer rounded border-border"
              data-testid="import-scripts"
              aria-describedby="import-scripts-hint"
            />
            <span>
              Import scripts
              <span id="import-scripts-hint" class="block text-xs text-muted-foreground">
                Scripts run in a sandbox when you send requests and can change your environment variables
              </span>
            </span>
          </label>
        </section>

        <p v-if="ui.notice" class="text-xs text-muted-foreground">{{ ui.notice }}</p>
      </div>

      <DialogFooter class="flex-col gap-2 border-t border-border px-4 py-3 sm:flex-row sm:items-center">
        <p class="min-w-0 flex-1 text-xs text-destructive" data-testid="import-error">{{ ui.confirmError }}</p>
        <Button variant="outline" size="sm" :disabled="ui.importing" @click="flow.cancel()">Cancel</Button>
        <Button size="sm" :disabled="!canImport" data-testid="import-submit" @click="flow.confirm()">
          <Loader2 v-if="ui.importing" class="size-3.5 animate-spin" />
          Import
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
