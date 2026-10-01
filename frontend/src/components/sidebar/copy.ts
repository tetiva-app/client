import type { Locale, PluralForms } from '@/lib/locale'

export interface TreeCopy {
  header: {
    import: string
    importFile: string
    importLink: string
    newCollection: string
    search: string
    clearSearch: string
    empty: string
    noMatches: string
    limitReached: string
  }
  menu: {
    newRequest: string
    newSubCollection: string
    rename: string
    openDetails: string
    checking: string
    importFile: string
    exportPostman: string
    moveTo: string
    delete: string
    deleteItems: PluralForms
  }
  deleteCollection: {
    title: string
    text: string
    published: string
  }
  deleteRequest: {
    title: string
    text: string
  }
  deleteSelected: {
    title: string
    text: string
    publishedOne: string
    published: PluralForms
  }
  rename: {
    collectionTitle: string
    requestTitle: string
    description: string
  }
  create: {
    requestTitle: string
    requestDescription: string
    collectionTitle: string
    collectionDescription: string
    subCollectionTitle: string
    subCollectionDescription: string
  }
  move: {
    title: PluralForms
    description: string
    root: string
  }
  names: {
    collection: string
    request: string
    generic: string
  }
  actions: {
    cancel: string
    save: string
    create: string
    delete: string
    move: string
  }
  results: {
    noWorkspace: string
    exported: string
    exportedWithWarnings: PluralForms
    moreWarnings: PluralForms
    exportFailed: string
    deleteFailed: string
  }
  rail: RailCopy
  publications: PublicationsPanelCopy
}

export type SyncStateKey =
  | 'connected' | 'pushing' | 'pulling' | 'subscribing' | 'offline' | 'resyncing'
  | 'disconnected' | 'idle' | 'auth_expired' | 'plan_limit' | 'update_required'

export interface RailCopy {
  collections: string
  environments: string
  history: string
  settings: string
  settingsUpdate: string
  sync: string
  syncState: Record<SyncStateKey, string>
}

export interface PublicationsPanelCopy {
  title: string
  outdated: PluralForms
  refresh: string
  loading: string
  publishCollection: string
  state: {
    ok: string
    changed: string
    unknown: string
    pending: string
  }
  tips: {
    ok: string
    changed: string
    environmentMissing: string
    unknown: string
    pending: string
  }
  update: string
  copyLink: string
  openPage: string
  lastKnown: string
  loadFailed: string
  signedOut: {
    title: string
    text: string
    action: string
  }
  noCapability: string
  empty: {
    sample: string
    title: string
    text: string
    audience: string
    secrets: string
    updates: string
  }
  quota: string
  plans: string
  cabinet: string
  picker: {
    title: string
    description: string
    search: string
    published: string
    none: string
    nothingFound: string
  }
}

