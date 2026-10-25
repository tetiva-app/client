<script setup lang="ts">
import { computed } from 'vue'
import AuthTextField from './AuthTextField.vue'
import { authFormValues, setAuthField } from '@/lib/auth-data'
import { useScreenCopy } from '@/composables/useLocale'
import { AUTH_COPY } from './copy'

const props = defineProps<{
  authData: string
}>()

const emit = defineEmits<{
  (e: 'update:authData', value: string): void
}>()

const copy = useScreenCopy(AUTH_COPY)
const values = computed(() => authFormValues('aws_sigv4', props.authData))

function update(key: string, value: string) {
  emit('update:authData', setAuthField(props.authData, key, value))
}
</script>

<template>
  <div class="space-y-3 max-w-md">
    <AuthTextField
      label="Access Key ID"
      :model-value="values.accessKeyId"
      placeholder="AKIA..."
      @update:model-value="update('accessKeyId', $event)"
    />
    <AuthTextField
      label="Secret Access Key"
      :model-value="values.secretAccessKey"
      secret
      @update:model-value="update('secretAccessKey', $event)"
    />
    <AuthTextField
      label="Session Token"
      :model-value="values.sessionToken"
      secret
      :hint="copy.aws.sessionTokenHint"
      @update:model-value="update('sessionToken', $event)"
    />
    <AuthTextField
      :label="copy.aws.region"
      :model-value="values.region"
      placeholder="us-east-1"
      @update:model-value="update('region', $event)"
    />
    <AuthTextField
      :label="copy.aws.service"
      :model-value="values.service"
      placeholder="execute-api"
      @update:model-value="update('service', $event)"
    />
  </div>
</template>
