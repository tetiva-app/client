import { fill, type Locale } from '@/lib/locale'
import type { SettingsRowIndex, SettingsSectionId } from '@/lib/settings-search'

export interface SettingsCopy {
  title: string
  searchPlaceholder: string
  nothingFound: string
  nothingFoundHint: string
  close: string
  off: string
  updateAvailable: string
  sections: Record<SettingsSectionId, { title: string; description: string }>
  interface: {
    language: string
    languageHint: string
    languageSystem: string
    languageNow: string
    languageEnglish: string
    languageRussian: string
    beta: string
    translated: string
    theme: string
    themeHint: string
    themeLight: string
    themeDark: string
    themeSystem: string
    themeNow: string
    themeNowLight: string
    themeNowDark: string
  }
  editor: {
    fontSize: string
    fontSizeHint: string
    wordWrap: string
    wordWrapHint: string
    sample: string
  }
  publishing: {
    enabled: string
    enabledHint: string
    pagesStay: string
    cabinet: string
  }
  mcp: {
    howItWorks: string
    flowAgent: string
    flowServer: string
    flowData: string
    flowDataHint: string
    flowToken: string
    flowNoToken: string
    server: string
    serverHint: string
    running: string
    stopped: string
    restartRequired: string
    envManaged: string
    enable: string
    connect: string
    urlOnly: string
    copyConfig: string
    copied: string
    showConfig: string
    hideConfig: string
    hintClaude: string
    hintCursor: string
    hintUrl: string
    token: string
    tokenHint: string
    showToken: string
    hideToken: string
    copyToken: string
    regenerate: string
    confirm: string
    regenerateWarning: string
    requireToken: string
    requireTokenHint: string
    requireTokenArmed: string
    requireTokenOff: string
    address: string
    addressHint: string
    addressExposed: string
    unavailable: string
    errors: {
      address: string
      envManaged: string
      saveFailed: string
    }
  }
  updates: {
    auto: string
    autoHint: string
    check: string
    lastChecked: string
    neverChecked: string
    checkNow: string
    checking: string
    upToDate: string
    unreachable: string
    available: string
    availableHint: string
    download: string
    sends: string
    sendsNothing: string
    sendsOff: string
  }
  about: {
    version: string
    whatsNew: string
    whatsNewHint: string
    open: string
    help: string
    showWelcome: string
    docs: string
    storedLocally: string
  }
}

