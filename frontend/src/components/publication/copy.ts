import type { Locale, PluralForms } from '@/lib/locale'
import type { HiddenVar, UnavailableReason, Visibility } from '@/types/publication'

export interface PublicationCopy {
  visibility: Record<Visibility, string>
  unavailable: Record<Exclude<UnavailableReason, ''>, string>
  unlistedHint: string
  tabs: {
    overview: string
    authorization: string
    scripts: string
    publish: string
    changed: string
  }
  menu: {
    publish: string
    publication: string
  }
  toasts: {
    published: string
    updated: string
    copyLink: string
    linkCopied: string
    copyFailed: string
    openFailed: string
    offline: string
  }
  thumbnail: {
    caption: string
    open: string
    download: string
  }
  dialog: {
    titlePublish: string
    titleUpdate: string
    versionMove: string
    reopen: string
    intro: string
    checking: string
    signIn: string
    tryAgain: string
    signInText: string
    noCapabilityText: string
    offlineTitle: string
    offlineText: string
    loadFailedTitle: string
    loadFailed: string
    manage: string
    saveFailed: string
    visibility: string
    visibilities: Record<Visibility, { label: string; hint: string }>
    proOnly: string
    seePlans: string
    password: string
    passwordKeep: string
    passwordShow: string
    passwordHide: string
    passwordHelp: string
    passwordInvalid: string
    environment: string
    none: string
    environmentMissing: string
    environmentHint: string
    scripts: string
    includeScripts: string
    scriptsHint: string
    buildingPreview: string
    acknowledge: string
    warnings: PluralForms
    cancel: string
    update: string
    publish: string
    confirmPublicTitle: string
    confirmPublicText: string
    confirmPublicAction: string
  }
  preview: {
    title: string
    folders: PluralForms
    requests: PluralForms
    examples: PluralForms
    environment: string
    environmentNone: string
    scripts: string
    scriptsIncluded: string
    scriptsLeftOut: string
    mustFix: string
    variables: string
    published: string
    hidden: string
    hiddenReasons: Record<HiddenVar['reason'], string>
    publishAsIs: string
    makeSecret: string
    neverPublished: string
    removed: string
    always: string
    warnings: string
    publishedAsWritten: string
    replaced: string
    ignored: PluralForms
    size: string
    snapshot: string
    compressed: string
    sizeOf: string
    overLimit: string
    noBlocking: string
    rules: Record<string, string>
    redactions: Record<string, string>
    blocking: Record<string, string>
    lists: Record<string, string>
  }
  panel: {
    thisCollection: string
    loadFailed: string
    tryAgain: string
    checking: string
    emptyTitle: string
    emptyText: string
    features: Record<'audience' | 'environment' | 'secrets', { title: string; text: string }>
    publish: string
    signInToPublish: string
    published: string
    updated: string
    version: string
    refresh: string
    reviewUpdate: string
    update: string
    copy: string
    open: string
    signInToManage: string
    signIn: string
    unpublishFailed: string
    unpublishing: string
    blocked: string
    blockedFor: string
    notRoot: string
    changed: string
    stillShows: string
    counters: Record<'views' | 'imports' | 'downloads', string>
    settings: string
    visibility: string
    environment: string
    none: string
    notOnDevice: string
    scripts: string
    included: string
    leftOut: string
    publishedAsIs: string
    unpublishTitle: string
    unpublishText: string
    unpublishButton: string
    readOnly: string
    confirmTitle: string
    confirmText: string
    confirmAction: string
    cancel: string
  }
}

