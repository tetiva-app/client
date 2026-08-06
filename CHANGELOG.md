# Changelog

## [Unreleased]

### Безопасность

- **MCP DevTools больше не открывает порт наружу.** Сервер слушал `:9300`, то есть все сетевые интерфейсы, и без аутентификации отдавал любому в локальной сети воркспейсы, коллекции, запросы и переменные окружения, а также позволял выполнять запросы от имени пользователя. Теперь адрес по умолчанию — `127.0.0.1:9300`
- Адрес вида `:9300` (без хоста) приводится к `127.0.0.1:9300` при старте — это чинит и уже сохранённые настройки тех, кто включал MCP раньше. Явно указанный хост (`0.0.0.0:9300`, адрес в локальной сети) сохраняется как есть: это осознанный выбор
- При старте на не-loopback адресе в лог пишется предупреждение, а в настройках рядом с полем адреса появляется заметное предупреждение о доступе из сети без аутентификации
- Инструменты MCP больше не отдают секреты: `auth_data` запросов и коллекций (bearer-токены, пароли basic-auth) заменяется на `[redacted]`, сохраняя только признак наличия авторизации и её тип; значения переменных с флагом «секрет» не возвращаются в `list_variables`, `get_environment` и в ответе на создание переменной. Приём значений на запись не изменился

## [v0.16.0] — 2026-07-27 — Экран приветствия и подтверждение почты

### Добавлено

- Экран приветствия при первом запуске: выбор «работать локально» или «подключить аккаунт», без принуждения к регистрации. Тексты на русском и английском по системной локали
- Необязательный тур из четырёх экранов — протоколы, gRPC из `.proto`, скрипты и тесты, воркспейсы. Открывается по ссылке и переоткрывается из настроек пунктом «Показать приветствие»
- После регистрации приложение сообщает, что отправило письмо, и ждёт подтверждения: автопроверка статуса, повторная отправка письма, кнопки «Я подтвердил» и «Выйти»
- Индикатор в панели слева показывает, что почта не подтверждена, и открывает окно синхронизации по клику

### Изменено

- Синхронизация включается только после подтверждения почты. Локальная работа доступна сразу и никак не ограничена; при недоступном сервере синхронизация запускается как раньше

### Исправлено

- Refresh-токен больше не затирается в локальной базе при входе и регистрации — сессия переживает перезапуск, даже если системная связка ключей недоступна

## [v0.15.3] — 2026-07-16

### Исправлено

- Windows: исправлено определение окружения WebView2 — приложение выполняет реальные запросы вместо показа демо-данных
- Windows: инсталлер и свойства exe-файла показывали «My Product» вместо Tetiva

## [v0.15.2] — 2026-07-14

### Изменено

- Повышена стабильность и надёжность соединения с облаком

## [v0.15.1] — 2026-07-13

### Исправлено

- **Активный воркспейс теперь подключается к синхронизации сразу после входа.** Раньше подключение к серверу зеркалило удалённые воркспейсы, но не линковало активный локальный — движок синхронизации для него не запускался, и облако в индикаторе оставалось серым. Теперь при подключении, регистрации и старте приложения активный воркспейс без remote-привязки автоматически создаётся на сервере и включается в синхронизацию
- В окне Sync метка «Tetiva Cloud» больше не выбивается из строки (иконка облака вернулась на базовую линию текста)

## [v0.15.0] — 2026-07-12

### Добавлено

- **Уведомления об обновлениях**: точка-бейдж на иконке настроек, когда доступна новая версия; проверка не чаще раза в 10 дней с тумблером «Check for updates automatically» (включён по умолчанию, приватность — при выключенном тумблере запросов нет); манифест берётся с нашего sync-сервера
- **Окно «Что нового»** показывается один раз после установки версии с заметками; доступно в любой момент из настроек. Язык (RU/EN) выбирается по системной локали, заметки встроены в приложение и работают полностью офлайн

## [v0.14.1] — 2026-07-12

### Исправлено

- **Ошибки операций больше не проглатываются молча** — любые неудачные создания/удаления/сохранения показывают тост с причиной
- **Сохранение запроса стало безопасным**: текст, набранный во время сохранения, больше не затирается ответом сервера; повторный Cmd+S во время сохранения не создаёт параллельный конфликтующий запрос; вкладка не закрывается, если сохранение не удалось
- **Cmd+S / Cmd+Enter больше не срабатывают дважды** на GraphQL- и gRPC-вкладках и не «протекают» сквозь модальные окна; Cmd+[ / Cmd+] не конфликтуют с отступами в редакторе кода
- **Cmd+W закрывает активную вкладку**, а не всё приложение (меню приложения настроено явно)
- Диалоги создания/переименования защищены от двойного сабмита и не закрываются при ошибке
- Удаление коллекции закрывает вкладки всех вложенных коллекций и запросов
- Нативное контекстное меню снова доступно в текстовых полях (copy/paste/спеллчек)

## [v0.14.0] — 2026-07-12

### Добавлено

- Подключение к синхронизации по умолчанию использует облачный сервер Tetiva Cloud — вводить адрес вручную больше не нужно
- Свой сервер указывается за тоглом «Use custom server»; адрес нормализуется автоматически (срезаются `https://` и хвостовой `/`, при отсутствии порта добавляется `:443`)

### Изменено

- В окне Sync подключение к облаку отображается как «Tetiva Cloud» вместо технического адреса сервера

## [v0.13.1] — 2026-07-12

### Исправлено

- **Sync больше не «отваливается странно» после сна Mac.** Срок жизни access-токена считался по монотонным часам Go, которые на macOS останавливаются во сне: после пробуждения клиент до ~13 минут предъявлял серверу давно истёкший токен, получал Unauthenticated и уходил в общий backoff до 5 минут. Теперь `expiresAt` хранится без монотонной составляющей (`Round(0)`), а ответ `Unauthenticated` на любом sync-RPC сбрасывает кэшированный токен и форсирует refresh на следующей попытке.
- **Отклонённый refresh-токен больше не приводит к вечному тихому ретраю.** Если сервер отвечает Unauthenticated на сам Refresh (сессия отозвана/потеряна), syncer останавливается и переходит в новое состояние `auth_expired`; иконка sync показывает «Session expired — sign in to resume sync» вместо бесконечного CloudOff.
- **Индикатор sync перестал хаотично мигать.** Движок держит по syncer'у на каждый workspace организации, и событие `sync:status` любого фонового workspace перещёлкивало глобальную иконку; поллинг через 5 секунд возвращал её обратно. Теперь индикатор учитывает события только активного workspace.
- **Refresh-токен всегда зеркалируется в SQLite-фолбэк.** Раньше копия в БД обновлялась только при сбое keychain; если позже загрузка упиралась в 2-секундный таймаут keychain (типично сразу после сна), фолбэк отдавал пустой или давно ротированный токен и sync уходил в offline на цикл backoff.

## [v0.13.0] — 2026-07-12

### Изменено

