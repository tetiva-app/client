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
  scripts: {
    pre: string
    post: string
    preHint: string
    postHint: string
    preConnect: string
    preConnectHint: string
  }
  saveExample: {
    button: string
    buttonHint: string
    namePlaceholder: string
    nameHint: string
    save: string
    saveHint: string
    cancel: string
    cancelHint: string
    saved: string
    secretsTitle: string
    secretsText: string
    saveAnyway: string
    blocked: {
      binary: string
      tooLarge: string
      draft: string
    }
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
    scripts: {
      pre: 'Pre-request',
      post: 'Post-response',
      preHint: 'Runs before each request',
      postHint: 'Runs after each response',
      preConnect: 'Pre-connect',
      preConnectHint: 'Runs before each connection',
    },
    saveExample: {
      button: 'Save as example',
      buttonHint: 'Save this response as an example',
      namePlaceholder: 'Example name',
      nameHint: 'Press Enter to save or Escape to cancel',
      save: 'Save',
      saveHint: 'Save (Enter)',
      cancel: 'Cancel',
      cancelHint: 'Cancel (Esc)',
      saved: 'Saved as example',
      secretsTitle: 'This example may contain secrets',
      secretsText: 'Found: {labels}. Credential headers are masked; the body and other headers are saved as they are.',
      saveAnyway: 'Save anyway',
      blocked: {
        binary: "Binary responses can't be saved as examples",
        tooLarge: 'Response is too large for an example (max 256 KB)',
        draft: 'Save the request first to keep examples',
      },
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
    scripts: {
      pre: 'Перед запросом',
      post: 'После ответа',
      preHint: 'Выполняется перед каждым запросом',
      postHint: 'Выполняется после каждого ответа',
      preConnect: 'Перед подключением',
      preConnectHint: 'Выполняется перед каждым подключением',
    },
    saveExample: {
      button: 'В примеры',
      buttonHint: 'Сохранить ответ как пример',
      namePlaceholder: 'Название примера',
      nameHint: 'Enter\u00a0— сохранить, Esc\u00a0— отменить',
      save: 'Сохранить',
      saveHint: 'Сохранить (Enter)',
      cancel: 'Отмена',
      cancelHint: 'Отмена (Esc)',
      saved: 'Ответ сохранён в примеры',
      secretsTitle: 'В примере могут быть секреты',
      secretsText: 'Найдено: {labels}. Заголовки с учётными данными скрыты, тело и остальные заголовки сохранятся как есть.',
      saveAnyway: 'Всё равно сохранить',
      blocked: {
        binary: 'Двоичный ответ нельзя сохранить как пример',
        tooLarge: 'Ответ слишком большой для примера (не больше 256\u00a0КБ)',
        draft: 'Сначала сохраните запрос, чтобы хранить к нему примеры',
      },
    },
  },
}
