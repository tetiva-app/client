import { describe, it, expect, vi, afterEach } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { Collection } from '@/types/collection'
import type { Request } from '@/types/request'
import { setCurrentLocale } from '@/lib/locale'
import { MAX_DESCRIPTION_BYTES } from '@/lib/description'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))
vi.mock('./MarkdownEditor.vue', () => ({ __esModule: true, default: { render: () => null } }))

import CollectionOverview from './CollectionOverview.vue'
import { useCollectionStore } from '@/stores/collections'
import { useRequestStore } from '@/stores/tabs'

afterEach(() => { setCurrentLocale('en') })

function text(html: string): string {
  return html.replace(/<!--[\s\S]*?-->/g, '').replace(/<[^>]+>/g, ' ').replace(/[ \t\n]+/g, ' ').trim()
}

async function render(opts: { folders?: number; requests?: number; description?: string } = {}): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  const collections = useCollectionStore().collectionsMap
  collections.set('c1', { id: 'c1', name: 'Petstore', parentId: null } as Collection)
  for (let i = 0; i < (opts.folders ?? 0); i++) {
    collections.set(`f${i}`, { id: `f${i}`, name: `F${i}`, parentId: 'c1' } as Collection)
  }
  for (let i = 0; i < (opts.requests ?? 0); i++) {
    useRequestStore().requestsMap.set(`r${i}`, { id: `r${i}`, collectionId: 'c1' } as Request)
  }
  const app = createSSRApp(CollectionOverview, { collectionId: 'c1', description: opts.description ?? '' })
  app.use(pinia)
  return text(await renderToString(app))
}

describe('collection overview', () => {
  it('counts requests and folders', async () => {
    expect(await render({ requests: 1, folders: 2 })).toContain('1 request 2 folders')
  })

  it('counts requests and folders in Russian', async () => {
    setCurrentLocale('ru')

    expect(await render({ requests: 2, folders: 1 })).toContain('2 запроса 1 папка')
    expect(await render({ requests: 5, folders: 5 })).toContain('5 запросов 5 папок')
  })

  it('tells an empty collection and offers a description in Russian', async () => {
    setCurrentLocale('ru')

    const html = await render()

    expect(html).toContain('Пустая коллекция')
    expect(html).toContain('Описание')
    expect(html).toContain('Добавить описание')
  })

  it('warns in Russian that a description over the limit will not save', async () => {
    setCurrentLocale('ru')

    const html = await render({ requests: 1, description: 'a'.repeat(MAX_DESCRIPTION_BYTES + 1) })

    expect(html).toContain('16\u00a0385 / 16\u00a0384 байта\u00a0— больше лимита, сохранится только после сокращения')
    expect(html).toContain('Изменить')
  })

  it('keeps the English counter', async () => {
    const html = await render({ requests: 1, description: 'a'.repeat(MAX_DESCRIPTION_BYTES + 1) })

    expect(html).toContain('16,385 / 16,384 bytes — over the limit, saves only when shortened')
  })
})
