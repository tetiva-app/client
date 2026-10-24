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
const values = computed(() => authFormValues('digest', props.authData))

function update(key: string, value: string) {
  emit('update:authData', setAuthField(props.authData, key, value))
}
</script>

<template>
  <div class="space-y-3 max-w-md">
    <AuthTextField
      :label="copy.fields.username"
      :model-value="values.username"
      @update:model-value="update('username', $event)"
    />
    <AuthTextField
      :label="copy.fields.password"
      :model-value="values.password"
      secret
      @update:model-value="update('password', $event)"
    />
    <p class="text-[11px] text-muted-foreground/60">
      {{ copy.digest.algorithmNote }}
    </p>
  </div>
</template>
