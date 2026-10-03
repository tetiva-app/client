export { systemLocale as pickLocale } from '@/lib/locale'

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
    version: '1.2.1',
    date: '2026-10-03',
    en: {
      added: [
        'Tetiva now updates itself on macOS and Windows: a new version downloads in the background, is checked against our signature and installs when you click "Restart to update" on the card at the bottom of the sidebar. The tabs you had open come back after the restart. On Linux the card shows the apt command.',
        'Postman collection variables come in with the collection as an environment, and Postman environment and globals files import from Import File… in the sidebar.',
        'pm.collectionVariables works in scripts: get, set and unset use the active environment.',
        'Windows: Tetiva installs for your user without an administrator prompt, and a copy in Program Files is removed after one confirmation.',
      ],
      fixed: [
        'Changes made in a cloud workspace while sync was off now reach the server once sync starts.',
        'The selected environment stays selected after a change on another device or the first sign-in, and variables keep the same order on every device (needs server 0.20).',
        'Environment import keeps disabled variables disabled, takes numbers and other non-text values, and no longer leaves a half-filled environment behind.',
        'The update dot and the sync warnings on the left rail no longer fade.',
      ],
    },
    ru: {
      added: [
        'Tetiva обновляется сама на macOS и Windows: новая версия скачивается в фоне, проверяется по нашей подписи и ставится, когда вы нажмёте «Перезапустить для обновления» на карточке внизу боковой панели. Открытые вкладки возвращаются после перезапуска. В Linux карточка показывает команду apt.',
        'Переменные коллекции Postman приходят вместе с коллекцией как окружение, а файлы окружений и глобальных переменных Postman импортируются через Import File… в боковой панели.',
        'В скриптах работает pm.collectionVariables: get, set и unset работают с активным окружением.',
        'Windows: Tetiva ставится для вашего пользователя без запроса прав администратора, а копия в Program Files удаляется после одного подтверждения.',
      ],
      fixed: [
        'Изменения в облачном воркспейсе, сделанные при выключенной синхронизации, доходят до сервера, как только синхронизация запустится.',
        'Выбранное окружение остаётся выбранным после правки на другом устройстве и после первого входа, а переменные идут в одном порядке на всех устройствах (нужен сервер 0.20).',
        'Импорт окружения не включает выключенные переменные, принимает числа и другие нетекстовые значения и больше не оставляет недозаполненное окружение.',
        'Точка обновления и предупреждения синхронизации на левой панели больше не тускнеют.',
      ],
    },
  },
  {
    version: '1.2.0',
    date: '2026-09-27',
    en: {
      added: [
        'Publish a collection as a page on share.tetiva.app: descriptions, code samples, response examples, an "Open in Tetiva" button and downloads for Tetiva and Postman. A preview shows what goes out, and secrets are replaced before anything leaves your computer. Free publishes one page with a "Made with Tetiva" badge; Pro adds unlimited pages, passwords and unlisted links.',
        'The Publications panel on the left lists your published pages; a badge on its icon shows how many are out of date.',
        'Open a published collection from its page in one click, or paste its link into "Import from Link…". Before anything is saved you see the requests, the hosts they call and their scripts; scripts stay out unless you tick the box.',
        'The arrow next to Send, Query, Invoke and Connect copies the request as code in one click: cURL, Python, JavaScript, Go, Java, C# and PHP, plus gRPCurl and websocat for gRPC and WebSocket. "Generate code…" in the same menu shows the code first.',
        'Save a response as a named example and edit it on the Examples tab. Examples sync across your devices and your team (needs server 0.19) and travel through Postman import and export.',
        "Settings are split into sections and have a search and a language choice. Russian (beta) covers the settings themselves, publishing, the collection tree and its menus, the Sync dialog, the welcome screen and What's New for now.",
        'Linux: a .deb package for ARM64.',
      ],
      fixed: [
        'Copy as cURL takes your unsaved edits and no longer lets curl read a local file when a body or form field starts with @ or <.',
        'On first sign-in the active local workspace is uploaded to the cloud with what it already holds. Free takes up to 20 top-level collections; the rest wait, with a notice.',
        'Closing a detached request window saves its edits first.',
        'macOS: text fields no longer autocorrect or capitalize what you type.',
        'Cancel now works in GraphQL and gRPC requests, and an answer that arrives after it no longer replaces the result.',
      ],
    },
    ru: {
      added: [
        'Коллекцию можно опубликовать страницей на share.tetiva.app: описания, примеры кода и ответов, кнопка «Открыть в Tetiva» и скачивание для Tetiva и Postman. Перед публикацией видно, что уйдёт наружу, а секреты заменяются ещё на вашем компьютере. На Free\u00a0— одна страница с плашкой «Сделано в Tetiva», на Pro\u00a0— без лимита, с паролем и скрытыми ссылками.',
        'Панель «Публикации» в левой колонке собирает опубликованные страницы, а число на её иконке показывает, сколько из них устарели.',
        'Опубликованную коллекцию можно открыть со страницы одним кликом или вставить её ссылку в «Import from Link…». До сохранения видно запросы, хосты, куда они ходят, и скрипты; скрипты не импортируются, пока вы не отметите галочку.',
        'Стрелка у Send, Query, Invoke и Connect копирует запрос в виде кода одним кликом: cURL, Python, JavaScript, Go, Java, C# и PHP, а для gRPC и WebSocket\u00a0— gRPCurl и websocat. «Generate code…» в том же меню сначала покажет код.',
        'Ответ можно сохранить как именованный пример и поправить во вкладке Examples. Примеры синхронизируются между устройствами и командой (нужен сервер 0.19) и переносятся при импорте и экспорте Postman.',
        'Настройки разбиты на разделы, в них появились поиск и выбор языка. На русский (бета) пока переведены сами настройки, публикация, дерево коллекций с его меню, окно синхронизации, приветствие и «Что нового».',
        'Linux: пакет .deb для ARM64.',
      ],
      fixed: [
        'Copy as cURL учитывает несохранённые правки и больше не даёт curl прочитать локальный файл, если тело или поле формы начинается с @ или <.',
        'При первом входе активный локальный воркспейс выгружается в облако с тем, что в нём уже есть. На Free\u00a0— до 20 коллекций верхнего уровня, остальные ждут, о чём приложение сообщит.',
        'Закрытие отдельного окна запроса сначала сохраняет правки.',
        'macOS: поля ввода больше не исправляют набранное и не делают первую букву заглавной.',
        'Кнопка Cancel теперь работает и в GraphQL, и в gRPC, а ответ, пришедший после отмены, не затирает результат.',
      ],
    },
  },
  {
    version: '1.1.1',
    date: '2026-09-14',
    en: {
      added: [
        'Request descriptions now sync between devices and reach the server (needs server 0.18). Docs written in 1.1.0 are sent once on first launch.',
        'Descriptions save themselves 1.5 s after you stop typing and when you switch or close a tab. Cmd+S still works.',
        'A description is limited to 16 KiB: a byte counter shows up near the limit and a too-long text is refused with a clear message.',
      ],
      fixed: [
        'Cmd+S, Cmd+F, Cmd+A, Cmd+, and Cmd+[ / ] work on Cyrillic and other non-Latin layouts; AltGr and Shift combinations no longer trigger them.',
        'Closing a tab during a save no longer loses the edits typed meanwhile; a failed collection save keeps the tab open.',
        'The description editor no longer skips every second update from sync, and Ctrl+Z no longer reverts synced text. Undo survives switching between Docs and the other tabs.',
        'Tables: Enter on an empty last row leaves the table cleanly, and a pipe inside inline code no longer breaks the row. Code, code block and link buttons escape what they wrap.',
        'Postman round trip keeps empty folders and request docs; oversized descriptions are truncated with a warning instead of failing the import.',
        'An item the server refuses as too large is parked with its own notice instead of stalling sync.',
      ],
    },
    ru: {
      added: [
        'Описания запросов теперь синхронизируются между устройствами и доходят до сервера (нужен сервер 0.18). Написанное в 1.1.0 отправится один раз при первом запуске.',
        'Описания сохраняются сами через 1,5 с после остановки набора и при переключении или закрытии вкладки. Cmd+S по-прежнему работает.',
        'Описание ограничено 16 КиБ: у границы появляется счётчик байт, слишком длинный текст отклоняется с понятным сообщением.',
      ],
      fixed: [
        'Cmd+S, Cmd+F, Cmd+A, Cmd+, и Cmd+[ / ] работают в русской и других не-латинских раскладках; сочетания с AltGr и Shift их больше не запускают.',
        'Закрытие вкладки во время сохранения больше не теряет набранное; неудачное сохранение коллекции оставляет вкладку открытой.',
        'Редактор описания больше не пропускает каждое второе обновление из синка, а Ctrl+Z не откатывает пришедший текст. Отмена переживает переключение между Docs и другими вкладками.',
        'Таблицы: Enter на пустой последней строке выходит из таблицы чисто, а вертикальная черта внутри инлайн-кода не ломает строку. Кнопки кода, блока кода и ссылки экранируют то, что оборачивают.',
        'Постман-оборот сохраняет пустые папки и описания запросов; слишком длинные описания усекаются с предупреждением, а не роняют импорт.',
        'Элемент, который сервер не принимает по размеру, откладывается с отдельным уведомлением и не останавливает синк.',
      ],
    },
  },
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
        'Вставьте команду curl в строку адреса\u00a0— она превратится в запрос: метод, адрес, заголовки, тело и авторизация.',
        'Схемы авторизации OAuth 2.0 (client credentials, password, authorization code с PKCE через браузер), JWT Bearer, Digest и AWS Signature V4\u00a0— у запроса или у коллекции, от которой их наследуют вложенные запросы.',
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
        'Ссылки на документацию: пункт «Documentation» в настройках и «?» рядом с MCP, скриптами, gRPC-редактором, синхронизацией и окружениями\u00a0— каждый ведёт на свою страницу.',
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
        'gRPC-метаданные приходят в скрипт копией\u00a0— сохранённый запрос скрипт больше не меняет.',
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
        'Экран приветствия при первом запуске: работать локально или подключить аккаунт\u00a0— выбор можно изменить в любой момент.',
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
        'Текущий воркспейс теперь синхронизируется сразу после входа\u00a0— иконка облака больше не остаётся серой.',
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
        'Облачный сервер Tetiva подключён по умолчанию\u00a0— адрес вводить не нужно.',
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
