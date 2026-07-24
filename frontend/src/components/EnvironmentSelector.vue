<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ChevronDown, Settings } from 'lucide-vue-next'
import { useEnvironmentStore } from '@/stores/environments'

const emit = defineEmits<{
  (e: 'manage'): void
}>()

const store = useEnvironmentStore()
const dropdownOpen = ref(false)

onMounted(() => {
  store.fetchAll()
})

function selectEnv(id: string) {
  store.setActive(id)
  dropdownOpen.value = false
}

function clearEnv() {
  store.clearActive()
  dropdownOpen.value = false
}

function handleClickOutside() {
  dropdownOpen.value = false
}

function openManage() {
  dropdownOpen.value = false
  emit('manage')
}
</script>

<template>
  <div class="relative shrink-0">
    <button
      class="flex items-center gap-1.5 h-7 px-2.5 text-xs rounded border border-border hover:bg-muted/30 transition-colors cursor-pointer"
      :class="store.activeEnvironment ? 'text-emerald-400' : 'text-muted-foreground'"
      @click="dropdownOpen = !dropdownOpen"
    >
      <span class="inline-block size-1.5 rounded-full" :class="store.activeEnvironment ? 'bg-emerald-400' : 'bg-muted-foreground'" />
      {{ store.activeEnvironment?.name ?? 'No Environment' }}
      <ChevronDown class="size-3 text-muted-foreground" />
    </button>

    <Teleport to="body">
      <div
        v-if="dropdownOpen"
        class="fixed inset-0 z-40"
        @click="handleClickOutside"
      />
    </Teleport>

    <div
      v-if="dropdownOpen"
      class="absolute top-full right-0 z-50 mt-1 min-w-[180px] rounded-md border border-border bg-popover py-1 shadow-md"
    >
      <button
        class="flex w-full items-center gap-2 px-3 py-1.5 text-xs hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer text-muted-foreground"
        :class="{ 'bg-black/5 dark:bg-white/10': !store.activeEnvironment }"
        @click="clearEnv"
      >
        <span class="inline-block size-1.5 rounded-full bg-muted-foreground" />
        No Environment
      </button>

      <div v-if="store.environments.length > 0" class="my-1 border-t border-border" />

      <button
        v-for="env in store.environments"
        :key="env.id"
        class="flex w-full items-center gap-2 px-3 py-1.5 text-xs hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
        :class="{ 'bg-black/5 dark:bg-white/10 text-emerald-400': env.isActive, 'text-foreground': !env.isActive }"
        @click="selectEnv(env.id)"
      >
        <span class="inline-block size-1.5 rounded-full" :class="env.isActive ? 'bg-emerald-400' : 'bg-transparent'" />
        {{ env.name }}
      </button>

      <div class="my-1 border-t border-border" />

      <button
        class="flex w-full items-center gap-2 px-3 py-1.5 text-xs text-muted-foreground hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
        @click="openManage"
      >
        <Settings class="size-3" />
        Manage Environments...
      </button>
    </div>
  </div>
</template>
