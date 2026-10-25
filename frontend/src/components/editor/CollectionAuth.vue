<script setup lang="ts">
import { computed } from 'vue'
import { ShieldOff } from 'lucide-vue-next'
import AuthEditor from './AuthEditor.vue'
import AuthSelect from './auth/AuthSelect.vue'
import InheritedAuth from './InheritedAuth.vue'
import type { AuthType } from '@/types/request'
import { collectionAuthOptions } from '@/constants/auth'
import { switchAuthDraft } from '@/composables/useAuthDrafts'
import { useAuthTokenStore } from '@/stores/auth-tokens'
import { provideScreenLocale, useCopy, useLocale } from '@/composables/useLocale'
import { AUTH_COPY } from './auth/copy'

const props = defineProps<{
  collectionId: string
  authType: string
  authData: string
  version?: number
  nested?: boolean
}>()

const emit = defineEmits<{
  'update:authType': [value: string]
  'update:authData': [value: string]
}>()

provideScreenLocale()
const locale = useLocale()
const copy = useCopy(AUTH_COPY)

const authTypeOptions = computed(() => collectionAuthOptions(!!props.nested, locale.value))

const tokens = useAuthTokenStore()

// This selector is the collection's own; AuthEditor's cancel never sees it.
function selectType(value: AuthType) {
  if (value === props.authType) return
  if (props.authType === 'oauth2') {
    void tokens.cancelFlow({
      ownerKind: 'collection',
      ownerId: props.collectionId,
      authType: props.authType,
      authData: props.authData,
    })
  }
  const next = switchAuthDraft(props.collectionId, props.authType, props.authData, value)
  emit('update:authType', value)
  emit('update:authData', next)
}
</script>

<template>
  <div class="p-4 space-y-4">
    <AuthSelect
      class="max-w-[240px]"
      :label="copy.typeLabel"
      test-id="collection-auth-type-selector"
      :model-value="authType"
      :options="authTypeOptions"
      @update:model-value="selectType($event as AuthType)"
    />

    <AuthEditor
      v-if="authType !== 'none'"
      :auth-type="(authType as AuthType)"
      :auth-data="authData"
      owner-kind="collection"
      :owner-id="collectionId"
      :owner-version="version"
      hide-type-selector
      @update:auth-type="emit('update:authType', $event)"
      @update:auth-data="emit('update:authData', $event)"
    />

    <div v-if="authType === 'none' && nested" class="py-10 text-center">
      <p class="text-sm text-muted-foreground">{{ copy.collection.inherits }}</p>
      <InheritedAuth kind="collection" :id="collectionId">
        <template #none>{{ copy.collection.noParent }}</template>
      </InheritedAuth>
    </div>

    <div v-else-if="authType === 'none'" class="flex flex-col items-center justify-center py-16 text-center">
      <div class="rounded-lg bg-muted/20 p-4 mb-4 border border-border/50">
        <ShieldOff class="size-7 text-muted-foreground/60" />
      </div>
      <p class="text-sm font-medium text-muted-foreground">{{ copy.collection.noneTitle }}</p>
      <p class="text-xs text-muted-foreground/50 mt-1.5 max-w-[280px]">{{ copy.collection.noneText }}</p>
    </div>
  </div>
</template>
