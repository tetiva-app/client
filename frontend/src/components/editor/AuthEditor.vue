<script setup lang="ts">
import { computed, defineAsyncComponent, onActivated, onMounted, ref, watch } from 'vue'
import type { AuthType } from '@/types/request'
import AuthSelect from './auth/AuthSelect.vue'
import AuthTextField from './auth/AuthTextField.vue'
import { useCollectionStore } from '@/stores/collections'
import { useRequestStore } from '@/stores/tabs'
import { AUTH_TYPE_LABELS, REQUEST_AUTH_OPTIONS } from '@/constants/auth'
import { authFormValues, defaultAuthData, setAuthField } from '@/lib/auth-data'
import { tokenStatusLabel, useAuthTokenStore } from '@/stores/auth-tokens'
import { getAuthService } from '@/services'
import type { AuthOwnerKind, ResolvedOwner } from '@/services/auth-api'

// The new forms carry CodeMirror (JWT claims) and a token panel; the tab opens
// on basic/bearer far more often than on those.
const OAuth2Form = defineAsyncComponent(() => import('./auth/OAuth2Form.vue'))
const JwtForm = defineAsyncComponent(() => import('./auth/JwtForm.vue'))
const DigestForm = defineAsyncComponent(() => import('./auth/DigestForm.vue'))
const AwsForm = defineAsyncComponent(() => import('./auth/AwsForm.vue'))

const props = defineProps<{
  authType: AuthType
  authData: string
  ownerKind: AuthOwnerKind
  ownerId: string
  ownerVersion?: number
  protocol?: string
  hideTypeSelector?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:authType', value: AuthType): void
  (e: 'update:authData', value: string): void
}>()

// Digest and AWS Signature are applied on the outgoing HTTP request, so the
// usecase refuses them on the other protocols (rejectNonHTTPAuth).
const HTTP_ONLY_TYPES: AuthType[] = ['digest', 'aws_sigv4']
const httpOnly = computed(() => !props.protocol || props.protocol === 'http')
const authTypes = computed(() => {
  if (httpOnly.value) return REQUEST_AUTH_OPTIONS
  const offered = REQUEST_AUTH_OPTIONS.filter(o => !HTTP_ONLY_TYPES.includes(o.value))
  // A request that already carries an HTTP-only type keeps naming it properly;
  // the amber note below says it cannot be sent.
  const current = REQUEST_AUTH_OPTIONS.find(o => o.value === props.authType)
  return current && !offered.includes(current) ? [...offered, current] : offered
})
const httpOnlyBlocked = computed(() => !httpOnly.value && HTTP_ONLY_TYPES.includes(props.authType))

const addToOptions: { value: string; label: string }[] = [
  { value: 'header', label: 'Header' },
  { value: 'query', label: 'Query Params' },
]

const values = computed(() => authFormValues(props.authType, props.authData))

// A type this build does not know still names itself rather than reading "None".
const currentAuthLabel = computed(() => AUTH_TYPE_LABELS[props.authType] || props.authType || 'None')

// A mis-click on the type selector must not wipe credentials: what each type
// held is kept for the session and handed back when the user returns to it.
const drafts = ref<Partial<Record<AuthType, string>>>({})

function selectAuthType(type: AuthType) {
  if (type === props.authType) return
  // The flow belongs to the OAuth block that started it, and the block is about
  // to go: cancel before the emit, not on unmount (a sub-tab switch unmounts too).
  if (props.authType === 'oauth2') {
    void tokens.cancelFlow({
      ownerKind: props.ownerKind,
      ownerId: props.ownerId,
      authType: props.authType,
      authData: props.authData,
    })
  }
  drafts.value = { ...drafts.value, [props.authType]: props.authData }
  emit('update:authType', type)
  emit('update:authData', drafts.value[type] ?? defaultAuthData(type))
}

function updateField(field: string, value: string) {
  emit('update:authData', setAuthField(props.authData, field, value))
}

// Inherit shows what the request will actually send, read-only; the owner also
// addresses the token, so an inherited OAuth 2.0 status is the collection's.
const tokens = useAuthTokenStore()
const collections = useCollectionStore()
const tabs = useRequestStore()
const inherited = ref<ResolvedOwner | null>(null)

const inheritedType = computed(() => inherited.value?.authType as AuthType | undefined)

const inheritedLabel = computed(() => {
  const type = inheritedType.value
  return type ? AUTH_TYPE_LABELS[type] ?? type : ''
})

// An inherited digest/aws_sigv4 fails on send just as a directly chosen one
// does, and the selector says nothing about it — the inherit block must.
const inheritedHttpOnlyBlocked = computed(() =>
  !httpOnly.value && props.authType === 'inherit' && !!inheritedType.value && HTTP_ONLY_TYPES.includes(inheritedType.value),
)

const inheritedOwnerName = computed(() => {
  const owner = inherited.value
  if (!owner || owner.ownerKind !== 'collection') return ''
  return collections.collectionsMap.get(owner.ownerId)?.name ?? ''
})

const inheritedStatus = computed(() => {
  const owner = inherited.value
  if (!owner || owner.authType !== 'oauth2') return ''
  return tokenStatusLabel(tokens.entry(owner.ownerKind, owner.ownerId))
})