- **Ребрендинг: приложение переименовано в Tetiva.** Имя окна, welcome-экран, окно настроек, заголовки вкладок, MCP-сервер («Tetiva DevTools») и пресеты MCP-конфигов (`mcpServers.tetiva`), bundle (`productName: Tetiva`, `productIdentifier: yudinsv.com.Tetiva`).
- **Автоматическая миграция данных со старого имени — ничего настраивать не нужно.** (1) Каталог данных: при первом старте `~/.gophercourier` переименовывается в `~/.tetiva` (SQLite, WAL, настройки переезжают как есть); если переименование невозможно, приложение продолжает работать со старым каталогом. (2) Keyring: refresh-токен sync-логина читается из нового сервиса `tetiva`, при отсутствии — подхватывается из legacy-сервиса `gophercourier` и переносится; Logout чистит оба. (3) Env-переменные: `TETIVA_DATA_DIR`/`TETIVA_MCP`/`TETIVA_MCP_ADDR` — новые имена, старые `GOPHERCOURIER_*` продолжают работать как fallback.

## [v0.12.1] — 2026-06-16

### Исправлено

- **Бинарные файлы Office (xlsx/docx/pptx) больше не показываются как «белиберда».** Определение бинарности ответа по `Content-Type` перешло с наивного поиска подстроки на корректный разбор media type. Раньше тип вроде `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` ошибочно считался текстом, потому что внутри подстроки `openxmlformats`/`spreadsheetml` встречается `xml` — и сырые байты ZIP-архива рендерились как текст вместо кнопки «Save to file». Учитываются структурные суффиксы `+xml`/`+json` (RFC 6839), поэтому `image/svg+xml`, `application/atom+xml`, `application/ld+json` по-прежнему распознаются как текст.
- **Заголовок `Content-Disposition: attachment` теперь принудительно трактует ответ как файл на скачивание** независимо от `Content-Type` (например, `text/csv` с `attachment` предлагается сохранить, а не показывается inline).

## [v0.12.0] — 2026-06-15

### Добавлено
- Поддержка протокола **WebSocket** (Raw, RFC 6455): подключение к `ws://`/`wss://`, отправка текстовых сообщений (кнопкой Send или **Cmd/Ctrl+Enter**), живой лог входящих/исходящих сообщений с таймстампами и направлением, индикация состояния соединения, селектор активного окружения. Переменные окружения (`{{var}}`) подставляются в URL; заголовки и auth применяются на handshake. Лог эфемерный (живёт пока открыт таб), факт подключения пишется в историю.

## [v0.11.0] — 2026-06-14 — MCP-настройки в окне настроек

### Добавлено

- **Секция MCP / DevTools в настройках.** Статус сервера (Running/Stopped), URL SSE-эндпоинта и «Copy config» с пресетами для AI-клиентов (Claude Desktop / Cursor / raw URL).
- **Управление MCP-сервером из UI:** переключатель Enable и порт. Сохраняются и применяются при следующем запуске приложения («Restart required to apply»). Если MCP задан env-переменными (`GOPHERCOURIER_MCP` / `GOPHERCOURIER_MCP_ADDR`) — поля заблокированы, показывается «Managed by environment variables».

### Технические детали

- Новый generic KV-слой настроек: таблица `app_settings`, domain `settings`-usecase, SQLite-репозиторий — фундамент для будущих настроек.
- `internal/app/mcp.go` теперь всегда подключает MCP-модуль; SSE стартует в lifecycle-хуке после готовности БД, env переопределяет персистентные значения.
- Wails `SettingsService` (`GetMCPSettings` / `SetMCPSettings`) + фронтовый сервис с mock-режимом.
- Добавлен первый `fx.ValidateApp`-тест — `go test` теперь ловит ошибки DI-графа.

## [v0.10.0] — 2026-06-13 — Настройки: тема, редактор, обновления

### Добавлено

- **Страница настроек (модальное окно)** — разблокирована из ActivityBar, открывается шестерёнкой внизу панели или хоткеем `Cmd/Ctrl+,` (работает даже без открытых вкладок). Изменения применяются мгновенно, без кнопки Save.
- **Переключение цветовой темы: Light / Dark / System.** По умолчанию — `System` (тема следует за настройкой macOS, как в Bruno/Hoppscotch). Тема применяется инлайн-скриптом в `index.html` до первой отрисовки — без вспышки при старте. Режим `System` реактивно реагирует на смену схемы ОС через `matchMedia`.
- **Настройки редактора:** размер шрифта CodeMirror (12/13/14/16 px) и перенос строк (word wrap). Применяются к `CodeEditor` и `CodeViewer` без пересоздания редактора (CM6 `Compartment` + CSS-переменная `--gc-editor-font-size`).
- **Светлая подсветка синтаксиса** для редакторов — читаемая на белом фоне (раньше One Dark был захардкожен и нечитаем в светлой теме).
- **Секция About:** версия приложения и проверка обновлений через GitHub Releases API. Любая ошибка (приватный репозиторий → 404, rate-limit, сеть, таймаут) показывает нейтральное «Couldn't check for updates», а не ложное «up to date».

### Технические детали

- Настройки хранятся в `localStorage` (каждый доступ обёрнут в try/catch) и синхронизируются между окнами приложения через `storage`-событие.
- Версия прокидывается во фронтенд через Vite `define` (`__APP_VERSION__`) из `package.json` — единый источник с Go `AppVersion`.

## [v0.9.8] — 2026-06-10 — Фикс browser mock mode + выравнивание сайдбара

### Исправлено

- **Browser mock mode ломался после первого же ленивого импорта Wails runtime** (`frontend/src/services/index.ts`). Детект окружения `isWailsEnvironment()` проверял `window._wails` при каждом вызове, но npm-пакет `@wailsio/runtime` устанавливает `window._wails` side-effect'ом самого импорта — даже в обычном браузере. Любой `await import('@wailsio/runtime')` (события в App.vue, диалоги, клипборд) «превращал» браузер в Wails-окружение, и сервисы, создаваемые после этого (например RequestService при первом создании запроса), выбирали Wails-реализацию → все вызовы падали 404 на `/wails/runtime`. Теперь окружение детектится один раз при загрузке модуля сервисов: в настоящем Wails-окне инжектированный `runtime.js` выставляет `_wails` до исполнения модулей приложения, так что boot-time проверка надёжна, а поздние импорты больше не влияют.
- **Имена коллекций и запросов в сайдбаре центрировались вместо выравнивания влево** (`CollectionItem.vue`, `RequestItem.vue`). Строки дерева — `<button>`, а дефолтный UA-стиль кнопок `text-align: center` наследовался текстовым span'ам. Добавлен `text-left`. Баг был виден и в Wails-окне.

## [v0.9.7] — 2026-05-20 — Keyring scoping per client_id

### Исправлено

- **macOS Keychain entry теперь scoped по `client_id`** (account = `refresh_token:<client_uuid>` вместо константного `refresh_token`). Раньше два Wails-клиента, запущенные на одном Mac под одним email, перезаписывали refresh token друг друга в keychain — на refresh сервер возвращал `client_id mismatch: unauthorized`, и второй клиент терял авторизацию. Теперь каждая инсталляция использует свой keyring entry, изолированный по per-install `client_id`. Legacy unscoped запись best-effort удаляется на `Logout` для cleanup. SQLite fallback не изменился (уже изолирован per-install через `DATA_DIR`).

## [v0.9.6] — 2026-05-20 — Recovery sync_queue.sending на старте

### Исправлено