export const TREE_COPY: Record<Locale, TreeCopy> = {
  en: {
    header: {
      import: 'Import',
      importFile: 'Import File…',
      importLink: 'Import from Link…',
      newCollection: 'New Collection',
      search: 'Search',
      clearSearch: 'Clear search',
      empty: 'No collections yet. Click + to create one.',
      noMatches: 'No matches',
      limitReached: 'Showing top 200 results. Refine query to narrow down.',
    },
    menu: {
      newRequest: 'New Request',
      newSubCollection: 'New Sub-Collection',
      rename: 'Rename',
      openDetails: 'Open Details',
      checking: 'Checking…',
      importFile: 'Import File…',
      exportPostman: 'Export as Postman',
      moveTo: 'Move to…',
      delete: 'Delete',
      deleteItems: { one: 'Delete {n} item', other: 'Delete {n} items' },
    },
    deleteCollection: {
      title: 'Delete collection',
      text: 'Delete "{name}" and all its contents? This action cannot be undone.',
      published: 'The collection is published — its page will be taken down.',
    },
    deleteRequest: {
      title: 'Delete request',
      text: 'Delete request "{name}"? This action cannot be undone.',
    },
    deleteSelected: {
      title: 'Delete selected items',
      text: 'Delete all selected items? This action cannot be undone.',
      publishedOne: 'A published collection is among them — its page will be taken down.',
      published: {
        one: '{n} published collection is among them — its page will be taken down.',
        other: '{n} published collections are among them — their pages will be taken down.',
      },
    },
    rename: {
      collectionTitle: 'Rename Collection',
      requestTitle: 'Rename Request',
      description: 'Enter a new name for "{name}".',
    },
    create: {
      requestTitle: 'New Request',
      requestDescription: 'Enter a name for the request.',
      collectionTitle: 'New Collection',
      collectionDescription: 'Enter a name for the collection.',
      subCollectionTitle: 'New Sub-Collection',
      subCollectionDescription: 'Enter a name for the sub-collection.',
    },
    move: {
      title: { one: 'Move {n} item to…', other: 'Move {n} items to…' },
      description: 'Select a destination collection.',
      root: 'Root (top level)',
    },
    names: {
      collection: 'Collection name',
      request: 'Request name',
      generic: 'Name',
    },
    actions: {
      cancel: 'Cancel',
      save: 'Save',
      create: 'Create',
      delete: 'Delete',
      move: 'Move',
    },
    results: {
      noWorkspace: 'No active workspace',
      exported: 'Exported to {path}',
      exportedWithWarnings: {
        one: 'Exported with {n} warning: {list}',
        other: 'Exported with {n} warnings: {list}',
      },
      moreWarnings: { one: 'and {n} more', other: 'and {n} more' },
      exportFailed: "Couldn't export: {detail}",
      deleteFailed: "Couldn't delete. Please try again.",
    },
    rail: {
      collections: 'Collections',
      environments: 'Environments',
      history: 'History',
      settings: 'Settings',
      settingsUpdate: 'Settings — Tetiva {version} is available',
      sync: 'Sync',
      syncState: {
        connected: 'Sync connected',
        pushing: 'Pushing changes...',
        pulling: 'Pulling updates...',
        subscribing: 'Connecting...',
        offline: 'Offline',
        resyncing: 'Resyncing...',
        disconnected: 'Not connected',
        idle: 'Idle',
        auth_expired: 'Session expired — sign in to resume sync',
        plan_limit: 'Sync paused — plan limit reached',
        update_required: 'Sync stopped — update the app to read the newest changes',
      },
    },
    publications: {
      title: 'Publications',
      outdated: { one: 'Publications — {n} out of date', other: 'Publications — {n} out of date' },
      refresh: 'Refresh',
      loading: 'Loading publications',
      publishCollection: 'Publish a collection…',
      state: {
        ok: 'up to date',
        changed: 'has changes',
        unknown: 'not checked',
        pending: 'being taken down',
      },
      tips: {
        ok: 'Matches the page · updated {when} · version {n}',
        changed: 'The page still shows version {n}',
        environmentMissing: "Can't compare: the environment used to publish isn't on this device",
        unknown: "Can't compare with the page",
        pending: 'The page is being taken down',
      },
      update: 'Update…',
      copyLink: 'Copy link',
      openPage: 'Open page',
      lastKnown: 'Showing the last known state',
      loadFailed: "Couldn't load the list",
      signedOut: {
        title: 'Sign in to see your publications',
        text: 'Pages on share.tetiva.app belong to your account. Published pages stay online while you are signed out.',
        action: 'Sign in',
      },
      noCapability: "This server doesn't publish pages",
      empty: {
        sample: 'My API',
        title: 'Publish a collection as a page',
        text: "A page on share.tetiva.app with request docs, response examples and code snippets. Readers don't need an account.",
        audience: 'Open to anyone, by link or by password',
        secrets: 'Secret variables, cookies and OAuth tokens stay on your device',
        updates: 'The page changes only when you update it',
      },
      quota: 'Your last publication was refused: the Free plan limit.',
      plans: 'See plans',
      cabinet: 'Pages in the web cabinet',
      picker: {
        title: 'Publish a collection',
        description: 'Top-level collections of this workspace can be published.',
        search: 'Find a collection',
        published: 'published',
        none: 'No collections yet',
        nothingFound: 'Nothing found',
      },
    },
  },
  ru: {
    header: {
      import: 'Импорт',
      importFile: 'Импорт из файла…',
      importLink: 'Импорт по ссылке…',
      newCollection: 'Новая коллекция',
      search: 'Поиск',
      clearSearch: 'Очистить поиск',
      empty: 'Коллекций пока нет. Нажмите +, чтобы создать.',
      noMatches: 'Ничего не найдено',
      limitReached: 'Показаны первые 200 результатов. Уточните запрос.',
    },
    menu: {
      newRequest: 'Новый запрос',
      newSubCollection: 'Новая подколлекция',
      rename: 'Переименовать',
      openDetails: 'Открыть коллекцию',
      checking: 'Проверяем…',
      importFile: 'Импорт из файла…',
      exportPostman: 'Экспорт в Postman',
      moveTo: 'Переместить…',
      delete: 'Удалить',
      deleteItems: {
        one: 'Удалить {n} элемент',
        few: 'Удалить {n} элемента',
        many: 'Удалить {n} элементов',
        other: 'Удалить {n} элемента',
      },
    },
    deleteCollection: {
      title: 'Удалить коллекцию',
      text: 'Удалить «{name}» со всем содержимым? Это действие нельзя отменить.',
      published: 'Коллекция опубликована\u00a0— её страница будет снята.',
    },
    deleteRequest: {
      title: 'Удалить запрос',
      text: 'Удалить запрос «{name}»? Это действие нельзя отменить.',
    },
    deleteSelected: {
      title: 'Удалить выбранное',
      text: 'Удалить все выбранные элементы? Это действие нельзя отменить.',
      publishedOne: 'Среди них есть опубликованная коллекция\u00a0— её страница будет снята.',
      published: {
        one: 'Среди них {n} опубликованная коллекция\u00a0— их страницы будут сняты.',
        few: 'Среди них {n} опубликованные коллекции\u00a0— их страницы будут сняты.',
        many: 'Среди них {n} опубликованных коллекций\u00a0— их страницы будут сняты.',
        other: 'Среди них {n} опубликованные коллекции\u00a0— их страницы будут сняты.',
      },
    },
    rename: {
      collectionTitle: 'Переименовать коллекцию',
      requestTitle: 'Переименовать запрос',
      description: 'Введите новое название для «{name}».',
    },
    create: {
      requestTitle: 'Новый запрос',
      requestDescription: 'Введите название запроса.',
      collectionTitle: 'Новая коллекция',
      collectionDescription: 'Введите название коллекции.',
      subCollectionTitle: 'Новая подколлекция',
      subCollectionDescription: 'Введите название подколлекции.',
    },
    move: {
      title: {
        one: 'Переместить {n} элемент в…',
        few: 'Переместить {n} элемента в…',
        many: 'Переместить {n} элементов в…',
        other: 'Переместить {n} элемента в…',
      },
      description: 'Выберите коллекцию назначения.',
      root: 'Верхний уровень',
    },
    names: {
      collection: 'Название коллекции',
      request: 'Название запроса',
      generic: 'Название',
    },
    actions: {
      cancel: 'Отмена',
      save: 'Сохранить',
      create: 'Создать',
      delete: 'Удалить',
      move: 'Переместить',
    },
    results: {
      noWorkspace: 'Нет активного пространства',
      exported: 'Экспортировано в {path}',
      exportedWithWarnings: {
        one: 'Экспортировано, {n} предупреждение: {list}',
        few: 'Экспортировано, {n} предупреждения: {list}',
        many: 'Экспортировано, {n} предупреждений: {list}',
        other: 'Экспортировано, {n} предупреждения: {list}',
      },
      moreWarnings: { one: 'и ещё {n}', few: 'и ещё {n}', many: 'и ещё {n}', other: 'и ещё {n}' },
      exportFailed: 'Не удалось экспортировать: {detail}',
      deleteFailed: 'Не удалось удалить. Попробуйте ещё раз.',
    },
    rail: {
      collections: 'Коллекции',
      environments: 'Окружения',
      history: 'История',
      settings: 'Настройки',
      settingsUpdate: 'Настройки\u00a0— доступна Tetiva\u00a0{version}',
      sync: 'Синхронизация',
      syncState: {
        connected: 'Синхронизация включена',
        pushing: 'Отправляем изменения…',
        pulling: 'Получаем обновления…',
        subscribing: 'Подключаемся…',
        offline: 'Нет связи',
        resyncing: 'Синхронизируем заново…',
        disconnected: 'Не подключено',
        idle: 'Ожидание',
        auth_expired: 'Сессия истекла\u00a0— войдите, чтобы продолжить синхронизацию',
        plan_limit: 'Синхронизация на паузе\u00a0— достигнут лимит тарифа',
        update_required: 'Синхронизация остановлена\u00a0— обновите приложение, чтобы получить новые изменения',
      },
    },
    publications: {
      title: 'Публикации',
      outdated: {
        one: 'Публикации\u00a0— {n} устарела',
        few: 'Публикации\u00a0— {n} устарели',
        many: 'Публикации\u00a0— {n} устарели',
        other: 'Публикации\u00a0— {n} устарели',
      },
      refresh: 'Обновить список',
      loading: 'Загружаем публикации',
      publishCollection: 'Опубликовать коллекцию…',
      state: {
        ok: 'актуальна',
        changed: 'есть изменения',
        unknown: 'не проверено',
        pending: 'снимается',
      },
      tips: {
        ok: 'Совпадает со страницей · обновлена {when} · версия {n}',
        changed: 'На странице всё ещё версия {n}',
        environmentMissing: 'Не сравнить: окружения, с которым публиковали, нет на этом устройстве',
        unknown: 'Не сравнить со страницей',
        pending: 'Страница снимается с публикации',
      },
      update: 'Обновить…',
      copyLink: 'Копировать ссылку',
      openPage: 'Открыть страницу',
      lastKnown: 'Показано последнее известное состояние',
      loadFailed: 'Не удалось загрузить список',
      signedOut: {
        title: 'Войдите, чтобы видеть публикации',
        text: 'Страницы на share.tetiva.app принадлежат аккаунту. Опубликованные страницы работают и без входа.',
        action: 'Войти',
      },
      noCapability: 'Этот сервер не публикует страницы',
      empty: {
        sample: 'Мой API',
        title: 'Опубликуйте коллекцию как страницу',
        text: 'Страница на share.tetiva.app: запросы с описаниями, примеры ответов и сниппеты. Читателям не нужен аккаунт.',
        audience: 'Открыта всем, по ссылке или по паролю',
        secrets: 'Секретные переменные, cookies и OAuth-токены остаются на устройстве',
        updates: 'Страница меняется, только когда вы её обновите',
      },
      quota: 'Последняя публикация отклонена: лимит бесплатного плана.',
      plans: 'Тарифы',
      cabinet: 'Страницы в кабинете',
      picker: {
        title: 'Опубликовать коллекцию',
        description: 'Публикуются коллекции верхнего уровня этого пространства.',
        search: 'Найти коллекцию',
        published: 'опубликована',
        none: 'Коллекций пока нет',
        nothingFound: 'Ничего не найдено',
      },
    },
  },
}
