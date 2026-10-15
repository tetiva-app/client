import { describe, it, expect, afterEach } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { setCurrentLocale } from '@/lib/locale'
import MarkdownToolbar from './MarkdownToolbar.vue'

afterEach(() => { setCurrentLocale('en') })

async function labels(props: { inTable?: boolean } = {}): Promise<string[]> {
  const html = await renderToString(createSSRApp(MarkdownToolbar, props))
  return [...html.matchAll(/aria-label="([^"]+)"/g)].map(m => m[1])
}

describe('markdown toolbar', () => {
  it('labels the buttons in English', async () => {
    expect(await labels()).toEqual([
      'Bold', 'Italic', 'Inline code', 'Link', 'Heading', 'Bullet list', 'Numbered list', 'Quote', 'Code block', 'Insert table',
    ])
    expect(await labels({ inTable: true })).toContain('Table actions')
  })

  it('labels the buttons in Russian', async () => {
    setCurrentLocale('ru')

    expect(await labels()).toEqual([
      'Жирный', 'Курсив', 'Код в строке', 'Ссылка', 'Заголовок', 'Маркированный список', 'Нумерованный список',
      'Цитата', 'Блок кода', 'Вставить таблицу',
    ])
    expect(await labels({ inTable: true })).toContain('Действия с таблицей')
    expect(await renderToString(createSSRApp(MarkdownToolbar))).toContain('Введите / для команд')
  })
})
