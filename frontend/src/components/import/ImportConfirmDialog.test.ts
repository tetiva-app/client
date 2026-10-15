import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { createSSRApp, defineComponent, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createPinia, setActivePinia } from 'pinia'
import type { ImportPreview, ImportWarning } from '@/services'
import { setCurrentLocale } from '@/lib/locale'

vi.mock('@/services', () => ({ isWailsEnvironment: () => false }))

vi.mock('@/components/ui/dialog', () => {
  const inline = (tag: string) => defineComponent({
    inheritAttrs: false,
    setup: (_, { slots, attrs }) => () => h(tag, attrs, slots.default?.()),
  })
  return {
    Dialog: inline('div'),
    DialogContent: inline('div'),
    DialogDescription: inline('p'),
    DialogFooter: inline('div'),
    DialogHeader: inline('div'),
    DialogTitle: inline('h2'),
  }
})

import ImportConfirmDialog from './ImportConfirmDialog.vue'
import { useImportUi } from '@/stores/importUi'
import { useWorkspaceStore } from '@/stores/workspace'
import { useCollectionStore } from '@/stores/collections'
import { useEnvironmentStore } from '@/stores/environments'

afterEach(() => { setCurrentLocale('en') })

const PREVIEW: ImportPreview = {
  format: 'postman', title: 'Legacy', folders: 0, requests: 2, examples: 0, environmentNames: [],
  hosts: ['api.example.com'], scripts: [], warnings: [],
}

const DROPPED: ImportWarning = {
  code: 'file_body_dropped', subject: { kind: 'request', name: 'a' }, params: {}, text: 'request "a": dropped',
}

interface RenderOpts {
  preview?: Partial<ImportPreview>
  remote?: string | null
  environments?: string[]
}

async function render(folderAuth: string, opts: RenderOpts = {}): Promise<string> {
  const pinia = createPinia()
  setActivePinia(pinia)
  useWorkspaceStore().workspaces = [{
    id: 'w1', name: 'Team', isActive: true, version: 1, remoteWorkspaceId: opts.remote ?? null, createdAt: '', updatedAt: '',
  }]
  useEnvironmentStore().environments = (opts.environments ?? []).map((name, i) => ({
    id: `e${i}`, name, isActive: false, version: 1, createdAt: '', updatedAt: '',
  }))
  useCollectionStore().collectionsMap.set('folder-1', {
    id: 'folder-1', name: 'Refunds', parentId: null, workspaceId: 'w1', authType: folderAuth, authData: '{}',
  } as never)
  const ui = useImportUi()
  ui.source = { kind: 'file', previewId: 'p-1', content: '{}', fileName: 'legacy.json', parentId: 'folder-1' }
  ui.preview = { ...PREVIEW, ...opts.preview }
  const app = createSSRApp(ImportConfirmDialog)
  app.use(pinia)
  return renderToString(app)
}

describe('import confirmation', () => {
  it("warns that a Postman import into a folder picks up the folder's authorization", async () => {
    const html = await render('bearer')

    expect(html).toContain('data-testid="import-inherited-auth"')
    expect(html).toContain('Requests without their own authorization will use Refunds&#39;s authorization')
  })

  it('stays quiet for a folder without authorization', async () => {
    expect(await render('none')).not.toContain('data-testid="import-inherited-auth"')
  })

  it('tells where Postman collection variables go, under the name the import will give them', async () => {
    const html = await render('none', { preview: { environmentNames: ['Legacy'] }, environments: ['Default', 'Legacy'] })

    expect(html).toContain('Collection variables become the environment “Legacy (2)”.')
    expect(html).toContain('only one environment is active at a time')
  })

  it('names the HAR and Hoppscotch formats and words Hoppscotch variables like Postman ones', async () => {
    expect(await render('none', { preview: { format: 'har' } })).toContain('HAR file · 0 folders')

    const html = await render('none', { preview: { format: 'hoppscotch', environmentNames: ['Petstore'] } })
    expect(html).toContain('Hoppscotch collection · 0 folders')
    expect(html).toContain('Collection variables become the environment “Petstore”.')
  })

  it('keeps the Tetiva wording for a snapshot environment', async () => {
    const html = await render('none', { preview: { format: 'tetiva', environmentNames: ['prod'] } })

    expect(html).toContain('Adds the environment “prod”. Secret values stay empty.')
    expect(html).not.toContain('Collection variables')
  })

  it('names an OpenAPI spec and tells where its server and credentials go', async () => {
    const html = await render('bearer', { preview: { format: 'openapi', environmentNames: ['Petstore'] } })

    expect(html).toContain('OpenAPI spec · 0 folders · 2 requests · 0 examples')
    expect(html).toContain('The server address, path parameters and credentials go to the environment “Petstore”, which becomes active.')
    expect(html).toContain('data-testid="import-inherited-auth"')
  })

  it('names an Insomnia file and lists the environments it adds', async () => {
    const html = await render('none', {
      preview: { format: 'insomnia', environmentNames: ['Petstore · Local', 'Petstore · Production'] },
      environments: ['Petstore · Local'],
      remote: 'r1',
    })

    expect(html).toContain('Insomnia collection · ')
    expect(html).toContain('Adds the environments “Petstore · Local (2)”, “Petstore · Production”.')
    expect(html).toContain(
      'This collection and the environments “Petstore · Local (2)”, “Petstore · Production” will be shared with everyone in Team.',
    )
    expect(html).not.toContain('Collection variables')
  })

  it('words the switch for one Insomnia environment', async () => {
    const html = await render('none', { preview: { format: 'insomnia', environmentNames: ['Petstore'] } })

    expect(html).toContain('Adds the environment “Petstore”. Switch to it in the environment picker to use it')
  })

  it('tells that each Hoppscotch collection gets its own environment', async () => {
    const html = await render('none', { preview: { format: 'hoppscotch', environmentNames: ['Billing', 'Shop'] } })

    expect(html).toContain('Collection variables become the environments “Billing”, “Shop”, one per collection. Switch to one in the environment picker')
  })

  it('warns that a cloud workspace gets the environment too', async () => {
    const html = await render('none', { preview: { environmentNames: ['Legacy'] }, remote: 'r1' })

    expect(html).toContain('This collection and the environment “Legacy” will be shared with everyone in Team.')
  })

  it('shows no environment section without one', async () => {
    const html = await render('none', { remote: 'r1' })

    expect(html).not.toContain('data-testid="import-environment"')
    expect(html).toContain('This collection will be shared with everyone in Team.')
  })
})