export const PUBLICATION_COPY: Record<Locale, PublicationCopy> = {
  en: {
    visibility: { public: 'Public', unlisted: 'Unlisted', password: 'Password' },
    unavailable: {
      not_logged_in: 'Sign in to publish',
      no_capability: "This server doesn't support publishing",
      not_root: 'Only top-level collections can be published',
      offline: 'Offline — showing the last known state',
    },
    unlistedHint: "Want to see it first? Publish as an Unlisted link — it won't show up in search, and you can switch to Public later.",
    tabs: {
      overview: 'Overview',
      authorization: 'Authorization',
      scripts: 'Scripts',
      publish: 'Publish',
      changed: 'Changed since publication',
    },
    menu: { publish: 'Publish…', publication: 'Publication…' },
    toasts: {
      published: 'Published',
      updated: 'Publication updated',
      copyLink: 'Copy link',
      linkCopied: 'Link copied',
      copyFailed: "Couldn't copy the link",
      openFailed: "Couldn't open the link",
      offline: 'The page is offline',
    },
    thumbnail: { caption: 'How the page looks', open: 'Open in Tetiva', download: 'Download' },
    dialog: {
      titlePublish: 'Publish “{name}”',
      titleUpdate: 'Update publication · {name}',
      versionMove: 'version {from} → {to}',
      reopen: 'The page will open again at its previous link.',
      intro: 'A read-only page on share.tetiva.app. The link is created when you publish.',
      checking: 'Checking the collection…',
      signIn: 'Sign in',
      tryAgain: 'Try again',
      signInText: 'Pages on share.tetiva.app belong to your Tetiva account.',
      noCapabilityText: 'Connect to a server with public pages to publish this collection.',
      offlineTitle: "Can't reach the server",
      offlineText: 'Check your connection and try again.',
      loadFailedTitle: "Couldn't check the publication",
      loadFailed: "Couldn't load the publication status",
      manage: "You can't manage this publication",
      saveFailed: "Couldn't save your latest changes. Fix them and try again",
      visibility: 'Visibility',
      visibilities: {
        public: { label: 'Public', hint: 'Anyone can open the page. Search engines may index it.' },
        unlisted: { label: 'Unlisted link', hint: "Only people with the link can open the page. It isn't indexed." },
        password: { label: 'Password', hint: "Readers enter a password to open the page. It isn't indexed." },
      },
      proOnly: 'Available on Pro.',
      seePlans: 'See plans',
      password: 'Password',
      passwordKeep: 'Leave empty to keep the current password',
      passwordShow: 'Show password',
      passwordHide: 'Hide password',
      passwordHelp: '8–72 bytes. A new password locks out readers who unlocked the page with the old one.',
      passwordInvalid: 'Password must be 8–72 bytes',
      environment: 'Environment',
      none: 'None',
      environmentMissing: "The environment used last time isn't on this device. Choose again.",
      environmentHint: 'Its non-secret variables are published with the page.',
      scripts: 'Scripts',
      includeScripts: 'Include scripts',
      scriptsHint: "Scripts may contain secrets. They're published as written.",
      buildingPreview: 'Building the preview…',
      acknowledge: "I've checked this",
      warnings: { one: '{n} warning', other: '{n} warnings' },
      cancel: 'Cancel',
      update: 'Update publication',
      publish: 'Publish',
      confirmPublicTitle: 'Make the page public?',
      confirmPublicText: 'The page will become public and searchable.',
      confirmPublicAction: 'Make public',
    },
    preview: {
      title: 'Preview',
      folders: { one: '{n} folder', other: '{n} folders' },
      requests: { one: '{n} request', other: '{n} requests' },
      examples: { one: '{n} example', other: '{n} examples' },
      environment: 'Environment',
      environmentNone: 'none',
      scripts: 'Scripts',
      scriptsIncluded: 'included',
      scriptsLeftOut: 'left out',
      mustFix: 'Must be fixed before publishing · {n}',
      variables: 'Variables · {name}',
      published: 'Published · {n}',
      hidden: 'Hidden · {n} — published as {token} with an empty value',
      hiddenReasons: {
        secret: 'secret',
        referenced: 'referenced from a secret field',
        suspicious: 'name looks like a secret',
      },
      publishAsIs: 'Publish as is',
      makeSecret: 'Make secret',
      neverPublished: 'never published',
      removed: 'Removed from the page · {n}',
      always: 'always',
      warnings: 'Warnings · {n}',
      publishedAsWritten: 'Published as written.',
      replaced: 'Replaced with <redacted> unless you publish it as is.',
      ignored: {
        one: '{n} earlier “Publish as is” choice no longer matches anything and will be dropped.',
        other: '{n} earlier “Publish as is” choices no longer match anything and will be dropped.',
      },
      size: 'Size',
      snapshot: 'Snapshot',
      compressed: 'Compressed',
      sizeOf: '{used} of {limit}',
      overLimit: 'The collection is over the size limit. Remove large response examples and try again.',
      noBlocking: 'No blocking errors',
      rules: {
        'jwt': 'JWT',
        'aws-access-key': 'AWS access key',
        'github-token': 'GitHub token',
        'slack-token': 'Slack token',
        'stripe-key': 'Stripe secret key',
        'google-api-key': 'Google API key',
        'telegram-bot-token': 'Telegram bot token',
        'bearer-token': 'Bearer token',
        'oauth-token-field': 'OAuth token',
        'private-key': 'Private key',
        'json-secret-key': 'Secret JSON field',
        'graphql-secret-argument': 'Secret GraphQL argument',
        'xml-secret-field': 'Secret XML field',
        'form-secret-field': 'Secret form field',
        'url-secret-parameter': 'Secret URL parameter',
      },
      // Keyed by Redaction.category (internal/domain/usecase/publication/selectors.go).
      redactions: {
        var: 'referenced from a secret field',
        auth: 'secret auth field; keep its value in a secret variable',
        header: 'sensitive header',
        metadata: 'sensitive metadata',
        query: 'sensitive query parameter',
        url: 'credentials in the URL',
        form: 'sensitive form field',
        file: 'only the file name is published',
        script: 'scripts are left out',
        cookie: 'cookies are never published',
      },
      // Keyed by BlockingError.code (internal/domain/usecase/publication/limits.go).
      blocking: {
        text_too_long: 'text longer than 1 MiB',
        value_too_long: 'value longer than {limit} KiB',
        name_too_long: 'name longer than {limit} characters',
        header_name_invalid: 'header name "{name}" may contain only token characters and {{variable}} references',
        url_too_long: 'URL longer than 8 KiB',
        url_control_char: 'URL contains a control character',
        name_blank: 'collection name is blank',
        too_many_values: '{count} JSON values; at most {limit} can be published',
        too_many: '{count} {list}; at most {limit} can be published',
        auth_too_deep: 'nested deeper than {limit} levels',
        auth_too_many_values: '{count} values inside; at most {limit} can be published',
        too_many_items: '{count} folders and requests; at most {limit} can be published',
        folders_too_deep: 'folders are nested deeper than {limit} levels',
        method_unsupported: 'HTTP method "{value}" cannot be published',
        protocol_unsupported: 'protocol "{value}" cannot be published',
        body_type_unsupported: 'body type "{value}" cannot be published',
        status_out_of_range: 'status {value} is outside 0–999',
        auth_type_unsupported: 'auth type "{value}" cannot be published',
      },
      lists: {
        headers: 'headers',
        metadata: 'metadata entries',
        form_fields: 'form fields',
        subprotocols: 'subprotocols',
        messages: 'messages',
        examples: 'examples',
        variables: 'variables',
        auth_fields: 'auth fields',
      },
    },
    panel: {
      thisCollection: 'this collection',
      loadFailed: "Couldn't check the publication",
      tryAgain: 'Try again',
      checking: 'Checking the publication…',
      emptyTitle: 'Publish “{name}” as a public page',
      emptyText: 'A read-only page on share.tetiva.app with request docs, response examples and code snippets. '
        + "Readers open it in Tetiva or download the collection. They don't need an account.",
      features: {
        audience: { title: 'You choose who opens it', text: 'Anyone, people with the link, or people with a password' },
        environment: { title: 'Pick an environment', text: 'Its non-secret variables go to the page' },
        secrets: {
          title: 'Secrets stay on your device',
          text: 'Secret variables, cookies and OAuth tokens are never published. You review everything before it goes out',
        },
      },
      publish: 'Publish…',
      signInToPublish: 'Sign in to publish',
      published: 'Published',
      updated: 'Updated {when} · version {n}',
      version: 'version {n}',
      refresh: 'Refresh',
      reviewUpdate: 'Review & update…',
      update: 'Update publication…',
      copy: 'Copy',
      open: 'Open',
      signInToManage: 'Sign in to update or unpublish the page',
      signIn: 'Sign in',
      unpublishFailed: "Couldn't unpublish: {error}",
      unpublishing: 'Unpublishing…',
      blocked: 'Blocked by the platform',
      blockedFor: 'Blocked by the platform: {reason}',
      notRoot: 'This collection is no longer top-level — it will be unpublished',
      changed: 'Changed since publication.',
      stillShows: 'The page still shows version {n}.',
      counters: { views: 'Views', imports: 'Opened in Tetiva', downloads: 'Downloads' },
      settings: 'Publication settings',
      visibility: 'Visibility',
      environment: 'Environment',
      none: 'None',
      notOnDevice: 'not on this device',
      scripts: 'Scripts',
      included: 'Included',
      leftOut: 'Left out',
      publishedAsIs: 'Published as is',
      unpublishTitle: 'Unpublish',
      unpublishText: 'The page goes offline. You can publish the collection again later.',
      unpublishButton: 'Unpublish…',
      readOnly: "You can't manage this publication",
      confirmTitle: 'Unpublish collection',
      confirmText: 'The page at {url} goes offline for everyone.',
      confirmAction: 'Unpublish',
      cancel: 'Cancel',
    },
  },
  ru: {
    visibility: { public: 'Публичная', unlisted: 'По ссылке', password: 'С паролем' },
    unavailable: {
      not_logged_in: 'Войдите, чтобы публиковать',
      no_capability: 'Этот сервер не поддерживает публикацию',
      not_root: 'Публиковать можно только коллекции верхнего уровня',
      offline: 'Нет связи\u00a0— показано последнее известное состояние',
    },
    unlistedHint: 'Хотите сначала посмотреть? Опубликуйте по ссылке\u00a0— страница не попадёт в поиск, а сделать её публичной можно позже.',
    tabs: {
      overview: 'Обзор',
      authorization: 'Авторизация',
      scripts: 'Скрипты',
      publish: 'Публикация',
      changed: 'Изменена после публикации',
    },
    menu: { publish: 'Опубликовать…', publication: 'Публикация…' },
    toasts: {
      published: 'Опубликовано',
      updated: 'Публикация обновлена',
      copyLink: 'Скопировать ссылку',
      linkCopied: 'Ссылка скопирована',
      copyFailed: 'Не удалось скопировать ссылку',
      openFailed: 'Не удалось открыть ссылку',
      offline: 'Страница снята с публикации',
    },
    thumbnail: { caption: 'Как выглядит страница', open: 'Открыть в Tetiva', download: 'Скачать' },
    dialog: {
      titlePublish: 'Опубликовать «{name}»',
      titleUpdate: 'Обновить публикацию · {name}',
      versionMove: 'версия {from} → {to}',
      reopen: 'Страница снова откроется по прежней ссылке.',
      intro: 'Страница только для чтения на share.tetiva.app. Ссылка появится после публикации.',
      checking: 'Проверяем коллекцию…',
      signIn: 'Войти',
      tryAgain: 'Повторить',
      signInText: 'Страницы на share.tetiva.app привязаны к аккаунту Tetiva.',
      noCapabilityText: 'Чтобы опубликовать коллекцию, подключитесь к серверу с публичными страницами.',
      offlineTitle: 'Сервер недоступен',
      offlineText: 'Проверьте подключение и попробуйте снова.',
      loadFailedTitle: 'Не удалось проверить публикацию',
      loadFailed: 'Не удалось загрузить состояние публикации',
      manage: 'Управлять этой публикацией вы не можете',
      saveFailed: 'Не удалось сохранить последние изменения. Исправьте их и попробуйте снова',
      visibility: 'Доступ',
      visibilities: {
        public: { label: 'Публичная', hint: 'Страницу откроет любой. Поисковики могут её проиндексировать.' },
        unlisted: { label: 'По ссылке', hint: 'Страницу откроют только те, у кого есть ссылка. В поиск она не попадёт.' },
        password: { label: 'С паролем', hint: 'Чтобы открыть страницу, нужно ввести пароль. В поиск она не попадёт.' },
      },
      proOnly: 'Доступно в тарифе Pro.',
      seePlans: 'Тарифы',
      password: 'Пароль',
      passwordKeep: 'Оставьте пустым, чтобы не менять пароль',
      passwordShow: 'Показать пароль',
      passwordHide: 'Скрыть пароль',
      passwordHelp: 'От 8 до 72 байт. После смены пароля читателям придётся ввести новый.',
      passwordInvalid: 'Пароль должен быть от 8 до 72 байт',
      environment: 'Окружение',
      none: 'Нет',
      environmentMissing: 'Окружения, выбранного в прошлый раз, нет на этом устройстве. Выберите заново.',
      environmentHint: 'Его несекретные переменные публикуются вместе со страницей.',
      scripts: 'Скрипты',
      includeScripts: 'Публиковать скрипты',
      scriptsHint: 'В скриптах бывают секреты. Скрипты публикуются как есть.',
      buildingPreview: 'Собираем предпросмотр…',
      acknowledge: 'Проверено',
      warnings: {
        one: '{n} предупреждение',
        few: '{n} предупреждения',
        many: '{n} предупреждений',
        other: '{n} предупреждения',
      },
      cancel: 'Отмена',
      update: 'Обновить публикацию',
      publish: 'Опубликовать',
      confirmPublicTitle: 'Сделать страницу публичной?',
      confirmPublicText: 'Страница станет публичной и попадёт в поиск.',
      confirmPublicAction: 'Сделать публичной',
    },
    preview: {
      title: 'Предпросмотр',
      folders: { one: '{n} папка', few: '{n} папки', many: '{n} папок', other: '{n} папки' },
      requests: { one: '{n} запрос', few: '{n} запроса', many: '{n} запросов', other: '{n} запроса' },
      examples: { one: '{n} пример', few: '{n} примера', many: '{n} примеров', other: '{n} примера' },
      environment: 'Окружение',
      environmentNone: 'нет',
      scripts: 'Скрипты',
      scriptsIncluded: 'включены',
      scriptsLeftOut: 'не включены',
      mustFix: 'Исправьте до публикации · {n}',
      variables: 'Переменные · {name}',
      published: 'Публикуются · {n}',
      hidden: 'Скрыты · {n}\u00a0— публикуются как {token} с пустым значением',
      hiddenReasons: {
        secret: 'секретная',
        referenced: 'используется в секретном поле',
        suspicious: 'имя похоже на секрет',
      },
      publishAsIs: 'Публиковать как есть',
      makeSecret: 'Сделать секретной',
      neverPublished: 'не публикуется',
      removed: 'Убрано со страницы · {n}',
      always: 'всегда',
      warnings: 'Предупреждения · {n}',
      publishedAsWritten: 'Публикуется как есть.',
      replaced: 'Заменяется на <redacted>, если не публиковать как есть.',
      ignored: {
        one: '{n} прежний выбор «Публиковать как есть» больше ни к чему не относится и будет сброшен.',
        few: '{n} прежних выбора «Публиковать как есть» больше ни к чему не относятся и будут сброшены.',
        many: '{n} прежних выборов «Публиковать как есть» больше ни к чему не относятся и будут сброшены.',
        other: '{n} прежних выбора «Публиковать как есть» больше ни к чему не относятся и будут сброшены.',
      },
      size: 'Размер',
      snapshot: 'Снимок',
      compressed: 'Сжатый',
      sizeOf: '{used} из {limit}',
      overLimit: 'Коллекция больше допустимого размера. Удалите крупные примеры ответов и попробуйте снова.',
      noBlocking: 'Блокирующих ошибок нет',
      rules: {
        'jwt': 'JWT',
        'aws-access-key': 'Ключ доступа AWS',
        'github-token': 'Токен GitHub',
        'slack-token': 'Токен Slack',
        'stripe-key': 'Секретный ключ Stripe',
        'google-api-key': 'Ключ Google API',
        'telegram-bot-token': 'Токен бота Telegram',
        'bearer-token': 'Bearer-токен',
        'oauth-token-field': 'Токен OAuth',
        'private-key': 'Закрытый ключ',
        'json-secret-key': 'Секретное поле JSON',
        'graphql-secret-argument': 'Секретный аргумент GraphQL',
        'xml-secret-field': 'Секретное поле XML',
        'form-secret-field': 'Секретное поле формы',
        'url-secret-parameter': 'Секретный параметр URL',
      },
      redactions: {
        var: 'используется в секретном поле',
        auth: 'секретное поле авторизации; храните значение в секретной переменной',
        header: 'чувствительный заголовок',
        metadata: 'чувствительные метаданные',
        query: 'чувствительный параметр запроса',
        url: 'учётные данные в URL',
        form: 'чувствительное поле формы',
        file: 'публикуется только имя файла',
        script: 'скрипты не публикуются',
        cookie: 'cookie никогда не публикуются',
      },
      blocking: {
        text_too_long: 'текст длиннее 1 МБ',
        value_too_long: 'значение длиннее {limit} КБ',
        name_too_long: 'имя длиннее {limit} символов',
        header_name_invalid: 'в имени заголовка «{name}» допустимы только символы токена и ссылки {{variable}}',
        url_too_long: 'URL длиннее 8 КБ',
        url_control_char: 'в URL есть управляющий символ',
        name_blank: 'у коллекции пустое имя',
        too_many_values: 'значений JSON: {count}, а опубликовать можно не больше {limit}',
        too_many: '{list}: {count}, а опубликовать можно не больше {limit}',
        auth_too_deep: 'вложенность глубже {limit} уровней',
        auth_too_many_values: 'значений внутри: {count}, а опубликовать можно не больше {limit}',
        too_many_items: 'папок и запросов: {count}, а опубликовать можно не больше {limit}',
        folders_too_deep: 'папки вложены глубже {limit} уровней',
        method_unsupported: 'HTTP-метод «{value}» нельзя опубликовать',
        protocol_unsupported: 'протокол «{value}» нельзя опубликовать',
        body_type_unsupported: 'тип тела «{value}» нельзя опубликовать',
        status_out_of_range: 'статус {value} вне диапазона 0–999',
        auth_type_unsupported: 'тип авторизации «{value}» нельзя опубликовать',
      },
      lists: {
        headers: 'заголовков',
        metadata: 'записей метаданных',
        form_fields: 'полей формы',
        subprotocols: 'подпротоколов',
        messages: 'сообщений',
        examples: 'примеров',
        variables: 'переменных',
        auth_fields: 'полей авторизации',
      },
    },
    panel: {
      thisCollection: 'эту коллекцию',
      loadFailed: 'Не удалось проверить публикацию',
      tryAgain: 'Повторить',
      checking: 'Проверяем публикацию…',
      emptyTitle: 'Опубликовать «{name}» как публичную страницу',
      emptyText: 'Страница только для чтения на share.tetiva.app: документация запросов, примеры ответов и фрагменты кода. '
        + 'Читатели откроют её в Tetiva или скачают коллекцию. Аккаунт им не нужен.',
      features: {
        audience: { title: 'Вы решаете, кто её откроет', text: 'Все, те, у кого есть ссылка, или те, кто знает пароль' },
        environment: { title: 'Выберите окружение', text: 'Его несекретные переменные попадут на страницу' },
        secrets: {
          title: 'Секреты остаются на устройстве',
          text: 'Секретные переменные, cookie и токены OAuth не публикуются никогда. Перед публикацией вы всё проверяете сами',
        },
      },
      publish: 'Опубликовать…',
      signInToPublish: 'Войдите, чтобы публиковать',
      published: 'Опубликована',
      updated: 'Обновлена {when} · версия {n}',
      version: 'версия {n}',
      refresh: 'Обновить',
      reviewUpdate: 'Проверить и обновить…',
      update: 'Обновить публикацию…',
      copy: 'Копировать',
      open: 'Открыть',
      signInToManage: 'Войдите, чтобы обновить страницу или снять её с публикации',
      signIn: 'Войти',
      unpublishFailed: 'Не удалось снять с публикации: {error}',
      unpublishing: 'Снимаем с публикации…',
      blocked: 'Заблокировано платформой',
      blockedFor: 'Заблокировано платформой: {reason}',
      notRoot: 'Коллекция больше не верхнего уровня\u00a0— её снимут с публикации',
      changed: 'Изменена после публикации.',
      stillShows: 'Страница пока показывает версию {n}.',
      counters: { views: 'Просмотры', imports: 'Открыли в Tetiva', downloads: 'Скачивания' },
      settings: 'Настройки публикации',
      visibility: 'Доступ',
      environment: 'Окружение',
      none: 'Нет',
      notOnDevice: 'нет на этом устройстве',
      scripts: 'Скрипты',
      included: 'Включены',
      leftOut: 'Не включены',
      publishedAsIs: 'Публикуется как есть',
      unpublishTitle: 'Снять с публикации',
      unpublishText: 'Страница пропадёт из сети. Коллекцию можно опубликовать снова позже.',
      unpublishButton: 'Снять с публикации…',
      readOnly: 'Управлять этой публикацией вы не можете',
      confirmTitle: 'Снять коллекцию с публикации',
      confirmText: 'Страница {url} пропадёт из сети для всех.',
      confirmAction: 'Снять',
      cancel: 'Отмена',
    },
  },
}
