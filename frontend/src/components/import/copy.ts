import type { Locale, PluralForms } from '@/lib/locale'
import type { ImportFormat } from '@/services'

export type ImportErrorReason =
  | 'LINK_NOT_FOUND'
  | 'PASSWORD_REQUIRED'
  | 'PASSWORD_INVALID'
  | 'RATE_LIMITED'
  | 'SNAPSHOT_TOO_LARGE'
  | 'UPDATE_REQUIRED'
  | 'PREVIEW_EXPIRED'
  | 'UNSUPPORTED_FILE'
  | 'COLLECTION_FILE'
  | 'FILE_TOO_LARGE'

interface OneOrSeveral {
  single: string
  several: string
}

export interface ImportCopy {
  link: {
    title: string
    passwordTitle: string
    description: string
    passwordDescription: string
    linkLabel: string
    password: string
    cancel: string
    close: string
    unlock: string
    continue: string
  }
  confirm: {
    title: string
    untitled: string
    kinds: Record<ImportFormat, string>
    destination: string
    workspace: string
    folder: string
    alwaysTopLevel: string
    inheritedAuth: string
    shared: string
    sharedWithEnvironments: string
    environmentHeading: OneOrSeveral
    openapiEnvironment: string
    insomniaEnvironments: OneOrSeveral
    collectionVariables: OneOrSeveral
    tetivaEnvironment: string
    hosts: string
    changed: string
    scripts: string
    scriptCount: PluralForms
    phase: { pre: string; post: string }
    includeScripts: string
    scriptsHint: string
    cancel: string
    import: string
  }
  counts: {
    folders: PluralForms
    requests: PluralForms
    examples: PluralForms
    variables: PluralForms
  }
  environments: OneOrSeveral
  quoted: string
  busy: {
    checking: string
    unlocking: string
    downloading: string
    reading: string
    importingEnvironment: string
  }
  errors: {
    reasons: Record<ImportErrorReason, string>
    unreachable: string
    unreadable: string
    unreadableDetail: string
    snapshotFileTooLarge: string
    expiredLink: string
    badLink: string
    enterPassword: string
    noWorkspace: string
    readFailed: string
    importInProgress: string
  }
  results: {
    imported: string
    importedEnvironments: OneOrSeveral
    activeEnvironment: string
    open: string
    moreWarnings: PluralForms
    downloadedAgain: string
  }
}