export const SETTINGS_COPY: Record<Locale, SettingsCopy> = {
  en: {
    title: 'Settings',
    searchPlaceholder: 'Search settings',
    nothingFound: 'Nothing found for “{q}”',
    nothingFoundHint: 'Search covers English and Russian names.',
    close: 'Close',
    off: 'off',
    updateAvailable: 'Update available',
    sections: {
      interface: { title: 'Interface', description: 'Language and theme.' },
      editor: { title: 'Editor', description: 'How code looks in the body, script and response editors.' },
      publishing: { title: 'Publishing', description: 'Collections published as read-only web pages.' },
      mcp: { title: 'AI agents (MCP)', description: 'Claude, Cursor and other agents get access to your collections.' },
      updates: { title: 'Updates', description: 'Tetiva tells you when a new version is out.' },
      about: { title: 'About', description: 'Version, documentation and release notes.' },
    },
    interface: {
      language: 'Language',
      languageHint: 'Switches right away, no restart.',
      languageSystem: 'System',
      languageNow: 'now {language}',
      languageEnglish: 'English',
      languageRussian: 'Russian',
      beta: 'beta',
      translated: 'Translated: settings, publishing, the collection tree and its menus, sync, the welcome screen and What’s New. The request editor stays in English.',
      theme: 'Theme',
      themeHint: 'System follows your OS and switches along with it.',
      themeLight: 'Light',
      themeDark: 'Dark',
      themeSystem: 'System',
      themeNow: 'now {theme}',
      themeNowLight: 'light',
      themeNowDark: 'dark',
    },
    editor: {
      fontSize: 'Font size',
      fontSizeHint: 'For code in every editor and the response viewer; the rest of the interface keeps its size.',
      wordWrap: 'Word wrap',
      wordWrapHint: 'Long lines wrap instead of running off the edge.',
      sample: 'Preview',
    },
    publishing: {
      enabled: 'Publishing in this client',
      enabledHint: 'Turning it off hides the publishing tab, menu items and the Publications panel.',
      pagesStay: 'Published pages stay online.',
      cabinet: 'Manage pages in the web cabinet',
    },
    mcp: {
      howItWorks: 'How it works',
      flowAgent: 'AI agent',
      flowServer: 'MCP server',
      flowData: 'Your data',
      flowDataHint: 'collections, environments',
      flowToken: 'token',
      flowNoToken: 'no token',
      server: 'MCP server',
      serverHint: 'Turning it on and changing the address apply after a restart; the token settings apply right away.',
      running: 'Running',
      stopped: 'Stopped',
      restartRequired: 'Restart Tetiva to apply the changes.',
      envManaged: 'Managed by environment variables.',
      enable: 'Enable the MCP server',
      connect: 'Connect an agent',
      urlOnly: 'URL only',
      copyConfig: 'Copy config',
      copied: 'Copied',
      showConfig: 'Show config',
      hideConfig: 'Hide config',
      hintClaude: 'The token goes in the URL as ?token=…',
      hintCursor: 'Cursor sends the token in an Authorization: Bearer header.',
      hintUrl: 'Just the address, with the token in ?token=…',
      token: 'Access token',
      tokenHint: 'Agents send it with every call.',
      showToken: 'Show token',
      hideToken: 'Hide token',
      copyToken: 'Copy token',
      regenerate: 'Regenerate',
      confirm: 'Confirm',
      regenerateWarning: 'A new token disconnects every agent still using the old one.',
      requireToken: 'Require token',
      requireTokenHint: 'Recommended. Without it, any program on this computer can connect.',
      requireTokenArmed: 'Click the switch again to turn the token check off. Any process on this computer, a browser tab included, will then read your collections and variables and send requests as you.',
      requireTokenOff: 'The token check is off: any process on this computer, a browser tab included, can read your collections and variables and send requests as you.',
      address: 'Address',
      addressHint: 'With 127.0.0.1, only this computer can connect.',
      addressExposed: 'Reachable from your network: only the token stands between your collections and anyone who can reach this address. Keep 127.0.0.1 unless you need remote access, and never turn the token off.',
      unavailable: 'MCP info unavailable.',
      errors: {
        address: 'Use host:port with a port from 1 to 65535, for example 127.0.0.1:9300.',
        envManaged: "Can't change: the MCP server is managed by environment variables.",
        saveFailed: "Couldn't save: {detail}",
      },
    },
    updates: {
      auto: 'Check automatically',
      autoHint: 'On launch, at most once a day.',
      check: 'Check for updates',
      lastChecked: 'Last checked {when}.',
      neverChecked: 'Not checked yet.',
      checkNow: 'Check now',
      checking: 'Checking…',
      upToDate: 'You’re up to date',
      unreachable: 'Couldn’t reach the update server',
      available: 'Tetiva {version} is available',
      availableHint: 'You have {current}. The download opens in your browser.',
      download: 'Download',
      sends: 'What the check sends',
      sendsNothing: 'No account, collections or settings are sent.',
      sendsOff: 'Automatic check is off: nothing is sent until you press Check now.',
    },
    about: {
      version: 'Version {version}',
      whatsNew: 'What’s New',
      whatsNewHint: 'Release notes for the installed version.',
      open: 'Open',
      help: 'Help',
      showWelcome: 'Show welcome screen',
      docs: 'Documentation',
      storedLocally: 'Settings are stored on this device and don’t sync.',
    },
  },
  ru: {
    title: 'Настройки',
    searchPlaceholder: 'Найти настройку',
    nothingFound: 'По запросу «{q}» ничего не нашлось',
    nothingFoundHint: 'Поиск идёт по русским и английским названиям.',
    close: 'Закрыть',
    off: 'выкл.',
    updateAvailable: 'Есть обновление',
    sections: {
      interface: { title: 'Интерфейс', description: 'Язык и тема оформления.' },
      editor: { title: 'Редактор', description: 'Как выглядит код в редакторах тела запроса, скриптов и ответа.' },
      publishing: { title: 'Публикация', description: 'Коллекции, опубликованные как веб-страницы только для чтения.' },
      mcp: { title: 'ИИ-агенты (MCP)', description: 'Claude, Cursor и другие агенты получают доступ к вашим коллекциям.' },
      updates: { title: 'Обновления', description: 'Tetiva сообщает, когда выходит новая версия.' },
      about: { title: 'О программе', description: 'Версия, документация и список изменений.' },
    },
    interface: {
      language: 'Язык',
      languageHint: 'Меняется сразу, без перезапуска.',
      languageSystem: 'Как в системе',
      languageNow: 'сейчас {language}',
      languageEnglish: 'английский',
      languageRussian: 'русский',
      beta: 'бета',
      translated: 'Переведены настройки, публикация, дерево коллекций и его меню, синхронизация, приветствие и «Что нового». Редактор запросов\u00a0— на английском.',
      theme: 'Тема',
      themeHint: '«Как в системе» повторяет тему ОС и меняется вместе с ней.',
      themeLight: 'Светлая',
      themeDark: 'Тёмная',
      themeSystem: 'Как в системе',
      themeNow: 'сейчас {theme}',
      themeNowLight: 'светлая',
      themeNowDark: 'тёмная',
    },
    editor: {
      fontSize: 'Размер шрифта',
      fontSizeHint: 'Для кода во всех редакторах и в ответе; остальной интерфейс не меняется.',
      wordWrap: 'Перенос строк',
      wordWrapHint: 'Длинные строки переносятся, а не уходят за край.',
      sample: 'Пример',
    },
    publishing: {
      enabled: 'Публикация в этом клиенте',
      enabledHint: 'Если выключить, скроются вкладка публикации, пункты меню и панель «Публикации».',
      pagesStay: 'Опубликованные страницы останутся в сети.',
      cabinet: 'Управлять страницами в кабинете',
    },
    mcp: {
      howItWorks: 'Как это работает',
      flowAgent: 'ИИ-агент',
      flowServer: 'MCP-сервер',
      flowData: 'Ваши данные',
      flowDataHint: 'коллекции, окружения',
      flowToken: 'токен',
      flowNoToken: 'без токена',
      server: 'MCP-сервер',
      serverHint: 'Включение и смена адреса вступают в силу после перезапуска, настройки токена\u00a0— сразу.',
      running: 'Работает',
      stopped: 'Остановлен',
      restartRequired: 'Перезапустите Tetiva, чтобы изменения вступили в силу.',
      envManaged: 'Задано переменными окружения.',
      enable: 'Включить MCP-сервер',
      connect: 'Подключить агента',
      urlOnly: 'Только URL',
      copyConfig: 'Скопировать конфиг',
      copied: 'Скопировано',
      showConfig: 'Показать конфиг',
      hideConfig: 'Скрыть конфиг',
      hintClaude: 'Токен передаётся в адресе: ?token=…',
      hintCursor: 'Cursor передаёт токен в заголовке Authorization: Bearer.',
      hintUrl: 'Только адрес, токен\u00a0— в ?token=…',
      token: 'Токен доступа',
      tokenHint: 'Агент передаёт его при каждом обращении.',
      showToken: 'Показать токен',
      hideToken: 'Скрыть токен',
      copyToken: 'Скопировать токен',
      regenerate: 'Сменить',
      confirm: 'Подтвердить',
      regenerateWarning: 'Новый токен отключит всех агентов, у которых записан старый.',
      requireToken: 'Требовать токен',
      requireTokenHint: 'Рекомендуется. Без токена подключится любая программа на этом компьютере.',
      requireTokenArmed: 'Нажмите ещё раз, чтобы отключить проверку токена. Тогда любой процесс на компьютере, включая вкладку браузера, сможет читать коллекции и переменные и слать запросы от вашего имени.',
      requireTokenOff: 'Проверка токена выключена: любой процесс на компьютере, включая вкладку браузера, может читать коллекции и переменные и слать запросы от вашего имени.',
      address: 'Адрес',
      addressHint: 'С 127.0.0.1 подключиться можно только с этого компьютера.',
      addressExposed: 'Сервер виден из сети: от всех, кто может до него достучаться, коллекции защищает только токен. Оставьте 127.0.0.1, если не нужен удалённый доступ, и не отключайте токен.',
      unavailable: 'Сведения о MCP недоступны.',
      errors: {
        address: 'Укажите адрес как хост:порт, порт от 1 до 65535, например 127.0.0.1:9300.',
        envManaged: 'Не изменить: MCP-сервер задан переменными окружения.',
        saveFailed: 'Не удалось сохранить: {detail}',
      },
    },
    updates: {
      auto: 'Проверять автоматически',
      autoHint: 'При запуске, не чаще раза в сутки.',
      check: 'Проверка обновлений',
      lastChecked: 'Последняя проверка: {when}.',
      neverChecked: 'Ещё не проверялось.',
      checkNow: 'Проверить сейчас',
      checking: 'Проверяем…',
      upToDate: 'У вас последняя версия',
      unreachable: 'Сервер обновлений недоступен',
      available: 'Доступна Tetiva {version}',
      availableHint: 'У вас {current}. Скачивание откроется в браузере.',
      download: 'Скачать',
      sends: 'Что отправляет проверка',
      sendsNothing: 'Ни аккаунт, ни коллекции, ни настройки не отправляются.',
      sendsOff: 'Автопроверка выключена: ничего не отправляется, пока вы не нажмёте «Проверить сейчас».',
    },
    about: {
      version: 'Версия {version}',
      whatsNew: 'Что нового',
      whatsNewHint: 'Список изменений установленной версии.',
      open: 'Открыть',
      help: 'Справка',
      showWelcome: 'Показать приветствие',
      docs: 'Документация',
      storedLocally: 'Настройки хранятся на этом устройстве и не синхронизируются.',
    },
  },
}

