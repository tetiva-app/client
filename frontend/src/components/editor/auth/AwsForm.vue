<script setup lang="ts">
import { computed } from 'vue'
import AuthTextField from './AuthTextField.vue'
import { authFormValues, setAuthField } from '@/lib/auth-data'

const props = defineProps<{
  authData: string
}>()

const emit = defineEmits<{
  (e: 'update:authData', value: string): void
}>()

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
      hint="Only for temporary STS credentials."
      @update:model-value="update('sessionToken', $event)"
    />
    <AuthTextField
      label="Region"
      :model-value="values.region"
      placeholder="us-east-1"
      @update:model-value="update('region', $event)"
    />
    <AuthTextField
      label="Service"
      :model-value="values.service"
      placeholder="execute-api"
      @update:model-value="update('service', $event)"
    />
  </div>
</template>