- **`sync_queue` rows застрявшие в `status='sending'` теперь сбрасываются в `pending` на старте приложения** (`ResumeOnStartup` вызывает уже существовавший `SyncQueueRepo.ResetSending`). Раньше при крэше клиента в середине push'а строки оставались навсегда в `sending` — push worker читает только `pending`, и операции терялись (никогда не ретраились). Reset вызывается **до** проверки конфигурации sync — гарантирует recovery даже если sync был временно отключён между сессиями.

## [v0.9.5] — 2026-05-18 — MCP sync_pause / sync_resume / sync_disconnect_stream

### Добавлено

- **Три новых MCP-tool'а** для симуляции offline-сценариев. Раньше LWW-конфликты было трудно воспроизвести — после realtime-доставки version бампается локально, и client-side optimistic-lock рубит stale-edit ДО того как он дойдёт до server's LWW resolver. Теперь можно открыть контролируемое offline-окно.

  | Tool | Что делает |
  |---|---|
  | `sync_pause { workspace_id }` | Останавливает push/pull/realtime applier на workspace. Outbox queue копит локальные правки. Idempotent. |
  | `sync_resume { workspace_id }` | Восстанавливает: drain queue → pull increments → re-subscribe. Те же state transitions что и cold-start (Pushing → Pulling → Subscribing → Connected). Idempotent. |
  | `sync_disconnect_stream { workspace_id }` | Дропает текущий subscribe stream один раз. Existing exponential backoff (5s–5min) в `subscribeLoop` берёт инициативу на себя. Для тестирования reconnect + `ResyncRequired` provoke. |

- **Engine API**: `engine.Pause(workspaceID)`, `engine.Resume(workspaceID)`, `engine.DisconnectStream(workspaceID)` — все idempotent, race-safe (test `PauseResume_ConcurrentSafe` под `-race`), возвращают clear errors на bogus IDs (no panic).

### Технические детали

- **`Pause`** ставит `paused bool` на `workspaceSyncer` под `ws.mu` + `ws.cancel()` стопит goroutine. Syncer остаётся в `engine.workspaces`, чтобы `Resume` мог прочитать `remoteWorkspaceID` + `lastSyncSeq`.
- **`Resume`** удаляет paused syncer из map и вызывает `StartWorkspace` — fresh goroutine с тем же remote ID + last seq.
- **`DisconnectStream`** — каждый `subscribe()` теперь создаёт per-stream child context (`streamCtx`); `DisconnectStream` отменяет его. `subscribeLoop` видит `nil` error и входит в существующий backoff path.
- **`InjectRawSyncer`** — test-only helper для white-box MCP-тестов без поднятия gRPC.

### Тестирование

- 13 новых unit-тестов в `engine_test.go` (idempotency, queue retention при Pause, state transitions, concurrent safety под `-race`).
- 5 smoke-тестов MCP-tool handler'ов в `server_test.go` (unknown workspace → error, инжектированный syncer → success).
- Все тесты + `-race` зелёные.

## [v0.9.4] — 2026-05-18 — Sync wiring V2 + status indicator реактивен

### Исправлено

- **Подключение sync снова работает после серверного редизайна workspaces V2.** Remote workspaces не подтягивались, `StartWorkspace` не вызывался — клиент висел в "Not connected" даже при доступном сервере. Теперь выполняется полная цепочка `OrgService.ListMine → WorkspaceService.ListByOrg → upsert workspace в SQLite → StartWorkspace per workspace`.
- **`SyncAuthManager.storeTokens` теперь сохраняет `active_org_id`** из login/register/refresh response в `sync_config`. Без этого сервер падал на `pickActiveOrg` при subscribe (нет дефолтного active org → 400). Сигнатура обновлена: `storeTokens(access, refresh, activeOrgID string)`.
- **2-секундный таймаут на `keyring.Set/Get`** — keyring внутри Wails-sandbox иногда виснет навсегда (особенно при modal-приглашении macOS Keychain). Без таймаута весь sync-startup застревал. Падение на timeout fall-back'ится на ephemeral access-token, refresh-токен теряется до следующего login (deliberate trade-off — UI не вешается).
- **`SyncStatusIndicator` стал реактивным** — TopBar-иконка раньше висела серой ~5с после рестарта, даже когда engine уже был `connected`. Корень: компонент только поллил `GetStatus()` каждые 5 с, не подписываясь на `sync:status` event, который Go эмитит в `engine.go:254` на каждый state-transition (`Pushing` → `Pulling` → `Subscribing` → `Connected`). Добавлен `Events.On('sync:status', …)` в `onMounted` — состояние теперь меняется мгновенно. Поллинг оставлен только для `pending`-счётчика (event этим не покрывает).

### Тестирование

- `internal/infrastructure/sync/auth_test.go` обновлён под новую сигнатуру `storeTokens` (передача `activeOrgID`).

## [v0.9.3] — 2026-05-07

### Исправлено
- **Переключение типа body больше не чистит содержимое** — раньше при смене Body Type (например JSON → XML) текущее тело пропадало (на самом деле сохранялось в per-type draft, но визуально воспринималось как «всё стёрло»). Теперь переходы внутри text-family (`json` ↔ `xml` ↔ `raw`) сохраняют один и тот же текст, меняется только парсер/подсветка/Content-Type. Form/Binary как и прежде используют отдельные drafts (там другая структура).

### Добавлено
- **JSONC-комментарии в JSON-теле запроса** (паритет с Postman) — теперь в JSON-body можно писать `// line` и `/* block */` комментарии, чтобы отключать строки без удаления. Перед отправкой комментарии вырезаются на бэкенде (`request.stripJSONC`); в History пишется оригинальный текст с комментариями. Комментарии корректно обрабатывают строки с `//` или `/*` (не путаются внутри строковых литералов).
- CodeMirror JSON-линтер больше не подчёркивает JSONC-комментарии как ошибку синтаксиса.

### Тестирование
- Go: `TestStripJSONC` (line/block/строки/EOF/незакрытый блок), `TestExecute_StripsJSONCCommentsFromWireBody` (wire-тело без комментов + валидный JSON, history с оригиналом).
- Frontend: `useBodyDrafts.test.ts` — `isTextFamily`/`isTextFamilyTransition` + регрессия на сохранение/восстановление drafts.

## [v0.9.2] — 2026-04-27

