import { ref, readonly } from 'vue'

const selectedIds = ref<Set<string>>(new Set())
const lastSelectedId = ref<string | null>(null)

export function useTreeSelection() {
  function toggleSelect(id: string) {
    const newSet = new Set(selectedIds.value)
    if (newSet.has(id)) {
      newSet.delete(id)
    } else {
      newSet.add(id)
    }
    selectedIds.value = newSet
    lastSelectedId.value = id
  }

  function rangeSelect(id: string) {
    if (!lastSelectedId.value) {
      toggleSelect(id)
      return
    }

    const items = Array.from(document.querySelectorAll<HTMLElement>('[data-tree-item-id]'))
    const ids = items.map(el => el.dataset.treeItemId!)

    const startIdx = ids.indexOf(lastSelectedId.value)
    const endIdx = ids.indexOf(id)
    if (startIdx === -1 || endIdx === -1) {
      toggleSelect(id)
      return
    }

    const from = Math.min(startIdx, endIdx)
    const to = Math.max(startIdx, endIdx)

    const newSet = new Set(selectedIds.value)
    for (let i = from; i <= to; i++) {
      newSet.add(ids[i])
    }
    selectedIds.value = newSet
    lastSelectedId.value = id
  }

  // Anchorless on purpose: the ids come from code, not from a click to range-select from.
  function setSelection(ids: string[]) {
    selectedIds.value = new Set(ids)
    lastSelectedId.value = null
  }

  function clearSelection() {
    setSelection([])
  }

  function isSelected(id: string): boolean {
    return selectedIds.value.has(id)
  }

  function hasSelection(): boolean {
    return selectedIds.value.size > 0
  }

  function getSelectedIds(): string[] {
    return Array.from(selectedIds.value)
  }

  return {
    selectedIds: readonly(selectedIds),
    isSelected,
    hasSelection,
    getSelectedIds,
    toggleSelect,
    rangeSelect,
    setSelection,
    clearSelection,
  }
}
