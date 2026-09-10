<script setup lang="ts">
import { computed } from 'vue'
import AuthTextField from './AuthTextField.vue'
import AuthSelect from './AuthSelect.vue'
import JsonField from './JsonField.vue'
import { authFormValues, objectFieldText, setAuthField, setObjectField } from '@/lib/auth-data'

const props = defineProps<{
  authData: string
}>()

const emit = defineEmits<{
  (e: 'update:authData', value: string): void
}>()

// Every algorithm jwt.GetSigningMethod resolves to a key type signJWT can load.
const algorithms = [
  'HS256', 'HS384', 'HS512',
  'RS256', 'RS384', 'RS512',
  'PS256', 'PS384', 'PS512',
  'ES256', 'ES384', 'ES512',
  'EdDSA',
].map(value => ({ value, label: value }))

const secretBase64Options = [
  { value: 'false', label: 'Plain text' },
  { value: 'true', label: 'Base64 encoded' },
]

const addToOptions = [
  { value: 'header', label: 'Header' },
  { value: 'query', label: 'Query Params' },
]

const values = computed(() => authFormValues('jwt', props.authData))
const symmetric = computed(() => values.value.alg.startsWith('HS'))
const claimsText = computed(() => objectFieldText(props.authData, 'claims'))
const headerText = computed(() => objectFieldText(props.authData, 'header'))

function update(key: string, value: string) {
  emit('update:authData', setAuthField(props.authData, key, value))
}

// A half-typed object is kept in the field; auth_data only takes a complete one.
function updateObject(key: string, text: string) {
  const next = setObjectField(props.authData, key, text)
  if (next !== null) emit('update:authData', next)
}
</script>

<template>
  <div class="space-y-3 max-w-md">
    <AuthSelect
      label="Algorithm"
      :model-value="values.alg"
      :options="algorithms"
      @update:model-value="update('alg', $event)"
    />

    <template v-if="symmetric">
      <AuthTextField
        label="Secret"
        :model-value="values.secret"
        secret
        @update:model-value="update('secret', $event)"
      />
      <AuthSelect
        label="Secret Encoding"
        :model-value="values.secretBase64"
        :options="secretBase64Options"
        @update:model-value="update('secretBase64', $event)"
      />
    </template>

    <AuthTextField
      v-else
      label="Private Key"
      :model-value="values.privateKey"
      placeholder="-----BEGIN PRIVATE KEY-----"
      multiline
      secret
      @update:model-value="update('privateKey', $event)"
    />

    <JsonField
      label="Payload (claims)"
      :value="claimsText"
      placeholder='{ "sub": "1234567890" }'
      hint="iat and exp are added at send time unless the claims set them."
      @update:value="updateObject('claims', $event)"
    />

    <AuthTextField
      label="Expires In (seconds)"
      :model-value="values.expiresIn"
      hint="0 signs a token without exp."
      @update:model-value="update('expiresIn', $event)"
    />

    <JsonField
      label="JOSE Header"
      :value="headerText"
      placeholder='{ "kid": "key-1" }'
      hint="kid, typ, cty, x5t, x5u and jku only — alg comes from the algorithm above."
      @update:value="updateObject('header', $event)"
    />

    <AuthSelect
      label="Add token to"
      :model-value="values.addTo"
      :options="addToOptions"
      @update:model-value="update('addTo', $event)"
    />

    <AuthTextField
      v-if="values.addTo === 'query'"
      label="Query Parameter"
      :model-value="values.queryParam"
      @update:model-value="update('queryParam', $event)"
    />
    <AuthTextField
      v-else
      label="Header Prefix"
      :model-value="values.headerPrefix"
      @update:model-value="update('headerPrefix', $event)"
    />
  </div>
</template>
