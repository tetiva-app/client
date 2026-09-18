<script setup lang="ts">
import { ref, reactive, computed, watch, onActivated, onBeforeUnmount, onDeactivated, onMounted, onUnmounted } from 'vue'
import { watchDebounced } from '@vueuse/core'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useCollectionStore, type StashedLocals } from '@/stores/collections'
import { useRequestStore, AUTOSAVE_DELAY_MS } from '@/stores/tabs'
import { useEnvironmentStore } from '@/stores/environments'
import { isInsideOverlay, isModShortcut } from '@/lib/shortcut-guards'
import { adoptStoreValue, descriptionSaveBlocked } from '@/lib/description'
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

const isActiveTab = computed(
  () => tabStore.activeTab?.type === 'collection' && tabStore.activeTab.collectionId === props.collectionId,
)

const initialSection = tabStore.consumeInitialSection(props.collectionId)
const activeSection = ref<string>(initialSection ?? 'overview')

// Sections stay mounted once visited: a remount would drop the editor's undo history.
const visited = reactive(new Set<string>())
watch(activeSection, section => { visited.add(section) }, { immediate: true })

// KeepAlive never re-runs setup, so a request to open an already-open tab on a
// given section arrives here instead.
onActivated(() => {
  const section = tabStore.consumeInitialSection(props.collectionId)
  if (section) activeSection.value = section
})

const stashed = collectionStore.takeLocals(props.collectionId, collection.value)
const localPreScript = ref(stashed?.preScript ?? collection.value?.preScript ?? '')
const localPostScript = ref(stashed?.postScript ?? collection.value?.postScript ?? '')
const localDescription = ref(stashed?.description ?? collection.value?.description ?? '')
const localAuthType = ref(stashed?.authType ?? collection.value?.authType ?? 'none')
const localAuthData = ref(stashed?.authData ?? collection.value?.authData ?? '{}')

watch(collection, (next, prev) => {
  if (!next) return
  localPreScript.value = adoptStoreValue(localPreScript.value, prev?.preScript, next.preScript)
  localPostScript.value = adoptStoreValue(localPostScript.value, prev?.postScript, next.postScript)
  localDescription.value = adoptStoreValue(localDescription.value, prev?.description, next.description)
  localAuthType.value = adoptStoreValue(localAuthType.value, prev?.authType, next.authType)
  localAuthData.value = adoptStoreValue(localAuthData.value, prev?.authData, next.authData)
})

const isDirty = computed(() => {
  if (!collection.value) return false
  return localPreScript.value !== collection.value.preScript
    || localPostScript.value !== collection.value.postScript
    || localDescription.value !== collection.value.description
    || localAuthType.value !== collection.value.authType
    || localAuthData.value !== collection.value.authData
})

let saving: Promise<boolean> | null = null
async function saveCollection(): Promise<boolean> {
  for (let round = 0; round < 3; round++) {
    if (!collection.value || !isDirty.value) return true
    if (!saving) {
      saving = collectionStore.edit(
        props.collectionId,
        {
          preScript: localPreScript.value,
          postScript: localPostScript.value,
          description: localDescription.value,
          authType: localAuthType.value,
          authData: localAuthData.value,
        },
        collection.value.version,
      ).finally(() => { saving = null })
    }
    if (!(await saving)) return false
  }
  return !isDirty.value
}

watchDebounced(localDescription, () => {
  if (isDirty.value && !saveBlocked()) void saveCollection()
}, { debounce: AUTOSAVE_DELAY_MS })

// On window, not on the container: clicking a button inside does not focus it in
// WebKit, so a container handler would miss Cmd+S right after any button press.
function handleKeydown(event: KeyboardEvent) {
  if (!isActiveTab.value) return
  if (isInsideOverlay(event)) return
  if (isModShortcut(event, 'KeyS', 's')) {
    event.preventDefault()
    void saveCollection()
  }
}

onMounted(() => {
  tabStore.registerCollectionEditor(props.collectionId, {
    saveScripts: saveCollection,
    get scriptsDirty() { return isDirty.value },
  })
  window.addEventListener('keydown', handleKeydown)
})

function saveBlocked() {
  return descriptionSaveBlocked(localDescription.value, collection.value?.description ?? '')
}

function stash(): StashedLocals {
  const parked: StashedLocals = {
    locals: {
      preScript: localPreScript.value,
      postScript: localPostScript.value,
      description: localDescription.value,
      authType: localAuthType.value,
      authData: localAuthData.value,
    },
    base: {
      preScript: collection.value?.preScript ?? '',
      postScript: collection.value?.postScript ?? '',
      description: collection.value?.description ?? '',
      authType: collection.value?.authType ?? 'none',
      authData: collection.value?.authData ?? '{}',
    },
  }
  collectionStore.stashLocals(props.collectionId, parked)
  return parked
}

onDeactivated(() => { if (!saveBlocked()) void saveCollection() })

// No deactivate hook when the History pane unmounts KeepAlive; park first, nothing waits for this.
onBeforeUnmount(async () => {
  const parked = stash()
  if (saveBlocked()) return
  if (await saveCollection()) collectionStore.clearLocals(props.collectionId, parked)
})

onUnmounted(() => {
  tabStore.unregisterCollectionEditor(props.collectionId)
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div v-if="collection" class="flex flex-col h-full">
    <Tabs v-model="activeSection" :unmount-on-hide="false" class="flex flex-col h-full">
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
          v-if="visited.has('overview')"
          :collection-id="collectionId"
          :description="localDescription"
          @update:description="localDescription = $event"
        />
      </TabsContent>

      <TabsContent value="authorization" class="flex-1 min-h-0 overflow-auto mt-0">
        <CollectionAuth
          v-if="visited.has('authorization')"
          :collection-id="collectionId"
          :auth-type="localAuthType"
          :auth-data="localAuthData"
          :version="collection.version"
          @update:auth-type="localAuthType = $event"
          @update:auth-data="localAuthData = $event"
        />
      </TabsContent>

      <TabsContent value="scripts" class="flex-1 overflow-hidden mt-0">
        <ScriptEditor
          v-if="visited.has('scripts')"
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
