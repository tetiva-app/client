<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useCollectionStore } from '@/stores/collections'
import { useRequestStore } from '@/stores/tabs'
import { useEnvironmentStore } from '@/stores/environments'
import CollectionOverview from './CollectionOverview.vue'
import CollectionAuth from './CollectionAuth.vue'
import ScriptEditor from './ScriptEditor.vue'

const props = defineProps<{
  collectionId: string
}>()

const collectionStore = useCollectionStore()
const tabStore = useRequestStore()
const envStore = useEnvironmentStore()

const secretKeys = computed(() => {
  const active = envStore.activeEnvironment
  if (!active) return new Set<string>()
  const vars = envStore.getVariables(active.id)
  return new Set(vars.filter(v => v.isSecret).map(v => v.key))
})

const collection = computed(() =>
  collectionStore.collectionsMap.get(props.collectionId)
)

const initialSection = tabStore.consumeInitialSection(props.collectionId)
const activeSection = ref<string>(initialSection ?? 'overview')

const localPreScript = ref(collection.value?.preScript ?? '')
const localPostScript = ref(collection.value?.postScript ?? '')
const localDescription = ref(collection.value?.description ?? '')
const localAuthType = ref(collection.value?.authType ?? 'none')
const localAuthData = ref(collection.value?.authData ?? '{}')

const isDirty = computed(() => {
  if (!collection.value) return false
  return localPreScript.value !== collection.value.preScript
    || localPostScript.value !== collection.value.postScript
    || localDescription.value !== collection.value.description
    || localAuthType.value !== collection.value.authType
    || localAuthData.value !== collection.value.authData
})

async function saveCollection() {
  if (!collection.value || !isDirty.value) return
  await collectionStore.edit(
    props.collectionId,
    {
      preScript: localPreScript.value,
      postScript: localPostScript.value,
      description: localDescription.value,
      authType: localAuthType.value,
      authData: localAuthData.value,
    },
    collection.value.version,
  )
}

const hasParent = computed(() => !!collection.value?.parentId)

onMounted(() => {
  tabStore.registerCollectionEditor(props.collectionId, {
    saveScripts: saveCollection,
    get scriptsDirty() { return isDirty.value },
  })
})

onUnmounted(() => {
  tabStore.unregisterCollectionEditor(props.collectionId)
})

function handleKeydown(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key === 's') {
    event.preventDefault()
    saveCollection()
  }
}
</script>

<template>
  <div
    v-if="collection"
    class="flex flex-col h-full"
    tabindex="0"
    @keydown="handleKeydown"
  >
    <Tabs v-model="activeSection" class="flex flex-col h-full">
      <TabsList class="w-full justify-start rounded-none border-b border-border bg-transparent px-2 h-9 shrink-0">
        <TabsTrigger value="overview" class="text-xs data-[state=active]:shadow-none rounded-none border-b-2 border-transparent data-[state=active]:border-primary">
          Overview
        </TabsTrigger>
        <TabsTrigger value="authorization" class="text-xs data-[state=active]:shadow-none rounded-none border-b-2 border-transparent data-[state=active]:border-primary">
          Authorization
        </TabsTrigger>
        <TabsTrigger value="scripts" class="text-xs data-[state=active]:shadow-none rounded-none border-b-2 border-transparent data-[state=active]:border-primary">
          Scripts
        </TabsTrigger>
      </TabsList>

      <TabsContent value="overview" class="flex-1 min-h-0 overflow-auto mt-0">
        <CollectionOverview
          :collection-id="collectionId"
          :description="localDescription"
          @update:description="localDescription = $event"
        />
      </TabsContent>

      <TabsContent value="authorization" class="flex-1 min-h-0 overflow-auto mt-0">
        <CollectionAuth
          :collection-id="collectionId"
          :auth-type="localAuthType"
          :auth-data="localAuthData"
          :has-parent="hasParent"
          @update:auth-type="localAuthType = $event"
          @update:auth-data="localAuthData = $event"
        />
      </TabsContent>

      <TabsContent value="scripts" class="flex-1 overflow-hidden mt-0">
        <ScriptEditor
          :entity-id="collectionId"
          :pre-script="localPreScript"
          :post-script="localPostScript"
          :resolved-variables="envStore.resolvedVariables"
          :secret-keys="secretKeys"
          @update:pre-script="localPreScript = $event"
          @update:post-script="localPostScript = $event"
        />
      </TabsContent>
    </Tabs>
  </div>
</template>