### Изменено
- **Редизайн фильтров History-сайдбара** — inline-панель занимала ~280px вертикали (примерно половину сайдбара), оставляя место для 1–2 записей. Теперь фильтры компактнее:
  - **Постоянная toolbar-строка ~36px**: поле поиска URL (с magnifier icon, debounced) + кнопка-воронка с badge активных фасетов (Protocol+Status).
  - **Popover** (открывается кликом по кнопке-воронке) содержит только chip-toggle для Protocol (HTTP/gRPC/GraphQL) и Status (2xx/3xx/4xx/5xx/error) + Clear filters. URL-search вынесен наружу как самый частый use-case.
  - Закрытие popover по: повторному клику на кнопку, Escape, click-outside.
  - Header стал минималистичным: только заголовок «History» + trash-icon для Clear all (filter-кнопка убрана из header'а).
- Экономия вертикали ~80%: 280px → 36px idle.

## [v0.9.1] — 2026-04-26

### Исправлено
- **Replay падал с `CHECK constraint failed: json_valid(auth_data)`** — `request.CreateDraftFromHistory` строил draft без явного `AuthData`, оставляя поле пустой строкой `""`, что не является валидным JSON и нарушало CHECK-ограничение SQLite на колонке `requests.auth_data`. Теперь draft создаётся с безопасными дефолтами для всех колонок под `json_valid(...)`: `AuthData = "{}"`, `GRPCMetadata = map{}`.

### Тестирование
- Добавлен unit-test `TestCreateDraftFromHistory_PassesJSONCheckConstraints`, фиксирующий что draft несёт корректные дефолты.
- Добавлены sqlite-интеграционные тесты `TestRequestRepo_Create_RejectsEmptyAuthData` (доказывает что constraint реально срабатывает на пустой строке) и `TestRequestRepo_Create_AcceptsDraftDefaults` (проверяет что entity-форма draft'а проходит все json_valid CHECK'и на реальной схеме).

## [v0.9.0] — 2026-04-26

### Добавлено
- **History (история запросов)** — новая секция в ActivityBar (раньше была заглушка `Coming soon`). Сайдбар показывает последние 200 исполнений в текущем workspace, отсортированных по `created_at DESC`, сгруппированных по датам (Today / Yesterday / This week / Earlier). Записи денормализованы — содержат method, URL, request/response headers и body, status, duration, error.
- **Filter popover** — фильтрация по протоколу (HTTP/gRPC/GraphQL), по диапазону статуса (2xx/3xx/4xx/5xx/Error), и по подстроке URL (case-insensitive). Активный фильтр маркируется точкой на иконке.
- **History viewer в main pane** — read-only просмотр одной записи: Request (Headers, Body), Response (Headers, Body), Error (если был). Открывается кликом по item.
- **Replay** — кнопка в карточке записи и в viewer. Создаёт временный draft request (флаг `is_draft=1`), открывает его в новом табе, помечает badge'ом `Draft`. Пользователь может править и нажать Send (выполнится как обычный запрос — добавит новую запись в history). Закрытие таба автоматически hard-delete'ит draft. Replay сопровождается toast-уведомлением.
- **Save as request →** — в draft-табе в RequestEditor появляется баннер с кнопкой, открывающей диалог выбора коллекции и имени; после подтверждения draft превращается в постоянный request.
- **History кнопка в RequestEditor** (UrlBar) — переключает секцию на History с предзаполненным фильтром по `requestId`. Показывает только записи именно этого запроса. В History сайдбаре виден chip «Filtered by request» с кнопкой сброса.
- **Clear all** — удаление всей истории текущего workspace через ConfirmDialog. Удаление одной записи — trash-иконка на hover. Доступно из item-row и из header сайдбара.
- **Load more** — кнопка в footer'е (`200 of N records — Load more`), увеличивает offset на 200.
- **Cleanup leftover drafts on app start** — fx OnStart hook удаляет все draft'ы (`is_draft=1`) при запуске приложения. Защита от случая, когда приложение упало с открытым draft-табом.

### Изменено
- **`request.Repository.List` теперь фильтрует `is_draft=0`** — draft-запросы исключены из обычных списков, sidebar tree, search и export. Видны только через `GetByID` (нужно для открытия таба) и в табах через `requestsMap`.
- **`HistoryRepository` интерфейс расширен** — добавлены методы `GetByID`, `List`, `Count`, `Delete`, `DeleteAll`. До этого был только `Create`.
- **Wails: новый сервис `HistoryService`** (5 методов: List, GetByID, Delete, Clear, Replay) и два новых метода в `RequestService` (`DeleteDraft`, `PromoteDraft`).
- **`HistoryRepo.Create`** теперь корректно записывает `request_id = NULL` для orphan-history (когда исходный request уже удалён) — соответствует FK `ON DELETE SET NULL`. Раньше писалось пустой строкой и нарушало FK.

### Migration
- Миграция `013_request_drafts.sql` добавляет колонку `is_draft INTEGER NOT NULL DEFAULT 0` в `requests` + частичный индекс `idx_requests_drafts WHERE is_draft = 1`. Существующие записи получают `is_draft=0` автоматически.

### Known limitations
- Replay не восстанавливает pre/post-script и auth — `entities.History` хранит только resolved request/response (это by design, чтобы фиксировать «что было реально отправлено»). Tooltip кнопки Replay явно говорит про это.
- StatusKind 2xx-5xx исключают строки где `error_message != ''` — частичный сбой (server replied with 200 but transport failed) классифицируется только как Error, не как 2xx. Контракт зафиксирован тестом.
- Большие response_body (> 1 MB) хранятся в SQLite как есть. На текущей стадии проблемы не наблюдается; будет переработано если станет узким местом.

## [v0.8.1] — 2026-04-26

### Изменено
- **Внутренний рефакторинг:** `Result[T]` / `ResultError` / `Empty` / `OK` / `Err` / `ErrCode*` перенесены из `internal/domain/` в `internal/adapters/wails/`. Это восстанавливает clean-architecture-инвариант «domain без JSON-тегов» — конверт Wails JSON-RPC жил в неправильном слое. Поведение и wire-формат не изменились (поля `data` / `error` / `code` / `message` / `fields` остались байт-в-байт идентичны). Frontend и сохранённые данные не затронуты.

### Тестирование
- Добавлены contract-тесты, фиксирующие точный JSON-формат `Result`/`Err`/`Empty` (8 кейсов с byte-equality).
- Добавлено покрытие happy + error для каждого публичного метода всех 9 Wails-сервисов (66 методов, 111 тестов в матрице) — служит safety-net для будущих изменений сигнатур.

## [v0.8.0] — 2026-04-25

### Добавлено
- **Таб Cookies в response viewer.** Read-only список cookies для текущего запроса с двумя секциями:
  - *Sent* — какие куки клиент приложил к этому запросу (через jar).
  - *Received* — `Set-Cookie` из ответа со всеми атрибутами (Domain, Path, Expires, HttpOnly, Secure, SameSite).
- **Cookie Manager (модалка).** Двухпанельный UI: список доменов слева с counter, таблица cookies для выбранного домена справа. Поддерживает: ручное добавление cookie, редактирование (PUT-семантика — все поля), удаление по одной, очистку домена целиком, очистку всего jar в workspace. Триггеры: ссылка `Manage cookies →` в табе Cookies.

### Изменено
- **Cookie jar теперь persisted в SQLite и scoped per-workspace.** Куки сохраняются между запусками приложения и изолированы между воркспейсами — переключение workspace показывает другой набор cookies. Раньше jar был in-memory и process-wide; терялся при рестарте, общий между всеми workspaces.
- **`HTTPRequester` использует workspace-scoped jar для каждого запроса.** Per-call `workspaceJar` адаптер реализует `http.CookieJar` поверх SQLite-репозитория, фильтрует по `workspace_id` всю запись и чтение.
- **Response теперь содержит resolved URL** (`Response.URL`) — нужно для корректного matching cookies в табе при использовании env-переменных в URL (`{{gateway_host}}/...`).

### Cookie semantics (RFC 6265)
- **Domain matching:** добавлен флаг `host_only`. Если сервер прислал cookie без атрибута `Domain=` — cookie матчится только на exact host (origin), не на subdomains. С атрибутом `Domain=foo.com` — на host и любые subdomains.
- **Delete-cookie semantics:** `Set-Cookie` с `Max-Age=0` (или `Max-Age<0`), либо с `Expires` в прошлом — теперь корректно удаляет существующую запись из jar (раньше upsert'илась как session-cookie с пустым value). Это позволяет серверу очищать session при logout.
- **Path default:** упрощённая семантика — cookies без явного `Path=` получают `Path=/` (RFC требует directory-default-path; принято как известное упрощение).
- **Expiration:** prune-on-read — cookies с `expires_at < now` исключаются из выдачи; session cookies (без expiration) хранятся до ручной очистки.

### Migration
- В предыдущей версии cookie jar был in-memory и обнулялся на каждом старте — данные мигрировать не нужно. После апгрейда jar пуст.

## [v0.7.5] — 2026-04-25

### Изменено
- **Генерация «Copy as cURL» перенесена с фронтенда на бэкенд.** Раньше curl собирался в `frontend/src/lib/codegen/curl.ts` из «сырых» полей запроса и видел только resolved env vars — pre-script, `inherit`-auth от коллекций, multipart с файлами и binary body игнорировались, поэтому скопированная команда могла отличаться от того, что отправляет `Send`. Теперь curl строится через тот же pipeline, что и `Execute`: env vars → pre-script → auth resolver (с обходом иерархии коллекций) → applyAuth → автоматический `Content-Type` → энкодинг тела (multipart `-F 'k=@/path'` для файлов, `--data-binary '@/path'` для binary, `-d '<urlencoded>'` для не-file form). Реализовано через новый usecase-метод `request.BuildCurl` и Wails-эндпоинт `RequestService.GenerateCurl`. gRPC и GraphQL команда возвращает `ValidationError` (curl для них не поддерживается).
- **Pre-script при копировании curl выполняется в режиме dry-run.** Изменения headers и переменных, которые скрипт делает в pre-script, отражаются в итоговой команде, но **не сохраняются в БД** и не пишут запись в history. Так копирование curl остаётся безопасной операцией без побочных эффектов.
- **Cookies из сессионного jar теперь попадают в curl-команду.** Если pre-script или предыдущий запрос (например `auth/login`) положили в process-wide cookie jar `Set-Cookie`, то скопированный curl следующего запроса к тому же домену получит `-b 'name=value; ...'` — поведение совпадает с тем, что отправляет `Send`. Реализовано через новый интерфейс `request.CookieReader` (forward-compatible: уже принимает `workspaceID`, чтобы будущий persisted+per-workspace jar встал без поломки call site'ов).

## [v0.7.4] — 2026-04-25

### Изменено
- **Новая иконка приложения** — заменён `build/appicon.png` на финальный кастомный дизайн (gopher-курьер с рюкзаком на тёмном закруглённом фоне). Перегенерированы `build/darwin/icons.icns` и `build/windows/icon.ico` из 1024×1024 PNG.
- **Отключён Apple Icon Composer pipeline** — удалены `build/appicon.icon/` (Wails SVG-шаблон с градиентом/тенями/translucency) и `build/darwin/Assets.car`, из `Info.plist` и `Info.dev.plist` убран `CFBundleIconName`. Иконка готова целиком (со своим фоном и скруглением), повторная обработка Icon Composer'ом наложила бы лишние эффекты. macOS теперь берёт иконку из `CFBundleIconFile` → `icons.icns`. Из `build/Taskfile.yml` (`generate:icons`) убраны флаги `-iconcomposerinput` и `-macassetdir`.

## [v0.7.3] — 2026-04-24

### Добавлено
- **Версия приложения в заголовке окна** — window title теперь `GopherCourier 0.7.3` вместо просто `GopherCourier`, чтобы по одному взгляду было понятно, какая сборка запущена. Версия живёт в `internal/constants/app.go` (константа `AppVersion` + хелпер `AppTitle()`), бампается вместе с CHANGELOG. `frontend/package.json` синхронизирован с той же версией.

## [v0.7.2] — 2026-04-24

### Исправлено
- **Поиск по сайдбару: matched-папку нельзя было раскрыть осмысленно** — если имя папки совпадало с запросом, а её дети — нет, раскрытие показывало пустоту и выглядело сломанным. Теперь matched-папка считается «прозрачной»: при раскрытии отображаются все её вложенные папки и реквесты, даже если они сами не матчнули. Элементы, попавшие в дерево только как контекст (не являются прямым совпадением), отрисовываются с `opacity-50`, чтобы настоящие совпадения выделялись. Подсветка `<mark>` по-прежнему ставится только на реальные матчи через `useHighlight`.

## [v0.7.1] — 2026-04-24

### Добавлено
- **Кнопка Copy в response toolbar** — иконка-копия справа от поля поиска в body-табе, копирует отформатированное тело ответа через `navigator.clipboard.writeText`. При клике иконка на 1.5 секунды меняется на зелёный checkmark, тултип переключается на `Copied!`. Не требует фокуса в body и работает одинаково для JSON/XML/HTML/text; для binary не показывается. Контекстное меню (Copy Body / Select All) остаётся как запасной путь.

### Исправлено
- **Cmd/Ctrl+A и Cmd/Ctrl+C не работали в теле ответа** — CodeMirror 6 был настроен одновременно с `EditorView.editable.of(false)` и `EditorState.readOnly.of(true)`. Первое снимало `contenteditable` с контента, из-за чего нативное выделение и копирование через DOM не работали; внутренний `state.selection`, которое выставлял ручной Cmd+A-хендлер, не синхронизировалось с DOM selection и в clipboard попадал только первый символ. Оставлен только `readOnly` — `contenteditable` сохраняется, Cmd+A+Cmd+C копирует нативно.
- **Cmd/Ctrl+A копировал только видимую часть длинного ответа** — CodeMirror 6 виртуализирует большие документы и рендерит в DOM только текущий viewport, поэтому нативный Cmd+A внутри `contenteditable` выделял лишь видимые ~60 строк, а не весь буфер. Cmd+A теперь привязан к CM6-команде `selectAll` через `keymap.of(...)` — она выставляет `state.selection` на весь документ, и copy-handler CM6 пишет в clipboard полный текст из `state.doc`, а не DOM-выборку. Проверено на mock-ответе в 329 КБ (2000 пользователей).

## [v0.7.0] — 2026-04-24

### Добавлено
- **Поиск по сайдбару** — инлайн-поле в хедере `Collections`, фильтрует дерево по именам коллекций и реквестов. Поиск выполняется на Go-стороне через SQLite (UNION ALL по collections+requests, Unicode-aware фильтрация в Go для корректной работы с кириллицей). Совпадения подсвечиваются `<mark>`, иерархия сохраняется — папки-предки найденных элементов автоматически раскрываются на время поиска, пользовательское состояние раскрытия (`userExpanded`) не затирается. Реквесты из ни разу не раскрытых папок отображаются через изолированный preview-store — destructive actions (delete/move/rename) и открытие вкладки сначала подгружают полные данные через `ensureFullyLoaded`. Esc, кнопка ×, пустое поле — сбрасывают поиск, дерево возвращается в исходное состояние. Плашка «Showing top 200 results. Refine query» при превышении лимита. Race-condition защита через sequence counter.
- **Cmd/Ctrl+F** — shortcut фокусирует поле поиска сайдбара. В CodeMirror-редакторах (URL-бар, body, scripts) внутренний поиск CodeMirror работает как раньше — `searchKeymap` сам перехватывает `Mod-f` и не даёт событию всплыть.

### Изменено
- В хедере `Collections` убран текстовый лейбл — его место занимает поле поиска. Кнопки Import Postman / New Collection остались слева.


### Исправлено
- **Auto-commit на blur в add-row формах** — при заполнении новой строки (env-переменная, query param, header, form-data поле, gRPC metadata) переменная теперь создаётся при уходе фокуса наружу контейнера, а не только по Enter. Пересылка фокуса внутри add-row (Tab key→value, клик на `+`) не триггерит создание. Enter/Tab/click на `+` продолжают работать как раньше.

## [v0.6.7] — 2026-04-23

### Добавлено
- **Cmd/Ctrl+[ и Cmd/Ctrl+]** — навигация по табам (предыдущий/следующий) с циклическим wrap-around. Дополнение к существующим Cmd+1..9.

## [v0.6.6] — 2026-04-23

### Добавлено
- **Тултип env-переменных с drill-through** — при наведении на `{{var}}` в любом редакторе (URL, body, headers, scripts) показывается компактный поповер: имя переменной, значение, scope-бейдж («Environment» / «Undefined») и action-ссылка. Клик по ссылке открывает `EnvironmentModal`: для определённых — скролл к строке + flash-подсветка (1.2s, accent-цвет) + автофокус на поле значения; для неопределённых — prefill имени в add-row.
- **Cmd/Ctrl+Click на `{{var}}`** — shortcut для power-users, делает то же самое без наведения на тултип.

### Изменено
- Открытие `EnvironmentModal` переведено с локальных `ref` на Pinia-стор `useEnvModalUi`, чтобы vanilla-TS CodeMirror-extension мог триггерить открытие с контекстом (focus/prefill).

## [v0.6.5] — 2026-04-23

### Исправлено
- **Env-переменные не подсвечивались и не резолвились в скриптах коллекции** — `CollectionEditor.vue` не передавал `resolved-variables` и `secret-keys` в `ScriptEditor`, из-за чего CodeMirror-плагин получал пустой объект и все `{{var}}` отображались как undefined (оранжевые), даже если переменная была определена в активном environment. Теперь поведение скриптов коллекции идентично скриптам запроса.

## [v0.6.4] — 2026-04-22

### Добавлено
- **MCP DevTools — новые инструменты и расширение схемы** (покрывает pain points ревью):
  - `list_workspaces`, `get_workspace`, `create_workspace` — больше не нужно лезть в SQLite за UUID рабочего пространства.
  - `update_collection`, `move_collection` — редактирование и смена родителя без пересоздания / прямого SQL.
  - `update_request`, `move_request` — редактирование body/url/method/headers/auth/scripts без delete+create.
  - `send_request` — выполнение сохранённого запроса (HTTP/gRPC/GraphQL) через `request.Execute` с env-резолвом и историей.
- **`create_collection`**: добавлены параметры `parent_id`, `pre_script`, `post_script`, `auth_type`, `auth_data` — теперь можно создавать вложенные коллекции и наследуемые скрипты/auth одним вызовом.
- **`create_request`**: добавлены `protocol`, `headers`, `auth_type`, `auth_data`, `pre_script`, `post_script`, `grpc_service`/`grpc_method`/`grpc_proto_path`/`grpc_metadata`, `graphql_query`/`graphql_variables`/`graphql_schema_path`/`graphql_operation`. Закрыт schema drift между БД и MCP API.
- **Полный объект в ответах** `create_*`/`update_*`/`move_*` — больше не нужно делать отдельный `get_*` для контекста после изменения.

### Изменено
- `mcpadapter.NewServer` теперь принимает `workspace.Usecase` (обновлены FX-модуль `app/mcp.go` и standalone `cmd/mcp/main.go`).

## [v0.6.3] — 2026-04-22

### Исправлено
- **Сайдбар не скроллился при большом количестве коллекций** — `flex-1` у `ScrollArea` без `min-h-0` не позволял flex-ребёнку ужаться ниже контента. Добавлен `min-h-0` в `CollectionTree` и ещё 5 мест с тем же паттерном (EnvironmentModal, RequestEditor, ScriptEditor, CollectionEditor).
- **Chevron не появлялся у subfolder'ов с requests** — `fetchByCollection` вызывался только на expand, поэтому только что отрендеренные subfolder'ы не знали о своих requests. Теперь fetch происходит на mount (lazy-семантика сохранена — CollectionItem mount'ится только если родитель открыт).

### Изменено
- **Единая система тостов** вместо `alert()` — 9 мест (import/export коллекций и environments) теперь используют неблокирующие уведомления с анимацией и автоскрытием.
- **Фидбек при отсутствии активного workspace** — операции import/export больше не молчат, показывают ошибку через toast.
- **Generic `RenameDialog`** — один компонент вместо двух одинаковых (`RenameCollectionDialog` + `RenameRequestDialog` удалены).
- **Composable `useConfirmDelete`** — унифицированный flow подтверждения удаления, применён в `CollectionTree`, `EnvironmentModal`, `WorkspaceSwitcher`.
- **Убраны `as any` в `wails-portability.ts`** — используются типизированные конструкторы DTO из сгенерированных bindings.

### Добавлено
- **Playwright-тесты** на скролл сайдбара и появление chevron'а у subfolder'ов.

## [v0.6.2] — 2026-04-20

### Исправлено
- **Экспорт коллекций и environments не работал** — клик по «Export as Postman» ничего не делал на macOS. WKWebView не триггерит скачивание по blob-URL через `<a download>`. Теперь экспорт открывает нативный Save File диалог (`app.Dialog.SaveFile`) и пишет файл на Go-стороне.
- **HTTP cookies не сохранялись между запросами** — `http.Client` создавался без `CookieJar`, поэтому `Set-Cookie` из ответов не подхватывались и не отправлялись в следующих запросах (ломало auth-flow где access token в body + cookie, refresh только в cookie). Добавлен процессный in-memory `cookiejar.New(nil)`.

## [v0.6.1] — 2026-03-23

### Добавлено
- **Realtime sync** — мгновенное обновление UI при получении данных от других клиентов через subscribe stream
  - Подключение `SyncEngine.EventEmitter` к Wails Event System (`app.Event.Emit`)
  - Фронтенд подписка на `sync:changed` / `sync:entity_updated` в `App.vue` — автоматический `fetchAll` коллекций и environments
- **Управление workspace-ами через sync** — создание remote workspace из UI, auto-sync при подключении
  - `RemoteWorkspaceID` поле в entity + DTO + frontend type
  - `CreateRemoteWorkspace` — создание workspace на сервере и локальная привязка
  - `syncRemoteWorkspaces` — автоматическое подтягивание remote workspace-ов при connect/startup
  - Иконки Cloud/Monitor в WorkspaceSwitcher для визуального различия local/synced
  - Чекбокс "Sync to server" при создании нового workspace
- **MCP DevTools сервер** — HTTP SSE сервер для управления данными через Model Context Protocol
  - 21 инструмент: CRUD для коллекций, запросов, environments, переменных + sync операции
  - Standalone CLI (`cmd/mcp`) для тестирования без Wails
  - Поддержка `GOPHERCOURIER_DATA_DIR` для запуска нескольких клиентов на одной машине
- **Auto-reconnect на старте** — `ResumeOnStartup` восстанавливает sync-соединение из сохранённых credentials
  - Fallback хранения refresh token в SQLite когда OS keychain недоступен (migration 011)

### Исправлено
- **Nil pointer panic при soft-delete sync** — fix Go nil-interface pitfall в `readEntity` (typed nil в `any` не равен `nil`)
- **Soft-delete не синхронизировался** — `buildDeleteProto` создаёт минимальный proto для удалённых записей, которые `GetByID` не находит из-за фильтра `is_delete = 0`
- **Sync декораторы не тригерили push** — добавлены вызовы `NotifyWrite` после успешного enqueue в sync_queue
- **SyncConnectModal** — упрощён UI, убрано ручное linking workspace-ов (теперь автоматически)

## [v0.6.0] — 2026-03-22

### Добавлено
- **Sync Client Integration** — интеграция с gRPC sync-сервером для синхронизации данных между устройствами
  - Repository Decorator + Transactional Outbox — все записи автоматически попадают в sync_queue в рамках одной SQLite TX
  - SyncEngine — state machine с push/pull/subscribe lifecycle, goroutine-per-workspace
  - SyncAuthManager — JWT авторизация с хранением refresh token в OS keychain (go-keyring), proactive refresh через singleflight
  - gRPC клиент — обёртка над auth/sync/workspace сервисами с proto mapper (domain ↔ SyncEntity)
  - 4 SyncedRepo декоратора (collection, request, environment, variable) — перехватывают Create/Update/Delete
  - Context-based TX (WithTx, DBTXFromContext) — все существующие репозитории стали TX-aware
  - FX wiring — named deps для inner/decorated разделения репозиториев
  - Frontend: SyncStatusIndicator (облачко в ActivityBar) + SyncConnectModal (Login/Register + workspace linking)
  - Migration 009: sync_config, sync_queue таблицы, is_synced/remote_workspace_id/last_sync_seq колонки

## [v0.5.1] — 2026-03-22

### Добавлено
- **GraphQL Autocomplete** — автодополнение полей, типов и аргументов в query editor на основе загруженной schema (`cm6-graphql` + `graphql-js`)
  - Syntax highlighting для GraphQL (keywords, типы, аргументы)
  - Linting — подчёркивание невалидных полей/запросов
  - Cmd/Ctrl+Click на поле → переход в Schema tab к документации типа
  - Autocomplete popup со стилизацией под тёмную/светлую тему
- **Schema tab Browse/SDL** — интерактивный браузер типов с поиском вместо отдельной docs panel
  - Operation detail view (аргументы + return type) при выборе операции
  - Кнопка сброса операции ("All operations") в Operation Selector
- **Поиск в GraphQL Response** — Search по ответу как в HTTP
- **Interactive Schema в detached window** — "Open in Window" показывает Browse view вместо raw SDL

### Исправлено
- **GQL бейдж в sidebar** — GraphQL запросы показывают "GQL" (розовый `#E535AB`) вместо "POST"
- **Единый цвет GraphQL** — URL bar, sidebar, диалог создания запроса используют один цвет
- **Сохранение GraphQL полей** — `wails-request.ts` не передавал `graphqlQuery`, `graphqlVariables`, `graphqlSchemaPath`, `graphqlOperation` при save/edit — данные терялись при Cmd+S
- **Variables panel** — содержимое пропадало при сворачивании/разворачивании
- **Диалог создания запроса** — кнопка GraphQL имела другой стиль и не показывала cursor pointer
- **URL bar selection с переменными** — выделение текста в URL с `{{var}}` теперь работает корректно

## [v0.5.0] — 2026-03-15

### Добавлено
- **GraphQL Requester** — поддержка протокола GraphQL (queries + mutations)
  - Introspection + импорт `.graphql` schema файлов
  - Operation Selector с автогенерацией example queries (глубина 3)
  - Split query editor с docs panel + variables
  - Dedicated response viewer с разделением Data/Errors
  - Schema viewer с фильтрацией по операциям
  - Auth tab (общий с HTTP)
  - Pre/Post scripts поддержка (`pm.request.graphqlVariables`, `pm.request.graphqlOperation`)
  - Detachable windows поддержка (schema viewer с language param)
  - Postman import/export совместимость (body.mode="graphql")

## [v0.4.0] — 2026-03-15

### Добавлено
- **Отсоединяемые окна** — система detachable windows для открытия табов в отдельных нативных окнах (Wails v3)
- **Detach Request** — правый клик на вкладке → «Open in Window» для открытия HTTP/gRPC запроса в отдельном окне
- **Schema Viewer Window** — кнопка «Open in Window» в GRPCSchemaViewer для просмотра proto-схемы в отдельном окне
- **Cross-window sync** — Wails Events для синхронизации окружений, коллекций и workspace между окнами
- **Double-open prevention** — клик на detached запрос в sidebar фокусирует существующее окно
- **Window prefs** — автосохранение размера/позиции окон в `~/.gophercourier/window-prefs.json`
- **Auto-save** — автоматическое сохранение при закрытии любого окна
- **Cascade close** — закрытие main window автоматически закрывает все дочерние окна

## [v0.3.0] — 2026-03-14

### Добавлено
- **gRPC поддержка** — unary вызовы, server reflection, импорт .proto файлов (файл/директория)
- **Выбор сервиса/метода** — комбинированный dropdown с поиском и клавиатурной навигацией
- **Вкладка Schema** — proto-определения с подсветкой синтаксиса (CodeMirror + protobuf mode)
- **Generate Example** — генерация примера JSON-тела из proto-схемы
- **Редактор метаданных** — gRPC metadata через KeyValueEditor
- **Наследование метаданных** — коллекции передают default metadata вложенным gRPC запросам
- **Pre/post скрипты для gRPC** — `pm.request.metadata`, `pm.request.grpcMethod`, `pm.request.protocol`, `pm.response.statusText`
- **Бейдж протокола** — HTTP (синий) / gRPC (фиолетовый) в sidebar
- **Выбор протокола** — HTTP / gRPC toggle при создании нового запроса

## [v0.2.5] — 2026-03-14

### Добавлено
- **Rename Request** — переименование запросов через контекстное меню в sidebar

## [v0.2.4] — 2026-03-14

### Исправлено
- **Copy as cURL** — дублирование заголовка Content-Type, если он уже задан в Headers вручную

## [v0.2.3] — 2026-03-14

### Добавлено
- **Переменные окружения в Auth-полях** — подсветка `{{переменных}}`, автокомплит и hover-tooltip в полях Bearer Token, Basic Auth и API Key
- **Компонент VariableInput** — переиспользуемый input/textarea с поддержкой переменных окружения (overlay-подсветка, autocomplete, tooltip)

## [v0.2.2] — 2026-03-14

### Исправлено
- **Move to... для одиночных элементов** — пункт «Move to...» теперь доступен в контекстном меню отдельной коллекции и запроса, а не только при мультиселекте

## [v0.2.1] — 2026-03-14

### Исправлено
- **Секретные переменные в body** — tooltip и автокомплит в CodeMirror теперь маскируют секретные env-переменные (`••••••••`)
- **Ложные ошибки JSON в body** — `{{variable}}` паттерны больше не подсвечиваются красным как синтаксические ошибки
- **Пустой body** — пустой JSON-редактор больше не показывает ошибку валидации

## [v0.2] — 2026-03-14

### Добавлено
- **Workspaces** — изолированные контейнеры для коллекций, окружений и истории
  - CRUD воркспейсов (создание, переименование, удаление)
  - Переключатель воркспейсов в sidebar с dropdown-меню
  - Автоматическая активация другого воркспейса при удалении активного
  - Автоматическое переключение на только что созданный воркспейс
  - Запрет удаления последнего воркспейса
  - Изоляция данных: коллекции, environments и история привязаны к воркспейсу
  - 20 новых Go-тестов (6 repo + 14 usecase)
- **Кнопка "..." в sidebar** — overflow menu для коллекций и запросов при hover (дублирует правый клик)
- **Блокировка нативного контекстного меню** — убрано системное меню (Look Up, Translate) в пользу кастомных

### Исправлено
- Текст пустого состояния: убрана некорректная подсказка Cmd+N

## [v0.1] — 2026-03-11

### Добавлено
- **Collection Detail Backend** — персистенция описания и авторизации коллекций
  - Описание коллекции (Markdown) с предпросмотром
  - Авторизация на уровне коллекции (Basic, Bearer, API Key)
  - Наследование авторизации: запросы с типом `inherit` берут auth из родительской коллекции
  - AuthResolver обходит иерархию коллекций (до 50 уровней)
  - Импорт/экспорт описания и авторизации коллекций в формате Postman
  - Unified dirty tracking для description/auth/scripts в CollectionEditor
  - 14 новых Go-тестов (AuthResolver, Create/Edit validation, Postman import/export)
- **Сохранение бинарных ответов** — детекция бинарных HTTP-ответов по Content-Type с предложением сохранить файл
  - Автоматическое определение бинарного контента (image/*, audio/*, video/*, application/pdf, zip и др.)
  - Сохранение raw bytes во временный файл вместо конвертации в строку
  - Кнопка "Save to file" с нативным диалогом сохранения
  - Парсинг имени файла из Content-Disposition (включая RFC 5987 кодировку)
  - Защита от path traversal при сохранении
  - 45 новых Go-тестов
- **Headers enabled/disabled persistence** — состояние включённости заголовков теперь сохраняется в БД
  - `HeaderItem` struct с полями key/value/enabled вместо `map[string][]string`
  - Отключённые заголовки не отправляются при выполнении и не попадают в cURL
  - Обратная совместимость: старый формат JSON автоматически мигрируется при чтении
  - Postman import/export сохраняет disabled-состояние заголовков
- **Form Data файловые поля** — поддержка типа Text/File для form-data полей
  - Переключатель типа поля в FormEditor с кнопкой Browse для файлов
  - Автоматическое multipart/form-data encoding при наличии файловых полей
  - Postman import теперь включает файловые поля (ранее пропускались)
- **Multi-select в sidebar** — Cmd+Click для добавления в выделение, Shift+Click для диапазона
- **Массовое удаление** — Delete/Backspace для удаления выделенных элементов с диалогом подтверждения
- **Move to...** — перемещение выделенных коллекций/запросов в другую папку через диалог с деревом
- **Collection Move** — бэкенд: перемещение коллекций с валидацией циклических зависимостей (5 тестов)
- **Request Move** — бэкенд: перемещение запросов между коллекциями (3 теста)
- **Pre/Post-request скрипты** на JavaScript (Goja engine)
  - Частичная совместимость с Postman: pm.environment, pm.request, pm.response, pm.test, console.log
  - Скрипты на уровне коллекции с наследованием по иерархии (Request → Collection → Parent)
  - Pre и post скрипты наследуются независимо
  - Таб "Scripts" в редакторе запроса с переключателем Pre-request / Post-response
  - Контекстное меню "Edit Scripts" для коллекций с модальным редактором
  - Таб "Tests" в просмотрщике ответа с результатами тестов и консольным выводом
  - Автосохранение переменных из скриптов в активное окружение
  - JavaScript syntax highlighting в CodeMirror 6
- **Environment Variables** — управление окружениями (Dev/Staging/Prod) с переменными
  - CRUD для environments и variables (Go usecase + SQLite + 28 новых тестов)
  - Секретные переменные (`is_secret`) — маскируются в UI, не будут синкаться на сервер
  - Подстановка `{{variable}}` в URL, headers, body и auth при Execute
  - Environment Selector dropdown в URL bar — быстрое переключение за 1 клик
  - Management Modal (split-layout) — список env + редактор переменных
  - Подсветка `{{var}}` в URL bar — фиолетовый для resolved, оранжевый для undefined
  - Tooltip при наведении — показывает resolved значение (секреты замаскированы)
  - Activity Bar иконка Environments → открывает модалку
  - Mock service с pre-populated данными (base_url, auth_token, api_version)
- **KeyValueEditor** — извлечён общий компонент из HeadersEditor/ParamsEditor/FormEditor (-158 строк)
- **parseTime** — поддержка SQLite datetime формата в репозиториях
- **Params Editor** — редактирование query-параметров в таблице с двусторонней синхронизацией с URL
- **Body Editor** — полноценный редактор тела запроса (JSON/XML/Raw с CodeMirror, Form URL Encoded, Binary file picker)
- **Auth** — поддержка аутентификации: Basic Auth, Bearer Token, API Key (header/query)
- **Auth в Execute** — автоматическое применение auth при отправке запроса (маппинг в заголовки/URL)
- **Auto Content-Type** — автоматическая установка Content-Type при выборе типа body (JSON → application/json и т.д.)
- **Form encoding** — автоматическая сериализация form-полей в application/x-www-form-urlencoded при Execute
- **Binary body** — поддержка отправки файлов с валидацией существования при Execute
- **Pretty Print** — кнопка форматирования JSON/XML в Body Editor

### Добавлено (ранее)
- **HTTP Requester** — отправка HTTP запросов через кнопку Send или Cmd+Enter
- **Response Viewer** — отображение ответов с подсветкой синтаксиса (CodeMirror 6, тема One Dark Pro)
- **Response Headers** — вкладка с таблицей заголовков ответа
- **History** — автоматическое сохранение каждого выполненного запроса в историю (SQLite)
- **JSON auto-prettify** — автоматическое форматирование JSON ответов
- **Status badge** — цветной badge для статус-кодов (2xx зелёный, 4xx оранжевый, 5xx красный)
- **Error state** — структурированное отображение ошибок с подсказками (connection refused, timeout, DNS)
- **Cmd+1..9** — переключение между открытыми табами по горячим клавишам
- **Custom context menu** — кастомное контекстное меню для response body (Copy Body, Select All)
- **Mock service** — URL-based симуляция всех состояний для тестирования (200, 404, 500, error, slow)

### Исправлено
- Определение Wails v3 окружения (window._wails вместо __wails__)
- Закрытие таба при удалении запроса
- Hover в контекстном меню sidebar (видимый контраст в тёмной теме)
- Cursor pointer на кнопке Send и элементах контекстного меню
