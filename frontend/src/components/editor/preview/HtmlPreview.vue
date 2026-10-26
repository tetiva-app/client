<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useCopy } from '@/composables/useLocale'
import { htmlPreviewDoc } from '@/lib/response-preview'
import { EDITOR_COPY } from '../copy'

const props = defineProps<{ html: string }>()

const copy = useCopy(EDITOR_COPY)
const doc = computed(() => htmlPreviewDoc(props.html))

// A second load is a navigation the scrub missed (SVG animation, parser tricks).
const frameKey = ref(0)
let shown = false
watch(doc, () => { shown = false })

function onLoad() {
  if (!shown) {
    shown = true
    return
  }
  shown = false
  frameKey.value++
}
</script>

<template>
  <!-- Keep the sandbox empty: allow-scripts or allow-same-origin would hand the page the Wails runtime. -->
  <iframe
    :key="frameKey"
    sandbox=""
    :srcdoc="doc"
    :title="copy.response.previewFrame"
    class="block size-full border-0 bg-white [color-scheme:light]"
    data-testid="html-preview"
    @load="onLoad"
  />
</template>
