<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import type { AuthType } from '@/types/request'
import AuthSelect from './auth/AuthSelect.vue'
import AuthTextField from './auth/AuthTextField.vue'
import InheritedAuth from './InheritedAuth.vue'
import { AUTH_TYPE_LABELS, REQUEST_AUTH_OPTIONS } from '@/constants/auth'
import { authFormValues, setAuthField } from '@/lib/auth-data'
import { switchAuthDraft } from '@/composables/useAuthDrafts'
import { hasLiteral } from '@/lib/secrets'
import { useAuthTokenStore } from '@/stores/auth-tokens'
import type { AuthOwnerKind } from '@/services/auth-api'
import { useScreenCopy } from '@/composables/useLocale'
import { fill } from '@/lib/locale'
import { AUTH_COPY } from './auth/copy'

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

const copy = useScreenCopy(AUTH_COPY)

const addToOptions = computed(() => [
  { value: 'header', label: copy.value.fields.inHeader },
  { value: 'query', label: copy.value.fields.inQuery },
])

const values = computed(() => authFormValues(props.authType, props.authData))

// A type this build does not know still names itself rather than reading "None".
const currentAuthLabel = computed(() => AUTH_TYPE_LABELS[props.authType] || props.authType || 'None')

// A mis-click on the type selector must not wipe credentials: what each type
// held is kept for the session and handed back when the user returns to it.
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
  const next = switchAuthDraft(props.ownerId, props.authType, props.authData, type)
  emit('update:authType', type)
  emit('update:authData', next)
}

function updateField(field: string, value: string) {
  emit('update:authData', setAuthField(props.authData, field, value))
}

const tokens = useAuthTokenStore()
</script>

<template>
  <div class="space-y-4" :class="hideTypeSelector ? '' : 'm-3'">
    <div v-if="!hideTypeSelector">
      <AuthSelect
        class="max-w-[240px]"
        :label="copy.typeLabel"
        test-id="auth-type-selector"
        :model-value="authType"
        :options="authTypes"
        @update:model-value="selectAuthType($event as AuthType)"
      />
      <p v-if="httpOnlyBlocked" class="mt-1 text-[11px] text-amber-500" data-testid="auth-http-only">
        {{ fill(copy.httpOnly, { label: currentAuthLabel }) }}
      </p>
    </div>

    <div v-if="authType === 'inherit'" class="py-6 text-center">
      <p class="text-sm text-muted-foreground">{{ copy.request.inherits }}</p>
      <InheritedAuth v-if="ownerKind === 'request' && ownerId" kind="request" :id="ownerId" :protocol="protocol">
        <template #none>{{ copy.request.noParent }}</template>
      </InheritedAuth>
    </div>

    <div v-else-if="authType === 'none'" class="py-6 text-center">
      <p class="text-sm text-muted-foreground">{{ copy.request.none }}</p>
    </div>

    <div v-else-if="authType === 'basic'" class="space-y-3 max-w-md">
      <AuthTextField
        :label="copy.fields.username"
        :model-value="values.username"
        @update:model-value="updateField('username', $event)"
      />
      <AuthTextField
        :label="copy.fields.password"
        :model-value="values.password"
        secret
        @update:model-value="updateField('password', $event)"
      />
    </div>

    <div v-else-if="authType === 'bearer'" class="space-y-3 max-w-md">
      <AuthTextField
        :label="copy.fields.prefix"
        :model-value="values.prefix"
        placeholder="Bearer"
        @update:model-value="updateField('prefix', $event)"
      />
      <AuthTextField
        :label="copy.fields.token"
        :model-value="values.token"
        :placeholder="copy.fields.tokenPlaceholder"
        multiline
        :secret="hasLiteral(values.token)"
        :hint="values.token.trim() ? undefined : copy.fields.tokenMissing"
        @update:model-value="updateField('token', $event)"
      />
    </div>

    <div v-else-if="authType === 'api_key'" class="space-y-3 max-w-md">
      <AuthTextField
        :label="copy.fields.key"
        :model-value="values.key"
        placeholder="X-API-Key"
        :hint="values.key.trim() ? undefined : copy.fields.keyMissing"
        @update:model-value="updateField('key', $event)"
      />
      <AuthTextField
        :label="copy.fields.value"
        :model-value="values.value"
        :placeholder="copy.fields.valuePlaceholder"
        :secret="hasLiteral(values.value)"
        @update:model-value="updateField('value', $event)"
      />
      <AuthSelect
        :label="copy.fields.addTo"
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
