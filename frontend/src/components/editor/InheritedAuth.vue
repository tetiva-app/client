<script setup lang="ts">
import { computed, onActivated, onDeactivated, onMounted, ref, watch } from 'vue'
import type { AuthType } from '@/types/request'
import { AUTH_TYPE_LABELS } from '@/constants/auth'
import { useCollectionStore } from '@/stores/collections'
import { useRequestStore } from '@/stores/tabs'
import { tokenStatusLabel, useAuthTokenStore } from '@/stores/auth-tokens'
import { getAuthService } from '@/services'
import type { AuthOwnerKind, ResolvedOwner } from '@/services/auth-api'
import { useWindowEvents } from '@/composables/useWindowEvents'
import { chainSignature } from '@/lib/inherited-auth'

const props = defineProps<{
  kind: 'request' | 'collection'
  id: string
  protocol?: string
}>()

const HTTP_ONLY_TYPES: AuthType[] = ['digest', 'aws_sigv4']

const tokens = useAuthTokenStore()
const collections = useCollectionStore()
const tabs = useRequestStore()

type State = { status: 'loading' } | { status: 'error' } | { status: 'ready'; owner: ResolvedOwner | null }
const state = ref<State>({ status: 'loading' })

const owner = computed(() => (state.value.status === 'ready' ? state.value.owner : null))
const ownerType = computed(() => owner.value?.authType as AuthType | undefined)
const ownerLabel = computed(() => (ownerType.value ? AUTH_TYPE_LABELS[ownerType.value] ?? ownerType.value : ''))
const ownerName = computed(() => (owner.value ? collections.collectionsMap.get(owner.value.ownerId)?.name ?? '' : ''))
const httpOnlyBlocked = computed(() =>
  !!props.protocol && props.protocol !== 'http' && !!ownerType.value && HTTP_ONLY_TYPES.includes(ownerType.value),
)
const tokenStatus = computed(() => {
  const o = owner.value
  if (!o || o.authType !== 'oauth2') return ''
  return tokenStatusLabel(tokens.entry(o.ownerKind, o.ownerId))
})

const startCollectionId = computed(() =>
  props.kind === 'collection' ? props.id : tabs.getById(props.id)?.collectionId,
)
const signature = computed(() => chainSignature(collections.collectionsMap, startCollectionId.value))

let generation = 0

async function load() {
  const current = ++generation
  const service = await getAuthService()
  const result = props.kind === 'collection'
    ? await service.resolveCollectionOwner(props.id)
    : await service.resolveOwner(props.id)
  if (current !== generation) return
  if (result.error) {
    state.value = { status: 'error' }
    return
  }
  const found = result.data.ownerId ? result.data : null
  state.value = { status: 'ready', owner: found }
  if (found?.authType === 'oauth2') {
    await tokens.refresh({
      ownerKind: found.ownerKind as AuthOwnerKind,
      ownerId: found.ownerId,
      authType: found.authType,
      authData: found.authData,
    })
  }
}

let active = true

function reloadWhenVisible() {
  if (active) void load()
}

watch(() => [props.kind, props.id], () => {
  state.value = { status: 'loading' }
  void load()
})
watch(signature, reloadWhenVisible)
onMounted(() => { void load() })
// Under KeepAlive a hidden tab only waits: activation reloads anyway.
onActivated(() => {
  active = true
  void load()
})
onDeactivated(() => { active = false })
useWindowEvents({ onSyncChanged: reloadWhenVisible })

function openOwner() {
  const o = owner.value
  if (!o || !ownerName.value) return
  tabs.openCollectionTab(o.ownerId, ownerName.value, { initialSection: 'authorization' })
}
</script>

<template>
  <template v-if="owner">
    <p class="mt-2 text-xs text-muted-foreground" data-testid="inherited-auth">
      {{ ownerLabel }} configured on
      <button
        v-if="ownerName"
        class="text-primary hover:underline cursor-pointer"
        type="button"
        data-testid="inherited-auth-owner"
        @click="openOwner"
      >collection “{{ ownerName }}”</button>
      <template v-else>the parent collection</template>
    </p>
    <p v-if="httpOnlyBlocked" class="mt-2 text-[11px] text-amber-500" data-testid="auth-http-only">
      {{ ownerLabel }} is supported for HTTP requests only; this request will fail to send.
    </p>
    <p v-if="tokenStatus" class="mt-1 text-xs text-muted-foreground/70" data-testid="inherited-token-status">
      {{ tokenStatus }} — manage the token on that collection's Auth tab
    </p>
  </template>
  <p
    v-else-if="state.status === 'ready' && $slots.none"
    class="mt-2 text-xs text-muted-foreground"
    data-testid="inherited-auth-none"
  >
    <slot name="none" />
  </p>
  <p v-else-if="state.status === 'error'" class="mt-2 text-xs text-muted-foreground" data-testid="inherited-auth-error">
    Couldn't resolve inherited auth
  </p>
</template>
