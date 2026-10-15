import type { Locale, PluralForms } from '@/lib/locale'

export interface EditorCopy {
  overview: {
    requests: PluralForms
    folders: PluralForms
    empty: string
    emptyHint: string
    placeholder: string
  }
  description: {
    label: string
    preview: string
    previewLabel: string
    edit: string
    editLabel: string
    add: string
    counter: PluralForms
    overLimit: string
  }
  docs: {
    placeholder: string
  }
  toolbar: {
    bold: string
    italic: string
    code: string
    link: string
    heading: string
    bulletList: string
    numberedList: string
    quote: string
    codeBlock: string
    insertTable: string
    tableActions: string
    typeSlash: string
  }
  tabs: {
    collection: string
  }
}

export const EDITOR_COPY: Record<Locale, EditorCopy> = {
  en: {
    overview: {
      requests: { one: '{n} request', other: '{n} requests' },
      folders: { one: '{n} folder', other: '{n} folders' },
      empty: 'Empty collection',
      emptyHint: 'Add requests or folders from the sidebar context menu',
      placeholder: 'Add a description for this collection (supports Markdown)...',
    },
    description: {
      label: 'Description',
      preview: 'Preview',
      previewLabel: 'Preview description',
      edit: 'Edit',
      editLabel: 'Edit description',
      add: 'Add description',
      counter: { one: '{used} / {n} byte', other: '{used} / {n} bytes' },
      overLimit: '{counter} — over the limit, saves only when shortened',
    },
    docs: {
      placeholder: 'Document this request: what it does, required params, gotchas (supports Markdown)...',
    },
    toolbar: {
      bold: 'Bold',
      italic: 'Italic',
      code: 'Inline code',
      link: 'Link',
      heading: 'Heading',
      bulletList: 'Bullet list',
      numberedList: 'Numbered list',
      quote: 'Quote',
      codeBlock: 'Code block',
      insertTable: 'Insert table',
      tableActions: 'Table actions',
      typeSlash: 'Type / for commands',
    },
    tabs: {
      collection: 'Collection',
    },
  },
  ru: {
    overview: {
      requests: { one: '{n} запрос', few: '{n} запроса', many: '{n} запросов', other: '{n} запроса' },
      folders: { one: '{n} папка', few: '{n} папки', many: '{n} папок', other: '{n} папки' },
      empty: 'Пустая коллекция',
      emptyHint: 'Добавьте запросы или папки через контекстное меню в боковой панели',
      placeholder: 'Опишите коллекцию (поддерживается Markdown)…',
    },
    description: {
      label: 'Описание',
      preview: 'Просмотр',
      previewLabel: 'Просмотр описания',
      edit: 'Изменить',
      editLabel: 'Изменить описание',
      add: 'Добавить описание',
      counter: { one: '{used} / {n} байт', few: '{used} / {n} байта', many: '{used} / {n} байт', other: '{used} / {n} байта' },
      overLimit: '{counter}\u00a0— больше лимита, сохранится только после сокращения',
    },
    docs: {
      placeholder: 'Опишите запрос: что он делает, какие параметры обязательны, какие есть подводные камни (поддерживается Markdown)…',
    },
    toolbar: {
      bold: 'Жирный',
      italic: 'Курсив',
      code: 'Код в строке',
      link: 'Ссылка',
      heading: 'Заголовок',
      bulletList: 'Маркированный список',
      numberedList: 'Нумерованный список',
      quote: 'Цитата',
      codeBlock: 'Блок кода',
      insertTable: 'Вставить таблицу',
      tableActions: 'Действия с таблицей',
      typeSlash: 'Введите / для команд',
    },
    tabs: {
      collection: 'Коллекция',
    },
  },
}