// The flow buttons live on the owning collection, so the name is the way there.
function openOwnerCollection() {
  const owner = inherited.value
  if (!owner || owner.ownerKind !== 'collection' || !inheritedOwnerName.value) return
  tabs.openCollectionTab(owner.ownerId, inheritedOwnerName.value, { initialSection: 'authorization' })
}

// Three triggers can overlap, so the newest answer wins.
let load = 0

async function loadInherited() {
  const generation = ++load
  if (props.authType !== 'inherit' || props.ownerKind !== 'request' || !props.ownerId) {
    inherited.value = null
    return
  }

  const service = await getAuthService()
  const result = await service.resolveOwner(props.ownerId)
  if (generation !== load) return
  if (result.error || !result.data.ownerId) {
    inherited.value = null
    return
  }

  inherited.value = result.data
  if (result.data.authType === 'oauth2') {
    await tokens.refresh({
      ownerKind: result.data.ownerKind as AuthOwnerKind,
      ownerId: result.data.ownerId,
      authType: result.data.authType,
      authData: result.data.authData,
    })
  }
}

watch(() => [props.authType, props.ownerId], () => { void loadInherited() })
onMounted(() => { void loadInherited() })
// The tab stays alive under KeepAlive: editing the owning collection and coming
// back would otherwise leave the scheme, the owner and the warning as they were.
onActivated(() => { void loadInherited() })
</script>

<template>
  <div class="space-y-4" :class="hideTypeSelector ? '' : 'm-3'">
    <div v-if="!hideTypeSelector">
      <AuthSelect
        class="max-w-[240px]"
        label="Authorization Type"
        test-id="auth-type-selector"
        :model-value="authType"
        :options="authTypes"
        @update:model-value="selectAuthType($event as AuthType)"
      />
      <p v-if="httpOnlyBlocked" class="mt-1 text-[11px] text-amber-500" data-testid="auth-http-only">
        {{ currentAuthLabel }} is supported for HTTP requests only; this request will fail to send.
      </p>
    </div>

    <div v-if="authType === 'inherit'" class="py-6 text-center">
      <p class="text-sm text-muted-foreground">This request inherits authentication from its parent collection</p>
      <template v-if="inherited">
        <p class="mt-2 text-xs text-muted-foreground" data-testid="inherited-auth">
          {{ inheritedLabel }} configured on
          <button
            v-if="inheritedOwnerName"
            class="text-primary hover:underline cursor-pointer"
            type="button"
            data-testid="inherited-auth-owner"
            @click="openOwnerCollection"
          >collection “{{ inheritedOwnerName }}”</button>
          <template v-else>the parent collection</template>
        </p>
        <p v-if="inheritedHttpOnlyBlocked" class="mt-2 text-[11px] text-amber-500" data-testid="auth-http-only">
          {{ inheritedLabel }} is supported for HTTP requests only; this request will fail to send.
        </p>
        <p v-if="inheritedStatus" class="mt-1 text-xs text-muted-foreground/70" data-testid="inherited-token-status">
          {{ inheritedStatus }} — manage the token on that collection's Auth tab
        </p>
      </template>
    </div>

    <div v-else-if="authType === 'none'" class="py-6 text-center">
      <p class="text-sm text-muted-foreground">No authentication configured for this request</p>
    </div>

    <div v-else-if="authType === 'basic'" class="space-y-3 max-w-md">
      <AuthTextField
        label="Username"
        :model-value="values.username"
        @update:model-value="updateField('username', $event)"
      />
      <AuthTextField
        label="Password"
        :model-value="values.password"
        secret
        @update:model-value="updateField('password', $event)"
      />
    </div>

    <div v-else-if="authType === 'bearer'" class="space-y-3 max-w-md">
      <AuthTextField
        label="Prefix"
        :model-value="values.prefix"
        placeholder="Bearer"
        @update:model-value="updateField('prefix', $event)"
      />
      <AuthTextField
        label="Token"
        :model-value="values.token"
        placeholder="Paste your token here"
        multiline
        @update:model-value="updateField('token', $event)"
      />
    </div>

    <div v-else-if="authType === 'api_key'" class="space-y-3 max-w-md">
      <AuthTextField
        label="Key"
        :model-value="values.key"
        placeholder="X-API-Key"
        @update:model-value="updateField('key', $event)"
      />
      <AuthTextField
        label="Value"
        :model-value="values.value"
        placeholder="Your API key value"
        @update:model-value="updateField('value', $event)"
      />
      <AuthSelect
        label="Add to"
        :model-value="values.addTo"
        :options="addToOptions"
        @update:model-value="updateField('addTo', $event)"
      />
    </div>

    <OAuth2Form
      v-else-if="authType === 'oauth2'"
      :auth-data="authData"
      :owner-kind="ownerKind"
      :owner-id="ownerId"
      :owner-version="ownerVersion"
      @update:auth-data="emit('update:authData', $event)"
    />

    <JwtForm
      v-else-if="authType === 'jwt'"
      :auth-data="authData"
      @update:auth-data="emit('update:authData', $event)"
    />

    <DigestForm
      v-else-if="authType === 'digest'"
      :auth-data="authData"
      @update:auth-data="emit('update:authData', $event)"
    />

    <AwsForm
      v-else-if="authType === 'aws_sigv4'"
      :auth-data="authData"
      @update:auth-data="emit('update:authData', $event)"
    />
  </div>
</template>
