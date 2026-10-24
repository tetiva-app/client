<script setup lang="ts">
import { computed } from 'vue'
import AuthTextField from './AuthTextField.vue'
import AuthSelect from './AuthSelect.vue'
import JsonField from './JsonField.vue'
import { authFormValues, objectFieldText, setAuthField, setObjectField } from '@/lib/auth-data'
import { useScreenCopy } from '@/composables/useLocale'
import { AUTH_COPY } from './copy'

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

const copy = useScreenCopy(AUTH_COPY)

const secretBase64Options = computed(() => [
  { value: 'false', label: copy.value.jwt.plainText },
  { value: 'true', label: copy.value.jwt.base64 },
])

const addToOptions = computed(() => [
  { value: 'header', label: copy.value.fields.inHeader },
  { value: 'query', label: copy.value.fields.inQuery },
])

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
      :label="copy.jwt.algorithm"
      :model-value="values.alg"
      :options="algorithms"
      @update:model-value="update('alg', $event)"
    />

    <template v-if="symmetric">
      <AuthTextField
        :label="copy.jwt.secret"
        :model-value="values.secret"
        secret
        @update:model-value="update('secret', $event)"
      />
      <AuthSelect
        :label="copy.jwt.secretEncoding"
        :model-value="values.secretBase64"
        :options="secretBase64Options"
        @update:model-value="update('secretBase64', $event)"
      />
    </template>

    <AuthTextField
      v-else
      :label="copy.jwt.privateKey"
      :model-value="values.privateKey"
      placeholder="-----BEGIN PRIVATE KEY-----"
      multiline
      secret
      @update:model-value="update('privateKey', $event)"
    />

    <JsonField
      :label="copy.jwt.claims"
      :value="claimsText"
      placeholder='{ "sub": "1234567890" }'
      :hint="copy.jwt.claimsHint"
      @update:value="updateObject('claims', $event)"
    />

    <AuthTextField
      :label="copy.jwt.expiresIn"
      :model-value="values.expiresIn"
      :hint="copy.jwt.expiresHint"
      @update:model-value="update('expiresIn', $event)"
    />

    <JsonField
      :label="copy.jwt.joseHeader"
      :value="headerText"
      placeholder='{ "kid": "key-1" }'
      :hint="copy.jwt.joseHint"
      @update:value="updateObject('header', $event)"
    />

    <AuthSelect
      :label="copy.fields.addTokenTo"
      :model-value="values.addTo"
      :options="addToOptions"
      @update:model-value="update('addTo', $event)"
    />

    <AuthTextField
      v-if="values.addTo === 'query'"
      :label="copy.fields.queryParam"
      :model-value="values.queryParam"
      @update:model-value="update('queryParam', $event)"
    />
    <AuthTextField
      v-else
      :label="copy.fields.headerPrefix"
      :model-value="values.headerPrefix"
      @update:model-value="update('headerPrefix', $event)"
    />
  </div>
</template>
