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

const values = computed(() => authFormValues('digest', props.authData))

function update(key: string, value: string) {
  emit('update:authData', setAuthField(props.authData, key, value))
}
</script>

<template>
  <div class="space-y-3 max-w-md">
    <AuthTextField
      label="Username"
      :model-value="values.username"
      @update:model-value="update('username', $event)"
    />
    <AuthTextField
      label="Password"
      :model-value="values.password"
      secret
      @update:model-value="update('password', $event)"
    />
    <p class="text-[11px] text-muted-foreground/60">
      The algorithm follows the server challenge: MD5, SHA-256 and SHA-512-256 are supported.
    </p>
  </div>
</template>
