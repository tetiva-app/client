import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp, defineComponent, type Component } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { HistoryRecord } from '@/types/history'

const dialogs = vi.hoisted(() => [] as Record<string, unknown>[])

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

vi.mock('@/components/ConfirmDialog.vue', () => ({
  default: defineComponent({
    props: ['open', 'title', 'description', 'confirmLabel', 'cancelLabel', 'destructive'],
    setup: (props) => { dialogs.push(props); return () => null },
  }),
}))

import HistorySidebar from './HistorySidebar.vue'
import HistoryFilterPopover from './HistoryFilterPopover.vue'
import { useHistoryStore } from '@/stores/history'
import { useWorkspaceStore } from '@/stores/workspace'
import { setCurrentLocale } from '@/lib/locale'
import type { Workspace } from '@/types/workspace'

const DAY = 86_400_000

function record(id: string, ageMs: number): HistoryRecord {
  return {
    id, workspaceId: 'w1', protocol: 'http', method: 'GET', url: `https://api.example.com/${id}`,
    requestHeaders: {}, requestBody: '', responseStatus: 200, responseHeaders: {}, responseBody: '',
    responseSize: 0, durationMs: 12, createdAt: new Date(Date.now() - ageMs).toISOString(),
  }
}

function render(component: Component, seed: (history: ReturnType<typeof useHistoryStore>) => void = () => {}): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  useWorkspaceStore().workspaces = [{ id: 'w1', name: 'Main', isActive: true } as Workspace]
  seed(useHistoryStore())
  const app = createSSRApp(component)
  app.use(pinia)
  return renderToString(app)
}

function withRecords(history: ReturnType<typeof useHistoryStore>) {
  history.items = [record('a', 0), record('b', DAY), record('c', 30 * DAY)]
  history.totalCount = 1284
  history.filter.requestId = 'r1'
}

afterEach(() => {
  setCurrentLocale('en')
  dialogs.length = 0
})

describe('history panel', () => {
  it('keeps its English wording', async () => {
    const html = await render(HistorySidebar, withRecords)

    expect(html).toContain('>History<')
    expect(html).toContain('title="Clear all history"')
    expect(html).toContain('placeholder="Search URL..."')
    expect(html).toContain('title="Filters"')
    expect(html).toContain('Filtered by request')
    expect(html).toContain('title="Clear filter"')
    for (const bucket of ['Today', 'Yesterday', 'Earlier']) expect(html).toContain(bucket)
    expect(html).toContain('title="Replay (open as draft)"')
    expect(html).toContain('title="Delete"')
    expect(html).toContain('just now')
    expect(html).toContain('30 days ago')
    expect(html).toContain('12ms')
    expect(html).toContain('3 of 1,284 records')
    expect(html).toContain('Load more')
  })

  it('speaks Russian when the app does', async () => {
    setCurrentLocale('ru')
    const html = await render(HistorySidebar, withRecords)

    expect(html).toContain('>История<')
    expect(html).toContain('title="Очистить всю историю"')
    expect(html).toContain('placeholder="Поиск по URL…"')
    expect(html).toContain('title="Фильтры"')
    expect(html).toContain('Фильтр по запросу')
    expect(html).toContain('title="Сбросить фильтр"')
    for (const bucket of ['Сегодня', 'Вчера', 'Раньше']) expect(html).toContain(bucket)
    expect(html).toContain('title="Повторить (открыть как черновик)"')
    expect(html).toContain('title="Удалить"')
    expect(html).toContain('только что')
    expect(html).toContain('30 дней назад')
    expect(html).toContain('12 мс')
    expect(html).toContain('Показано 3 из 1\u00a0284')
    expect(html).toContain('Загрузить ещё')
    expect(html).not.toMatch(/Today|Load more|records/)
  })

  it('asks before clearing and deleting in English', async () => {
    await render(HistorySidebar, withRecords)
    const [clear, remove] = dialogs

    expect(clear.title).toBe('Clear all history?')
    expect(clear.description).toBe('This deletes 1,284 records in workspace Main. This cannot be undone.')
    expect(clear.confirmLabel).toBe('Clear all')
    expect(clear.cancelLabel).toBe('Cancel')
    expect(remove.title).toBe('Delete history record?')
    expect(remove.description).toBe('This cannot be undone.')
    expect(remove.confirmLabel).toBe('Delete')
  })

  it('counts the records to clear in Russian', async () => {
    setCurrentLocale('ru')
    await render(HistorySidebar, withRecords)
    const [clear, remove] = dialogs

    expect(clear.title).toBe('Очистить всю историю?')
    expect(clear.description).toBe('В пространстве «Main» будут удалены 1 284 записи. Это действие нельзя отменить.')
    expect(clear.confirmLabel).toBe('Очистить')
    expect(clear.cancelLabel).toBe('Отмена')
    expect(remove.title).toBe('Удалить запись из истории?')
    expect(remove.description).toBe('Это действие нельзя отменить.')
    expect(remove.confirmLabel).toBe('Удалить')
    expect(remove.cancelLabel).toBe('Отмена')
  })

  it('words the empty states in Russian', async () => {
    setCurrentLocale('ru')
    const empty = await render(HistorySidebar)
    const noMatches = await render(HistorySidebar, h => { h.totalCount = 4; h.filter.urlContains = 'pets' })

    expect(empty).toContain('Истории пока нет')
    expect(empty).toContain('Отправленные запросы появятся здесь.')
    expect(noMatches).toContain('Ничего не найдено')
    expect(noMatches).toContain('Попробуйте изменить или сбросить фильтры.')
    expect(noMatches).toContain('Сбросить фильтры')
  })

  it('keeps the English empty states', async () => {
    const empty = await render(HistorySidebar)
    const noMatches = await render(HistorySidebar, h => { h.totalCount = 4; h.filter.urlContains = 'pets' })

    expect(empty).toContain('No request history yet')
    expect(empty).toContain('Execute a request to see it here.')
    expect(noMatches).toContain('No matches')
    expect(noMatches).toContain('Try clearing or relaxing your filters.')
  })
})

describe('history filters', () => {
  it('keeps the English labels', async () => {
    const html = await render(HistoryFilterPopover)

    expect(html).toContain('Protocol')
    expect(html).toContain('Status')
    expect(html).toMatch(/>\s*error\s*</)
    expect(html).toContain('Clear filters')
    expect(html).toContain('Esc to close')
  })

  it('speaks Russian and keeps protocol names and status classes', async () => {
    setCurrentLocale('ru')
    const html = await render(HistoryFilterPopover)

    expect(html).toContain('Протокол')
    expect(html).toContain('Статус')
    expect(html).toMatch(/>\s*ошибка\s*</)
    expect(html).toContain('Сбросить фильтры')
    expect(html).toContain('Esc — закрыть')
    for (const kept of ['HTTP', 'gRPC', 'GraphQL', 'WS', '2xx', '5xx']) expect(html).toContain(kept)
  })
})
