export interface ReleaseNotes {
  version: string
  date: string
  en: { added: string[]; fixed: string[] }
  ru: { added: string[]; fixed: string[] }
}

// Newest first. Only user-visible releases get an entry; technical patches are
// omitted so no What's New modal appears for them.
export const RELEASE_NOTES: ReleaseNotes[] = [
  {
    version: '1.1.0',
    date: '2026-09-07',
    en: {
      added: [
        'Sign in through your browser: the app opens the Tetiva cabinet, you confirm the sign-in there, and the session comes back on its own. Registration, e-mail confirmation and password reset live on the site too.',
        'Paste a curl command into the URL bar and it becomes a request: method, address, headers, body and auth.',
        'Auth schemes OAuth 2.0 (client credentials, password, authorization code with PKCE through the browser), JWT Bearer, Digest and AWS Signature V4 — on a request or on a collection that nested requests inherit from.',
        'WebSocket requests are edited like any other: Params, Auth, Headers, a pre-connect script, cookies, binary frames, saved messages, keepalive ping and subprotocols.',
        'A Markdown description next to every request and collection, with a real editor: headings, lists, tables you can fill from the keyboard, code, links. It travels through Postman import and export and is visible to MCP agents.',
        'The MCP server now requires a token (shown in Settings); agents see credentials masked.',
      ],
      fixed: [
        'Retrying a request from history no longer opens an empty tab and no longer carries the previous credentials.',
        'Creating a workspace with "Sync to server" works again, and a workspace created right after signing in can be synced.',
        'MCP update_request changes only the fields it receives instead of blanking the rest.',
      ],
    },
    ru: {
      added: [
        'Вход через браузер: приложение открывает кабинет Tetiva, вы подтверждаете вход там, и сессия сама возвращается. Регистрация, подтверждение почты и восстановление пароля тоже на сайте.',
        'Вставьте команду curl в строку адреса — она превратится в запрос: метод, адрес, заголовки, тело и авторизация.',
        'Схемы авторизации OAuth 2.0 (client credentials, password, authorization code с PKCE через браузер), JWT Bearer, Digest и AWS Signature V4 — у запроса или у коллекции, от которой их наследуют вложенные запросы.',
        'WebSocket-запрос редактируется как обычный: Params, Auth, Headers, скрипт перед подключением, куки, бинарные кадры, сохранённые сообщения, keepalive-ping и subprotocols.',
        'Описание в Markdown у каждого запроса и коллекции, в настоящем редакторе: заголовки, списки, таблицы с заполнением с клавиатуры, код, ссылки. Переносится при импорте и экспорте Postman и видно агентам через MCP.',
        'MCP-сервер теперь требует токен (показан в настройках); агенты видят учётные данные замаскированными.',
      ],
      fixed: [
        'Повтор запроса из истории больше не открывает пустую вкладку и не переносит прежние учётные данные.',
        'Создание воркспейса с галкой «Sync to server» снова работает, а воркспейс, созданный сразу после входа, можно синхронизировать.',
        'MCP update_request меняет только переданные поля, а не затирает остальные.',
      ],
    },
  },
  {
    version: '0.17.0',
    date: '2026-08-16',
    en: {
      added: [
        'Links to the documentation: a Documentation item in Settings and a "?" next to MCP, scripts, the gRPC editor, sync and environments — each opens its own page.',
        'Sync settings list the devices signed in to your account: sign one of them out, or sign out everywhere.',
        'When the server refuses to sync something because of a plan limit, the app says so instead of retrying in silence. Your data stays on disk.',
      ],
      fixed: [
        'Re-linking a workspace to a different cloud workspace now loads its contents right away.',
      ],
    },
    ru: {
      added: [
        'Ссылки на документацию: пункт «Documentation» в настройках и «?» рядом с MCP, скриптами, gRPC-редактором, синхронизацией и окружениями — каждый ведёт на свою страницу.',
        'В окне синхронизации видно устройства, где вы вошли в аккаунт: можно выйти с одного или со всех сразу.',
        'Если сервер отказал в синхронизации из-за лимита тарифа, приложение говорит об этом, а не повторяет попытки молча. Данные остаются на диске.',
      ],
      fixed: [
        'После перепривязки к другому облачному воркспейсу его содержимое загружается сразу.',
      ],
    },
  },
  {
    version: '0.16.1',
    date: '2026-08-12',
    en: {
      added: [],
      fixed: [
        'The MCP server now listens on localhost only, and its tools no longer return tokens, passwords or secret variables.',
        'A script from an imported collection can no longer freeze the app: a run that ignores the five-second limit is detached and the request fails instead.',
        'gRPC metadata reaches scripts as a copy, so a script no longer edits the saved request.',
      ],
    },
    ru: {
      added: [],
      fixed: [
        'MCP-сервер слушает только localhost, а его инструменты больше не отдают токены, пароли и секретные переменные.',
        'Скрипт из импортированной коллекции больше не может заморозить приложение: если он не укладывается в пять секунд, запрос завершается ошибкой, а окно продолжает работать.',
        'gRPC-метаданные приходят в скрипт копией — сохранённый запрос скрипт больше не меняет.',
      ],
    },
  },
  {
    version: '0.16.0',
    date: '2026-07-27',
    en: {
      added: [
        'A welcome screen on first launch: work locally or connect an account — switchable any time.',
        'An optional four-slide tour of what Tetiva does. Reopen it from Settings.',
        'After you sign up, the app says the confirmation email was sent and can resend it.',
      ],
      fixed: [
        'Your session survives a restart even when the system keychain is unavailable.',
      ],
    },
    ru: {
      added: [
        'Экран приветствия при первом запуске: работать локально или подключить аккаунт — выбор можно изменить в любой момент.',
        'Необязательный тур из четырёх экранов о возможностях Tetiva. Открыть заново можно в настройках.',
        'После регистрации приложение сообщает об отправленном письме и может отправить его повторно.',
      ],
      fixed: [
        'Сессия переживает перезапуск даже при недоступной связке ключей.',
      ],
    },
  },
  {
    version: '0.15.3',
    date: '2026-07-16',
    en: {
      added: [],
      fixed: [
        'Windows: the app now sends real requests instead of showing demo data.',
        'Windows: the installer is properly branded as Tetiva.',
      ],
    },
    ru: {
      added: [],
      fixed: [
        'Windows: приложение теперь выполняет реальные запросы вместо демо-данных.',
        'Windows: инсталлер корректно называется Tetiva.',
      ],
    },
  },
  {
    version: '0.15.2',
    date: '2026-07-14',
    en: {
      added: [],
      fixed: [
        'Improved stability and reliability of the cloud connection.',
      ],
    },
    ru: {
      added: [],
      fixed: [
        'Повышена стабильность и надёжность соединения с облаком.',
      ],
    },
  },
  {
    version: '0.15.1',
    date: '2026-07-13',
    en: {
      added: [],
      fixed: [
        'Your current workspace now starts syncing right after you sign in — the cloud icon no longer stays grey.',
      ],
    },
    ru: {
      added: [],
      fixed: [
        'Текущий воркспейс теперь синхронизируется сразу после входа — иконка облака больше не остаётся серой.',
      ],
    },
  },
  {
    version: '0.15.0',
    date: '2026-07-12',
    en: {
      added: [
        'Tetiva Cloud is connected by default — no server address to enter.',
        "Update notifications and this What's New window.",
      ],
      fixed: [
        'Cmd+W closes the active tab instead of quitting the app.',
        'Safer saving: your input is no longer lost.',
        'Operation errors are now shown right away.',
      ],
    },
    ru: {
      added: [
        'Облачный сервер Tetiva подключён по умолчанию — адрес вводить не нужно.',
        'Уведомления о новых версиях и это окно «Что нового».',
      ],
      fixed: [
        'Cmd+W закрывает активную вкладку, а не всё приложение.',
        'Безопасное сохранение: введённые данные больше не теряются.',
        'Ошибки операций теперь видны сразу.',
      ],
    },
  },
]

export function notesFor(version: string): ReleaseNotes | undefined {
  return RELEASE_NOTES.find((n) => n.version === version)
}

// System locale → UI language for the notes copy. Russian for any ru* tag.
export function pickLocale(navLang: string): 'ru' | 'en' {
  return navLang.toLowerCase().startsWith('ru') ? 'ru' : 'en'
}