export const IMPORT_COPY: Record<Locale, ImportCopy> = {
  en: {
    link: {
      title: 'Import from link',
      passwordTitle: 'Password required',
      description: 'Paste a link to a collection published on share.tetiva.app.',
      passwordDescription: '“{title}” is protected. Enter the password you got with the link.',
      linkLabel: 'Link or slug',
      password: 'Password',
      cancel: 'Cancel',
      close: 'Close',
      unlock: 'Unlock',
      continue: 'Continue',
    },
    confirm: {
      title: 'Import “{title}”',
      untitled: 'Untitled',
      kinds: {
        tetiva: 'Tetiva collection',
        postman: 'Postman collection',
        insomnia: 'Insomnia collection',
        openapi: 'OpenAPI spec',
        hoppscotch: 'Hoppscotch collection',
        har: 'HAR file',
      },
      destination: 'Destination',
      workspace: 'Workspace “{name}”',
      folder: ', folder “{name}”',
      alwaysTopLevel: 'Tetiva collections are always imported as a new top-level collection.',
      inheritedAuth: "Requests without their own authorization will use {name}'s authorization.",
      shared: 'This collection will be shared with everyone in {workspace}.',
      sharedWithEnvironments: 'This collection and the {environments} will be shared with everyone in {workspace}.',
      environmentHeading: { single: 'Environment', several: 'Environments' },
      openapiEnvironment: 'The server address, path parameters and credentials go to the {environments}, '
        + 'which becomes active. Secret values start empty.',
      insomniaEnvironments: {
        single: 'Adds the {environments}. Switch to it in the environment picker to use it — only one environment is active at a time.',
        several: 'Adds the {environments}. Switch to one in the environment picker to use it — only one environment is active at a time.',
      },
      collectionVariables: {
        single: 'Collection variables become the {environments}. '
          + 'Switch to it in the environment picker to use them — only one environment is active at a time.',
        several: 'Collection variables become the {environments}, one per collection. '
          + 'Switch to one in the environment picker to use them — only one environment is active at a time.',
      },
      tetivaEnvironment: 'Adds the {environments}. Secret values stay empty.',
      hosts: 'Sends requests to',
      changed: 'Changed on import · {n}',
      scripts: 'Scripts',
      scriptCount: { one: 'This collection contains {n} script', other: 'This collection contains {n} scripts' },
      phase: { pre: 'Pre-request', post: 'Post-response' },
      includeScripts: 'Import scripts',
      scriptsHint: 'Scripts run in a sandbox when you send requests and can change your environment variables',
      cancel: 'Cancel',
      import: 'Import',
    },
    counts: {
      folders: { one: '{n} folder', other: '{n} folders' },
      requests: { one: '{n} request', other: '{n} requests' },
      examples: { one: '{n} example', other: '{n} examples' },
      variables: { one: '{n} variable', other: '{n} variables' },
    },
    environments: { single: 'environment {names}', several: 'environments {names}' },
    quoted: '“{name}”',
    busy: {
      checking: 'Checking the link…',
      unlocking: 'Unlocking…',
      downloading: 'Downloading the collection…',
      reading: 'Reading the file…',
      importingEnvironment: 'Importing the environment…',
    },
    errors: {
      reasons: {
        LINK_NOT_FOUND: "This collection isn't published, or the link is wrong",
        PASSWORD_REQUIRED: 'Enter the password to import this collection',
        PASSWORD_INVALID: 'Wrong password',
        RATE_LIMITED: 'Too many attempts — try again in a minute',
        SNAPSHOT_TOO_LARGE: 'The collection is larger than 8 MiB',
        UPDATE_REQUIRED: 'Update Tetiva to import this collection',
        PREVIEW_EXPIRED: 'The download expired — import from the link again',
        UNSUPPORTED_FILE: "This file isn't a Tetiva, Postman, Insomnia or Hoppscotch collection, an OpenAPI spec or a HAR file",
        COLLECTION_FILE: 'This is a collection, not an environment — import it from the sidebar',
        FILE_TOO_LARGE: 'The file is larger than 48 MiB',
      },
      unreachable: "Can't reach the server — try again",
      unreadable: "The collection can't be read",
      unreadableDetail: "The collection can't be read: {detail}",
      snapshotFileTooLarge: 'The file is larger than 8 MiB',
      expiredLink: 'The link has expired — open it again from the page',
      badLink: 'Paste a share.tetiva.app link or a collection slug',
      enterPassword: 'Enter the password',
      noWorkspace: 'No active workspace',
      readFailed: "Couldn't read the file",
      importInProgress: 'Finish the current import first',
    },
    results: {
      imported: 'Imported “{title}”: {counts}',
      importedEnvironments: { single: 'Imported {environments} · {variables}', several: 'Imported {environments} · {variables}' },
      activeEnvironment: 'environment {name} is now active',
      open: 'Open',
      moreWarnings: { one: 'and {n} more', other: 'and {n} more' },
      downloadedAgain: 'The download expired, so the collection was downloaded again. Review it and import.',
    },
  },
  ru: {
    link: {
      title: 'Импорт по ссылке',
      passwordTitle: 'Нужен пароль',
      description: 'Вставьте ссылку на коллекцию, опубликованную на share.tetiva.app.',
      passwordDescription: 'Коллекция «{title}» защищена. Введите пароль, который вы получили вместе со ссылкой.',
      linkLabel: 'Ссылка или код коллекции',
      password: 'Пароль',
      cancel: 'Отмена',
      close: 'Закрыть',
      unlock: 'Открыть',
      continue: 'Продолжить',
    },
    confirm: {
      title: 'Импорт «{title}»',
      untitled: 'Без названия',
      kinds: {
        tetiva: 'Коллекция Tetiva',
        postman: 'Коллекция Postman',
        insomnia: 'Коллекция Insomnia',
        openapi: 'Спецификация OpenAPI',
        hoppscotch: 'Коллекция Hoppscotch',
        har: 'HAR-файл',
      },
      destination: 'Куда импортировать',
      workspace: 'Пространство «{name}»',
      folder: ', папка «{name}»',
      alwaysTopLevel: 'Коллекции Tetiva всегда импортируются отдельной коллекцией верхнего уровня.',
      inheritedAuth: 'Запросы без собственной авторизации унаследуют её от «{name}».',
      shared: 'Коллекция станет доступна всем в пространстве «{workspace}».',
      sharedWithEnvironments: 'Коллекция и {environments} станут доступны всем в пространстве «{workspace}».',
      environmentHeading: { single: 'Окружение', several: 'Окружения' },
      openapiEnvironment: 'Адрес сервера, параметры пути и учётные данные попадут в {environments}, '
        + 'и оно станет активным. Секретные значения в нём пока пустые.',
      insomniaEnvironments: {
        single: 'Импорт добавит {environments}. Чтобы им пользоваться, выберите его в списке окружений\u00a0— '
          + 'одновременно активно только одно окружение.',
        several: 'Импорт добавит {environments}. Чтобы пользоваться одним из них, выберите его в списке окружений\u00a0— '
          + 'одновременно активно только одно окружение.',
      },
      collectionVariables: {
        single: 'Переменные коллекции попадут в {environments}. Чтобы ими пользоваться, выберите его в списке окружений\u00a0— '
          + 'одновременно активно только одно окружение.',
        several: 'Переменные коллекций попадут в {environments}, по одному на коллекцию. '
          + 'Чтобы ими пользоваться, выберите одно из них в списке окружений\u00a0— одновременно активно только одно окружение.',
      },
      tetivaEnvironment: 'Импорт добавит {environments}. Секретные значения останутся пустыми.',
      hosts: 'Куда уходят запросы',
      changed: 'Изменено при импорте · {n}',
      scripts: 'Скрипты',
      scriptCount: {
        one: 'В коллекции {n} скрипт',
        few: 'В коллекции {n} скрипта',
        many: 'В коллекции {n} скриптов',
        other: 'В коллекции {n} скрипта',
      },
      phase: { pre: 'До запроса', post: 'После ответа' },
      includeScripts: 'Импортировать скрипты',
      scriptsHint: 'Скрипты выполняются в песочнице при отправке запросов и могут менять переменные окружения',
      cancel: 'Отмена',
      import: 'Импортировать',
    },
    counts: {
      folders: { one: '{n} папка', few: '{n} папки', many: '{n} папок', other: '{n} папки' },
      requests: { one: '{n} запрос', few: '{n} запроса', many: '{n} запросов', other: '{n} запроса' },
      examples: { one: '{n} пример', few: '{n} примера', many: '{n} примеров', other: '{n} примера' },
      variables: { one: '{n} переменная', few: '{n} переменные', many: '{n} переменных', other: '{n} переменные' },
    },
    environments: { single: 'окружение {names}', several: 'окружения {names}' },
    quoted: '«{name}»',
    busy: {
      checking: 'Проверяем ссылку…',
      unlocking: 'Проверяем пароль…',
      downloading: 'Скачиваем коллекцию…',
      reading: 'Читаем файл…',
      importingEnvironment: 'Импортируем окружение…',
    },
    errors: {
      reasons: {
        LINK_NOT_FOUND: 'Коллекция не опубликована, или ссылка неверна',
        PASSWORD_REQUIRED: 'Введите пароль, чтобы импортировать коллекцию',
        PASSWORD_INVALID: 'Неверный пароль',
        RATE_LIMITED: 'Слишком много попыток\u00a0— попробуйте через минуту',
        SNAPSHOT_TOO_LARGE: 'Коллекция больше 8 МБ',
        UPDATE_REQUIRED: 'Обновите Tetiva, чтобы импортировать эту коллекцию',
        PREVIEW_EXPIRED: 'Срок скачанной копии истёк\u00a0— импортируйте по ссылке ещё раз',
        UNSUPPORTED_FILE: 'Это не коллекция Tetiva, Postman, Insomnia или Hoppscotch, не спецификация OpenAPI и не HAR-файл',
        COLLECTION_FILE: 'Это коллекция, а не окружение\u00a0— импортируйте её из боковой панели',
        FILE_TOO_LARGE: 'Файл больше 48 МБ',
      },
      unreachable: 'Сервер недоступен\u00a0— попробуйте снова',
      unreadable: 'Не удалось прочитать коллекцию',
      unreadableDetail: 'Не удалось прочитать коллекцию: {detail}',
      snapshotFileTooLarge: 'Файл больше 8 МБ',
      expiredLink: 'Ссылка устарела\u00a0— откройте её снова со страницы',
      badLink: 'Вставьте ссылку на share.tetiva.app или код коллекции',
      enterPassword: 'Введите пароль',
      noWorkspace: 'Нет активного пространства',
      readFailed: 'Не удалось прочитать файл',
      importInProgress: 'Сначала завершите текущий импорт',
    },
    results: {
      imported: 'Импортирована коллекция «{title}»: {counts}',
      importedEnvironments: {
        single: 'Импортировано {environments} · {variables}',
        several: 'Импортированы {environments} · {variables}',
      },
      activeEnvironment: 'окружение {name} теперь активно',
      open: 'Открыть',
      moreWarnings: { one: 'и ещё {n}', few: 'и ещё {n}', many: 'и ещё {n}', other: 'и ещё {n}' },
      downloadedAgain: 'Срок скачанной копии истёк, поэтому коллекцию скачали заново. Проверьте её и импортируйте.',
    },
  },
}
