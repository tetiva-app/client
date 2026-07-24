<script setup lang="ts">
import { ref, onMounted, defineAsyncComponent } from 'vue'
import { getWindowService } from '@/services'
import type { GraphQLSchema } from '@/types/graphql'
import GRPCSchemaViewer from '@/components/editor/grpc/GRPCSchemaViewer.vue'

const GraphQLSchemaViewer = defineAsyncComponent(() => import('@/components/editor/graphql/GraphQLSchemaViewer.vue'))

const props = defineProps<{
  schemaId: string
}>()

const definition = ref('')
const source = ref('')
const language = ref('')
const graphqlSchema = ref<GraphQLSchema | null>(null)
const graphqlSelectedOp = ref('')
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    const windowService = await getWindowService()
    if (!windowService) {
      error.value = 'Window service not available'
      return
    }
    const result = await windowService.getSchemaContent(props.schemaId)
    if (result.error) {
      error.value = result.error.message
      return
    }
    definition.value = result.data.definition
    source.value = result.data.source
    language.value = result.data.language || 'protobuf'

    if (language.value === 'graphql' && result.data.schemaJSON) {
      try {
        const parsed = JSON.parse(result.data.schemaJSON)
        graphqlSchema.value = parsed.schema ?? parsed
        graphqlSelectedOp.value = parsed.selectedOperation ?? ''
      } catch {
        console.error('Failed to parse GraphQL schema JSON')
      }
    }
  } catch (err) {
    error.value = String(err)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="h-screen bg-background text-foreground overflow-hidden">
    <div v-if="loading" class="flex items-center justify-center h-full">
      <span class="text-sm text-muted-foreground">Loading schema...</span>
    </div>
    <div v-else-if="error" class="flex items-center justify-center h-full">
      <span class="text-sm text-destructive">{{ error }}</span>
    </div>
    <GraphQLSchemaViewer
      v-else-if="language === 'graphql' && graphqlSchema"
      :schema="graphqlSchema"
      :selected-operation="graphqlSelectedOp"
      hide-detach
    />
    <GRPCSchemaViewer
      v-else
      :definition="definition"
      :source="source"
      hide-detach
    />
  </div>
</template>
