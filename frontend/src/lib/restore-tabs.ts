import type { RestoreTabs } from '@/services'

export async function restoreTabs(saved: RestoreTabs | null, deps: {
  activeWorkspaceId: string | undefined
  openTab: (id: string) => Promise<void>
  openCollection: (id: string) => void
  setActive: (tabId: string) => void
  hasTab: (tabId: string) => boolean
}): Promise<void> {
  if (!saved || saved.workspaceId !== deps.activeWorkspaceId) return
  for (const tab of saved.tabs) {
    if (tab.type === 'request') await deps.openTab(tab.id)
    else deps.openCollection(tab.id)
  }
  if (deps.hasTab(saved.activeTabId)) deps.setActive(saved.activeTabId)
}
