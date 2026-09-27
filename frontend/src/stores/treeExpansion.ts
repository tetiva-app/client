import { ref } from 'vue'
import { defineStore } from 'pinia'

export const useTreeExpansionStore = defineStore('treeExpansion', () => {
  const expanded = ref(new Set<string>())

  function isExpanded(id: string): boolean {
    return expanded.value.has(id)
  }

  function setExpanded(id: string, open: boolean) {
    if (open) expanded.value.add(id)
    else expanded.value.delete(id)
  }

  return { expanded, isExpanded, setExpanded }
})
