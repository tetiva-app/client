<script setup lang="ts">
import { ref, watch, defineAsyncComponent } from 'vue'
import HelpLink from '@/components/ui/HelpLink.vue'

const CodeEditor = defineAsyncComponent(() => import('./CodeEditor.vue'))

const props = defineProps<{
  entityId: string
  preScript: string
  postScript: string
  resolvedVariables?: Record<string, string>
  secretKeys?: Set<string>
}>()

const emit = defineEmits<{
  (e: 'update:preScript', value: string): void
  (e: 'update:postScript', value: string): void
}>()

const activePhase = ref<'pre' | 'post'>('pre')

const preContent = ref(props.preScript)
const postContent = ref(props.postScript)

watch(() => props.preScript, (val) => { preContent.value = val })
watch(() => props.postScript, (val) => { postContent.value = val })
</script>

<template>
  <div class="flex flex-col h-full">
    <div class="flex items-center gap-1 px-3 py-2 border-b border-border">
      <button
        class="px-3 py-1 text-xs font-medium rounded transition-colors cursor-pointer"
        :class="activePhase === 'pre'
          ? 'bg-primary/10 text-primary'
          : 'text-muted-foreground hover:text-foreground hover:bg-muted/30'"
        @click="activePhase = 'pre'"
      >
        Pre-request
      </button>
      <button
        class="px-3 py-1 text-xs font-medium rounded transition-colors cursor-pointer"
        :class="activePhase === 'post'
          ? 'bg-primary/10 text-primary'
          : 'text-muted-foreground hover:text-foreground hover:bg-muted/30'"
        @click="activePhase = 'post'"
      >
        Post-response
      </button>
      <span class="ml-auto text-[11px] text-muted-foreground/40">
        {{ activePhase === 'pre' ? 'Runs before each request' : 'Runs after each response' }}
      </span>
      <HelpLink slug="scripting" />
    </div>

    <!-- Editors — both mounted, toggled via v-show to preserve undo history -->
    <div class="flex-1 min-h-0 overflow-auto relative">
      <div v-show="activePhase === 'pre'" class="absolute inset-0">
        <CodeEditor
          :key="entityId + '-pre'"
          :content="preContent"
          language="javascript"
          :resolved-variables="resolvedVariables"
          :secret-keys="secretKeys"
          @update:content="(v) => emit('update:preScript', v)"
        />
      </div>
      <div v-show="activePhase === 'post'" class="absolute inset-0">
        <CodeEditor
          :key="entityId + '-post'"
          :content="postContent"
          language="javascript"
          :resolved-variables="resolvedVariables"
          :secret-keys="secretKeys"
          @update:content="(v) => emit('update:postScript', v)"
        />
      </div>
    </div>
  </div>
</template>