describe('import confirmation wording', () => {
  const scripts = (n: number) => Array.from({ length: n }, (_, i) => ({ path: `Request ${i}`, phase: 'pre' as const, text: '' }))

  it('keeps the English header, destination, scripts and buttons', async () => {
    const html = await render('none', { preview: { scripts: scripts(1), warnings: [DROPPED] } })

    expect(html).toContain('Import “Legacy”')
    expect(html).toContain('Destination')
    expect(html).toContain('Workspace “Team”, folder “Refunds”')
    expect(html).toContain('Sends requests to')
    expect(html).toContain('Changed on import · 1')
    expect(html).toContain('This collection contains 1 script')
    expect(html).toContain('Import scripts')
    expect(html).toContain('Cancel')
  })

  it('words the notes from their codes and shows an unknown one as sent', async () => {
    const unknown: ImportWarning = { code: 'brand_new', subject: { kind: '', name: '' }, params: {}, text: 'something new' }
    const html = await render('none', { preview: { warnings: [DROPPED, unknown] } })

    expect(html).toContain('request &quot;a&quot;: the file body was imported without its file; pick it again')
    expect(html).toContain('something new')
    expect(html).not.toContain('dropped')
  })

  describe('in Russian', () => {
    beforeEach(() => { setCurrentLocale('ru') })

    it('speaks Russian in the header, destination and buttons', async () => {
      const html = await render('bearer', { preview: { title: '' } })

      expect(html).toContain('Импорт «Без названия»')
      expect(html).toContain('Коллекция Postman · 0 папок · 2 запроса · 0 примеров')
      expect(html).toContain('Куда импортировать')
      expect(html).toContain('Пространство «Team», папка «Refunds»')
      expect(html).toContain('Запросы без собственной авторизации унаследуют её от «Refunds».')
      expect(html).toContain('Куда уходят запросы')
      expect(html).toContain('Отмена')
      expect(html).toContain('Импортировать')
      for (const english of ['Import “', 'Destination', 'Workspace', 'Sends requests to', 'Cancel']) {
        expect(html).not.toContain(english)
      }
    })

    it('names the format of every kind of file', async () => {
      const kinds = {
        tetiva: 'Коллекция Tetiva',
        insomnia: 'Коллекция Insomnia',
        openapi: 'Спецификация OpenAPI',
        hoppscotch: 'Коллекция Hoppscotch',
        har: 'HAR-файл',
      } as const
      for (const [format, kind] of Object.entries(kinds)) {
        expect(await render('none', { preview: { format: format as ImportPreview['format'] } })).toContain(`${kind} · 0 папок`)
      }
    })

    it('names the environment and the shared workspace', async () => {
      const html = await render('none', { preview: { environmentNames: ['Legacy'] }, environments: ['Legacy'], remote: 'r1' })

      expect(html).toContain('>Окружение<')
      expect(html).toContain('Переменные коллекции попадут в окружение «Legacy (2)».')
      expect(html).toContain('Коллекция и окружение «Legacy (2)» станут доступны всем в пространстве «Team».')
    })

    it('words several environments, one per collection', async () => {
      const html = await render('none', { preview: { format: 'hoppscotch', environmentNames: ['Billing', 'Shop'] } })

      expect(html).toContain('>Окружения<')
      expect(html).toContain('Переменные коллекций попадут в окружения «Billing», «Shop», по одному на коллекцию.')
      expect(html).toContain('выберите одно из них в списке окружений')
    })

    it('words the OpenAPI, Insomnia and Tetiva environments', async () => {
      expect(await render('none', { preview: { format: 'openapi', environmentNames: ['Petstore'] } }))
        .toContain('Адрес сервера, параметры пути и учётные данные попадут в окружение «Petstore», и оно станет активным.')
      expect(await render('none', { preview: { format: 'insomnia', environmentNames: ['Local', 'Prod'] } }))
        .toContain('Импорт добавит окружения «Local», «Prod».')
      expect(await render('none', { preview: { format: 'tetiva', environmentNames: ['prod'] } }))
        .toContain('Импорт добавит окружение «prod». Секретные значения останутся пустыми.')
    })

    it('tells a cloud workspace without an environment', async () => {
      expect(await render('none', { remote: 'r1' })).toContain('Коллекция станет доступна всем в пространстве «Team».')
    })

    it('counts the scripts and the notes', async () => {
      const html = await render('none', { preview: { scripts: scripts(5), warnings: [DROPPED] } })

      expect(html).toContain('В коллекции 5 скриптов')
      expect(html).toContain('Импортировать скрипты')
      expect(html).toContain('Изменено при импорте · 1')
      expect(html).toContain('запрос «a»: файл из тела запроса (Binary) не импортирован\u00a0— выберите его заново')
    })
  })
})