interface IndexedRow {
  id: string
  section: SettingsSectionId
  text: (c: SettingsCopy) => string[]
  keywords: Record<Locale, string>
}

const ROWS: IndexedRow[] = [
  {
    id: 'language', section: 'interface',
    text: (c) => [c.interface.language, c.interface.languageHint],
    keywords: { en: 'locale translation russian english', ru: 'перевод русский английский локаль' },
  },
  {
    id: 'theme', section: 'interface',
    text: (c) => [c.interface.theme, c.interface.themeHint, c.interface.themeLight, c.interface.themeDark],
    keywords: { en: 'appearance colors', ru: 'оформление цвет' },
  },
  {
    id: 'font-size', section: 'editor',
    text: (c) => [c.editor.fontSize, c.editor.fontSizeHint],
    keywords: { en: 'text code', ru: 'текст код' },
  },
  {
    id: 'word-wrap', section: 'editor',
    text: (c) => [c.editor.wordWrap, c.editor.wordWrapHint],
    keywords: { en: 'lines', ru: 'строки' },
  },
  {
    id: 'publishing', section: 'publishing',
    text: (c) => [c.publishing.enabled, c.publishing.enabledHint, c.publishing.cabinet],
    keywords: { en: 'publish share pages publications web cabinet account', ru: 'опубликовать страницы публикации кабинет аккаунт' },
  },
  {
    id: 'mcp-server', section: 'mcp',
    text: (c) => [c.mcp.server, c.mcp.serverHint, c.mcp.running, c.mcp.stopped],
    keywords: { en: 'enable restart status', ru: 'включить перезапуск статус' },
  },
  {
    id: 'mcp-connect', section: 'mcp',
    text: (c) => [c.mcp.connect, c.mcp.copyConfig, c.mcp.urlOnly],
    keywords: { en: 'claude desktop cursor json', ru: 'конфигурация' },
  },
  {
    id: 'mcp-token', section: 'mcp',
    text: (c) => [c.mcp.token, c.mcp.tokenHint, c.mcp.regenerate],
    keywords: { en: 'bearer', ru: 'ключ' },
  },
  {
    id: 'mcp-require-token', section: 'mcp',
    text: (c) => [c.mcp.requireToken, c.mcp.requireTokenHint],
    keywords: { en: 'security', ru: 'безопасность защита' },
  },
  {
    id: 'mcp-address', section: 'mcp',
    text: (c) => [c.mcp.address, c.mcp.addressHint],
    keywords: { en: 'port host network', ru: 'порт сеть' },
  },
  {
    id: 'updates-auto', section: 'updates',
    text: (c) => [c.updates.auto, c.updates.autoHint],
    keywords: { en: 'update', ru: 'обновление' },
  },
  {
    id: 'updates-check', section: 'updates',
    text: (c) => [c.updates.check, c.updates.checkNow, c.updates.download],
    keywords: { en: 'version', ru: 'версия' },
  },
  {
    id: 'updates-sends', section: 'updates',
    text: (c) => [c.updates.sends, c.updates.sendsNothing],
    keywords: { en: 'privacy request', ru: 'приватность запрос' },
  },
  {
    id: 'about-version', section: 'about',
    text: (c) => [fill(c.about.version, { version: '' })],
    keywords: { en: 'tetiva', ru: 'tetiva' },
  },
  {
    id: 'about-whats-new', section: 'about',
    text: (c) => [c.about.whatsNew, c.about.whatsNewHint],
    keywords: { en: "whats new release notes changelog", ru: 'изменения' },
  },
  {
    id: 'about-help', section: 'about',
    text: (c) => [c.about.help, c.about.showWelcome, c.about.docs],
    keywords: { en: 'welcome onboarding docs', ru: 'приветствие справка' },
  },
]

export const SETTINGS_SEARCH_INDEX: SettingsRowIndex[] = ROWS.map((row) => ({
  id: row.id,
  section: row.section,
  text: {
    en: [SETTINGS_COPY.en.sections[row.section].title, ...row.text(SETTINGS_COPY.en), row.keywords.en],
    ru: [SETTINGS_COPY.ru.sections[row.section].title, ...row.text(SETTINGS_COPY.ru), row.keywords.ru],
  },
}))
