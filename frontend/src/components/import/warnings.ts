import { currentLocale, fill, formatNumber, plural, type Locale, type PluralForms } from '@/lib/locale'
import type { ImportWarning } from '@/services'

export type ImportWarningCode =
  | 'description_truncated'
  | 'example_skipped'
  | 'example_too_large'
  | 'example_payload_too_large'
  | 'example_secrets'
  | 'file_field_dropped'
  | 'file_body_dropped'
  | 'auth_unsupported'
  | 'oauth2_grant_unsupported'
  | 'jwt_field_dropped'
  | 'forms_sent_urlencoded'
  | 'foreign_scripts'
  | 'variables_unnamed'
  | 'unnamed_renamed'
  | 'unnamed_renamed_in'
  | 'name_truncated'
  | 'snapshot_part_missing'
  | 'snapshot_protocol_unsupported'
  | 'snapshot_method_unsupported'
  | 'snapshot_body_unsupported'
  | 'snapshot_ws_message_format'
  | 'snapshot_ws_examples_skipped'
  | 'snapshot_example_status'
  | 'postman_scripts'
  | 'postman_variables_unreadable'
  | 'postman_item_skipped'
  | 'postman_unnamed_folder'
  | 'postman_nested_items_skipped'
  | 'postman_description_type'
  | 'postman_descriptions_differ'
  | 'postman_disabled_script'
  | 'postman_oauth2_token_only'
  | 'postman_example_skipped'
  | 'postman_example_too_large'
  | 'postman_example_payload_too_large'
  | 'postman_example_wrong_type'
  | 'postman_example_not_object'
  | 'postman_example_status'
  | 'postman_example_name_truncated'
  | 'postman_globals'
  | 'insomnia_path_params_differ'
  | 'insomnia_workspaces'
  | 'insomnia_disabled_params'
  | 'insomnia_disabled_metadata'
  | 'insomnia_grpc_no_proto'
  | 'insomnia_vault_secrets'
  | 'insomnia_template_tags'
  | 'insomnia_socketio_skipped'
  | 'insomnia_unit_tests_skipped'
  | 'insomnia_cookies_skipped'
  | 'insomnia_specs_skipped'
  | 'insomnia_folder_headers'
  | 'insomnia_folder_environment'
  | 'insomnia_method_unsupported'
  | 'insomnia_graphql_not_json'
  | 'insomnia_auth_disabled'
  | 'insomnia_apikey_location'
  | 'hoppscotch_variable_conflicts'
  | 'hoppscotch_secrets_empty'
  | 'hoppscotch_headers_copied'
  | 'hoppscotch_auth_off'
  | 'openapi_external_ref'
  | 'openapi_broken_ref'
  | 'openapi_auth_header_param'
  | 'openapi_webhooks_skipped'
  | 'openapi_no_server'
  | 'openapi_relative_server'
  | 'openapi_own_server'
  | 'openapi_trace_skipped'
  | 'openapi_callbacks_skipped'
  | 'openapi_query_style'
  | 'openapi_links_skipped'
  | 'openapi_schemes_together'
  | 'openapi_scheme_undefined'
  | 'openapi_scheme_unsupported'
  | 'openapi_scheme_location_unsupported'
  | 'openapi_implicit_flow'
  | 'har_assets_skipped'
  | 'har_entries_skipped'
  | 'har_unreadable_skipped'
  | 'har_cookies_dropped'
  | 'har_credentials_plain'
  | 'har_files_dropped'
  | 'har_bodies_left_out'

export type SubjectKind = 'collection' | 'folder' | 'request' | 'environment' | 'example' | 'operation'

// single/several hides the count: Russian "one" also covers 21, so it cannot stand for "exactly one".
type WarningMessage = string | PluralForms | { single: string; several: string }

export interface ImportWarningCopy {
  subjects: Record<SubjectKind, { named: string; bare: string }>
  listMore: string
  messages: Record<ImportWarningCode, WarningMessage>
}

const EN: ImportWarningCopy = {
  subjects: {
    collection: { named: 'collection "{name}"', bare: 'collection' },
    folder: { named: 'folder "{name}"', bare: 'folder' },
    request: { named: 'request "{name}"', bare: 'request' },
    environment: { named: 'environment "{name}"', bare: 'environment' },
    example: { named: 'example "{name}"', bare: 'example' },
    operation: { named: '{name}', bare: '' },
  },
  listMore: '{list} and {n} more',
  messages: {
    description_truncated: 'description longer than {max} bytes was truncated',
    example_skipped: 'example "{example}" skipped: {reason}',
    example_too_large: 'example "{example}" skipped: body is larger than {kb} KB',
    example_payload_too_large: 'example "{example}" skipped: with its headers it is larger than {kb} KB',
    example_secrets: 'examples may contain secrets, check them before sharing: {examples}',
    file_field_dropped: 'file field "{field}" was imported without its file; pick it again',
    file_body_dropped: 'the file body was imported without its file; pick it again',
    auth_unsupported: 'auth type "{type}" is not supported and was imported as no auth',
    oauth2_grant_unsupported: 'OAuth 2.0 grant "{grant}" is not supported; pick one in the Auth tab',
    jwt_field_dropped: 'the JWT {field} is not a JSON object and was dropped',
    forms_sent_urlencoded: {
      one: '{n} multipart request has no file field and will be sent as a URL-encoded form, not multipart: {requests}',
      other: '{n} multipart requests have no file field and will be sent as a URL-encoded form, not multipart: {requests}',
    },
    foreign_scripts: '{app} scripts use the {api} API, which Tetiva does not run: rewrite them to pm.* after importing',
    variables_unnamed: {
      one: 'a variable without a name was skipped',
      other: '{n} variables without a name were skipped',
    },
    unnamed_renamed: 'an unnamed {kind} was imported as "{name}"',
    unnamed_renamed_in: 'an unnamed {kind} in "{parent}" was imported as "{name}"',
    name_truncated: 'name longer than {max} characters was truncated',

    snapshot_part_missing: 'the {part} part is missing, the request was skipped',
    snapshot_protocol_unsupported: 'protocol "{protocol}" is not supported, the request was skipped',
    snapshot_method_unsupported: 'method "{method}" is not supported, imported as GET',
    snapshot_body_unsupported: 'body type "{type}" is not supported, imported without a body',
    snapshot_ws_message_format: 'message "{message}" has an unknown format "{format}" and was skipped',
    snapshot_ws_examples_skipped: '{n} example(s) skipped: WebSocket requests have no examples',
    snapshot_example_status: 'example "{example}": status {status} is outside 0–999, imported as 0',

    postman_scripts: 'scripts were imported: only part of the pm.* API is available (pm.environment, pm.collectionVariables, pm.request, pm.response, pm.test, pm.expect), so some may need changes',
    postman_variables_unreadable: 'collection variables could not be read and were skipped',
    postman_item_skipped: 'an unnamed item with neither a request nor children was skipped',
    postman_unnamed_folder: 'an unnamed folder with {n} item(s) was imported as "{name}" rather than failing the import',
    postman_nested_items_skipped: 'items nested under a request were skipped',
    postman_description_type: 'description is {type}, imported as plain text',
    postman_descriptions_differ: 'item and request descriptions differ, request level kept',
    postman_disabled_script: 'disabled {event} script was not imported',
    postman_oauth2_token_only: 'the OAuth 2.0 block has an access token but no token URL; imported as a bearer token',
    postman_example_skipped: 'example {index} skipped: {reason}',
    postman_example_too_large: 'example {index} skipped: body is larger than {kb} KB',
    postman_example_payload_too_large: 'example {index} skipped: with its headers it is larger than {kb} KB',
    postman_example_wrong_type: 'example {index} skipped: field "{field}" must be of type {want}, got {got}',
    postman_example_not_object: 'example {index} skipped: it is {got}, not an object',
    postman_example_status: 'example {index}: status code {status} is out of range, imported as 0',
    postman_example_name_truncated: 'example {index}: name longer than {max} characters was truncated',
    postman_globals: 'Tetiva has no global variables: they were imported as the environment "{name}"',

    insomnia_path_params_differ: {
      single: 'path parameter {names} differs between requests: the environment keeps the first value',
      several: 'path parameters {names} differ between requests: the environment keeps the first value',
    },
    insomnia_workspaces: 'the file has {n} workspaces, each was imported as its own collection: {names}',
    insomnia_disabled_params: {
      one: '{n} disabled query parameter was dropped: Tetiva keeps query parameters in the URL',
      other: '{n} disabled query parameters were dropped: Tetiva keeps query parameters in the URL',
    },
    insomnia_disabled_metadata: {
      one: '{n} disabled gRPC metadata item was dropped',
      other: '{n} disabled gRPC metadata items were dropped',
    },
    insomnia_grpc_no_proto: 'gRPC requests were imported without their proto files: pick a .proto file or use server reflection',
    insomnia_vault_secrets: {
      one: '{n} vault secret was imported as an empty secret variable: fill in its value',
      other: '{n} vault secrets were imported as empty secret variables: fill in their values',
    },
    insomnia_template_tags: {
      one: '{n} Insomnia template tag was left as-is ({names}): Tetiva does not run it, use a variable or a script instead',
      other: '{n} Insomnia template tags were left as-is ({names}): Tetiva does not run them, use variables or scripts instead',
    },
    insomnia_socketio_skipped: {
      one: '{n} Socket.IO request was skipped: Tetiva does not support Socket.IO',
      other: '{n} Socket.IO requests were skipped: Tetiva does not support Socket.IO',
    },
    insomnia_unit_tests_skipped: {
      one: '{n} unit test was not imported',
      other: '{n} unit tests were not imported',
    },
    insomnia_cookies_skipped: {
      one: '{n} cookie from the cookie jar was not imported',
      other: '{n} cookies from the cookie jar were not imported',
    },
    insomnia_specs_skipped: {
      one: '{n} API spec document was not imported',
      other: '{n} API spec documents were not imported',
    },
    insomnia_folder_headers: 'folder headers were not imported',
    insomnia_folder_environment: 'folder environment variables were not imported',
    insomnia_method_unsupported: 'method {method} is not supported and was imported as GET',
    insomnia_graphql_not_json: 'the GraphQL body is not JSON and was imported as the query',
    insomnia_auth_disabled: '{type} auth is disabled in Insomnia and was imported as no auth',
    insomnia_apikey_location: 'an API key sent as a {location} is not supported and was imported as no auth',

    hoppscotch_variable_conflicts: 'variables defined more than once kept their first value: {keys}',
    hoppscotch_secrets_empty: 'Hoppscotch exports secrets without their values: fill in {keys}',
    hoppscotch_headers_copied: 'headers set on Hoppscotch collections and folders were copied into their requests',
    hoppscotch_auth_off: {
      single: "requests in folder {folders} will inherit the parent's auth: Tetiva cannot turn auth off at a folder, set them to None if they must go without",
      several: "requests in folders {folders} will inherit the parent's auth: Tetiva cannot turn auth off at a folder, set them to None if they must go without",
    },

    openapi_external_ref: 'references to "{target}" are not supported and were left empty',
    openapi_broken_ref: 'reference "{ref}" points to nothing and was left empty',
    openapi_auth_header_param: 'the Authorization header parameter of {operations} was not imported: set the auth on the collection or the request instead',
    openapi_webhooks_skipped: 'webhooks were skipped: {names}',
    openapi_no_server: 'the document names no server: set baseUrl in the environment',
    openapi_relative_server: 'the server URL "{url}" is relative: set baseUrl in the environment to the full address',
    openapi_own_server: 'its own server "{url}" was ignored, the request uses {{baseUrl}}',
    openapi_trace_skipped: 'TRACE {path} was skipped: TRACE requests are not supported',
    openapi_callbacks_skipped: 'callbacks were skipped',
    openapi_query_style: 'query parameter "{param}" uses style {style}, which was not reproduced',
    openapi_links_skipped: 'response links were skipped',
    openapi_schemes_together: 'security schemes {schemes} are required together: only "{scheme}" was imported',
    openapi_scheme_undefined: 'security scheme "{scheme}" is not defined: requests that use it were imported without auth',
    openapi_scheme_unsupported: 'security scheme "{scheme}" ({type}) is not supported: requests that use it were imported without auth',
    openapi_scheme_location_unsupported: 'security scheme "{scheme}" ({type} in {in}) is not supported: requests that use it were imported without auth',
    openapi_implicit_flow: 'security scheme "{scheme}": the OAuth 2.0 implicit flow is not supported, imported as authorization code without a token URL',

    har_assets_skipped: {
      one: '{n} page asset entry skipped: images, scripts, stylesheets, fonts and media are not imported',
      other: '{n} page asset entries skipped: images, scripts, stylesheets, fonts and media are not imported',
    },
    har_entries_skipped: {
      one: '{n} {scheme} entry skipped: only HTTP requests are imported',
      other: '{n} {scheme} entries skipped: only HTTP requests are imported',
    },
    har_unreadable_skipped: {
      one: '{n} unreadable URL entry skipped: only HTTP requests are imported',
      other: '{n} unreadable URL entries skipped: only HTTP requests are imported',
    },
    har_cookies_dropped: 'cookie headers were not imported: add the cookies in the cookie manager if the requests need them',
    har_credentials_plain: {
      one: '{n} request keeps credentials from the capture in its headers or query parameters as plain values: move them to secret variables before sharing',
      other: '{n} requests keep credentials from the capture in their headers or query parameters as plain values: move them to secret variables before sharing',
    },
    har_files_dropped: {
      one: '{n} file field was imported without the file; pick it again in the request body',
      other: '{n} file fields were imported without the file; pick it again in the request body',
    },
    har_bodies_left_out: {
      one: '{n} response body was left out of the examples: only JSON, XML and plain-text bodies up to {kb} KB are kept',
      other: '{n} response bodies were left out of the examples: only JSON, XML and plain-text bodies up to {kb} KB are kept',
    },
  },
}

const RU: ImportWarningCopy = {
  subjects: {
    collection: { named: 'коллекция «{name}»', bare: 'коллекция' },
    folder: { named: 'папка «{name}»', bare: 'папка' },
    request: { named: 'запрос «{name}»', bare: 'запрос' },
    environment: { named: 'окружение «{name}»', bare: 'окружение' },
    example: { named: 'пример «{name}»', bare: 'пример' },
    operation: { named: '{name}', bare: '' },
  },
  listMore: '{list} и ещё {n}',
  messages: {
    description_truncated: 'описание длиннее {max} байт обрезано',
    example_skipped: 'пример «{example}» пропущен: {reason}',
    example_too_large: 'пример «{example}» пропущен: тело больше {kb} КБ',
    example_payload_too_large: 'пример «{example}» пропущен: вместе с заголовками он больше {kb} КБ',
    example_secrets: 'в примерах могут быть секреты: {examples}\u00a0— проверьте их, прежде чем делиться коллекцией',
    file_field_dropped: 'поле-файл «{field}» импортировано без файла\u00a0— выберите файл заново',
    file_body_dropped: 'файл из тела запроса (Binary) не импортирован\u00a0— выберите его заново',
    auth_unsupported: 'тип авторизации «{type}» не поддерживается, авторизация не импортирована',
    oauth2_grant_unsupported: 'Grant Type «{grant}» в OAuth 2.0 не поддерживается\u00a0— выберите другой на вкладке авторизации',
    jwt_field_dropped: 'часть JWT «{field}»\u00a0— не JSON-объект, она не импортирована',
    forms_sent_urlencoded: {
      one: '{n} multipart-запрос без полей-файлов будет отправлен как форма URL-encoded, а не multipart: {requests}',
      few: '{n} multipart-запроса без полей-файлов будут отправлены как форма URL-encoded, а не multipart: {requests}',
      many: '{n} multipart-запросов без полей-файлов будут отправлены как форма URL-encoded, а не multipart: {requests}',
      other: '{n} multipart-запроса без полей-файлов будут отправлены как форма URL-encoded, а не multipart: {requests}',
    },
    foreign_scripts: 'скрипты {app} используют API {api}, который Tetiva не выполняет\u00a0— после импорта перепишите их на pm.*',
    variables_unnamed: {
      one: '{n} переменная без имени пропущена',
      few: '{n} переменные без имени пропущены',
      many: '{n} переменных без имени пропущены',
      other: '{n} переменные без имени пропущены',
    },
    unnamed_renamed: '{kind} без имени импортируется как «{name}»',
    unnamed_renamed_in: '{kind} без имени в «{parent}» импортируется как «{name}»',
    name_truncated: 'имя длиннее {max} символов обрезано',

    snapshot_part_missing: 'нет раздела «{part}»\u00a0— запрос пропущен',
    snapshot_protocol_unsupported: 'протокол «{protocol}» не поддерживается\u00a0— запрос пропущен',
    snapshot_method_unsupported: 'метод «{method}» не поддерживается и заменён на GET',
    snapshot_body_unsupported: 'тип тела «{type}» не поддерживается\u00a0— запрос импортирован без тела',
    snapshot_ws_message_format: 'сообщение «{message}» в неизвестном формате «{format}» пропущено',
    snapshot_ws_examples_skipped: {
      one: '{n} пример пропущен: у запросов WebSocket не бывает примеров',
      few: '{n} примера пропущены: у запросов WebSocket не бывает примеров',
      many: '{n} примеров пропущены: у запросов WebSocket не бывает примеров',
      other: '{n} примера пропущены: у запросов WebSocket не бывает примеров',
    },
    snapshot_example_status: 'пример «{example}»: статус {status} вне диапазона 0–999 и заменён на 0',

    postman_scripts: 'скрипты импортированы, но из API pm.* доступна только часть '
      + '(pm.environment, pm.collectionVariables, pm.request, pm.response, pm.test, pm.expect)\u00a0— некоторые, возможно, придётся поправить',
    postman_variables_unreadable: 'не удалось прочитать переменные коллекции, они пропущены',
    postman_item_skipped: 'элемент без имени, без запроса и без вложенных элементов пропущен',
    postman_unnamed_folder: {
      one: 'папка без имени ({n} элемент) импортирована как «{name}»',
      few: 'папка без имени ({n} элемента) импортирована как «{name}»',
      many: 'папка без имени ({n} элементов) импортирована как «{name}»',
      other: 'папка без имени ({n} элемента) импортирована как «{name}»',
    },
    postman_nested_items_skipped: 'вложенные в запрос элементы пропущены',
    postman_description_type: 'описание в формате {type} импортировано как обычный текст',
    postman_descriptions_differ: 'описания элемента и запроса различаются, оставлено описание запроса',
    postman_disabled_script: 'выключенный скрипт {event} не импортирован',
    postman_oauth2_token_only: 'в блоке OAuth 2.0 есть Access Token, но нет Token URL\u00a0— авторизация импортирована как Bearer Token',
    postman_example_skipped: 'пример №{index} пропущен: {reason}',
    postman_example_too_large: 'пример №{index} пропущен: тело больше {kb} КБ',
    postman_example_payload_too_large: 'пример №{index} пропущен: вместе с заголовками он больше {kb} КБ',
    postman_example_wrong_type: 'пример №{index} пропущен: поле «{field}» должно быть типа {want}, а не {got}',
    postman_example_not_object: 'пример №{index} пропущен: это {got}, а не объект',
    postman_example_status: 'пример №{index}: код статуса {status} вне диапазона и заменён на 0',
    postman_example_name_truncated: 'пример №{index}: имя длиннее {max} символов обрезано',
    postman_globals: 'в Tetiva нет глобальных переменных\u00a0— они импортированы как окружение «{name}»',

    insomnia_path_params_differ: {
      single: 'параметр пути {names} в разных запросах отличается\u00a0— в окружении сохранено первое значение',
      several: 'параметры пути {names} в разных запросах отличаются\u00a0— в окружении сохранены первые значения',
    },
    insomnia_workspaces: {
      one: 'в файле {n} рабочее пространство Insomnia, каждое импортировано отдельной коллекцией: {names}',
      few: 'в файле {n} рабочих пространства Insomnia, каждое импортировано отдельной коллекцией: {names}',
      many: 'в файле {n} рабочих пространств Insomnia, каждое импортировано отдельной коллекцией: {names}',
      other: 'в файле {n} рабочих пространства Insomnia, каждое импортировано отдельной коллекцией: {names}',
    },
    insomnia_disabled_params: {
      one: '{n} выключенный параметр запроса не импортирован\u00a0— Tetiva хранит параметры запроса в URL',
      few: '{n} выключенных параметра запроса не импортированы\u00a0— Tetiva хранит параметры запроса в URL',
      many: '{n} выключенных параметров запроса не импортированы\u00a0— Tetiva хранит параметры запроса в URL',
      other: '{n} выключенных параметра запроса не импортированы\u00a0— Tetiva хранит параметры запроса в URL',
    },
    insomnia_disabled_metadata: {
      one: '{n} выключенный элемент метаданных gRPC не импортирован',
      few: '{n} выключенных элемента метаданных gRPC не импортированы',
      many: '{n} выключенных элементов метаданных gRPC не импортированы',
      other: '{n} выключенных элемента метаданных gRPC не импортированы',
    },
    insomnia_grpc_no_proto: 'запросы gRPC импортированы без proto-файлов\u00a0— выберите файл .proto или используйте server reflection',
    insomnia_vault_secrets: {
      one: '{n} секрет из Vault импортирован как пустая секретная переменная\u00a0— впишите значение в окружении',
      few: '{n} секрета из Vault импортированы как пустые секретные переменные\u00a0— впишите значения в окружении',
      many: '{n} секретов из Vault импортированы как пустые секретные переменные\u00a0— впишите значения в окружении',
      other: '{n} секрета из Vault импортированы как пустые секретные переменные\u00a0— впишите значения в окружении',
    },
    insomnia_template_tags: {
      one: '{n} шаблонный тег Insomnia оставлен как есть ({names}): Tetiva такие теги не выполняет, используйте переменные или скрипты',
      few: '{n} шаблонных тега Insomnia оставлены как есть ({names}): Tetiva такие теги не выполняет, используйте переменные или скрипты',
      many: '{n} шаблонных тегов Insomnia оставлены как есть ({names}): Tetiva такие теги не выполняет, используйте переменные или скрипты',
      other: '{n} шаблонных тега Insomnia оставлены как есть ({names}): Tetiva такие теги не выполняет, используйте переменные или скрипты',
    },
    insomnia_socketio_skipped: {
      one: '{n} запрос Socket.IO пропущен: Tetiva не поддерживает Socket.IO',
      few: '{n} запроса Socket.IO пропущены: Tetiva не поддерживает Socket.IO',
      many: '{n} запросов Socket.IO пропущены: Tetiva не поддерживает Socket.IO',
      other: '{n} запроса Socket.IO пропущены: Tetiva не поддерживает Socket.IO',
    },
    insomnia_unit_tests_skipped: {
      one: '{n} юнит-тест не импортирован',
      few: '{n} юнит-теста не импортированы',
      many: '{n} юнит-тестов не импортированы',
      other: '{n} юнит-теста не импортированы',
    },
    insomnia_cookies_skipped: {
      one: 'cookie из Cookie Jar не импортированы: {n}',
      few: 'cookie из Cookie Jar не импортированы: {n}',
      many: 'cookie из Cookie Jar не импортированы: {n}',
      other: 'cookie из Cookie Jar не импортированы: {n}',
    },
    insomnia_specs_skipped: {
      one: '{n} документ спецификации API не импортирован',
      few: '{n} документа спецификации API не импортированы',
      many: '{n} документов спецификации API не импортированы',
      other: '{n} документа спецификации API не импортированы',
    },
    insomnia_folder_headers: 'заголовки папки не импортированы',
    insomnia_folder_environment: 'переменные окружения папки не импортированы',
    insomnia_method_unsupported: 'метод {method} не поддерживается и заменён на GET',
    insomnia_graphql_not_json: 'тело GraphQL не в формате JSON и импортировано как текст query',
    insomnia_auth_disabled: 'авторизация {type} выключена в Insomnia и не импортирована',
    insomnia_apikey_location: 'API-ключ, который передаётся как {location}, не поддерживается\u00a0— авторизация не импортирована',

    hoppscotch_variable_conflicts: 'у переменных, заданных несколько раз, сохранено первое значение: {keys}',
    hoppscotch_secrets_empty: 'Hoppscotch выгружает секреты без значений\u00a0— впишите значения в окружении: {keys}',
    hoppscotch_headers_copied: 'заголовки коллекций и папок Hoppscotch скопированы в их запросы',
    hoppscotch_auth_off: {
      single: 'запросы в папке {folders} унаследуют авторизацию родителя: в Tetiva у папки нельзя выключить авторизацию'
        + '\u00a0— если запросы должны идти без неё, выберите у них тип None',
      several: 'запросы в папках {folders} унаследуют авторизацию родителя: в Tetiva у папки нельзя выключить авторизацию'
        + '\u00a0— если запросы должны идти без неё, выберите у них тип None',
    },

    openapi_external_ref: 'ссылки на «{target}» не поддерживаются и оставлены пустыми',
    openapi_broken_ref: 'ссылка «{ref}» никуда не ведёт и оставлена пустой',
    openapi_auth_header_param: 'параметр-заголовок Authorization у {operations} не импортирован\u00a0— задайте авторизацию в коллекции или в запросе',
    openapi_webhooks_skipped: 'вебхуки пропущены: {names}',
    openapi_no_server: 'в документе не указан сервер\u00a0— задайте baseUrl в окружении',
    openapi_relative_server: 'адрес сервера «{url}» относительный\u00a0— задайте в окружении полный адрес в baseUrl',
    openapi_own_server: 'собственный сервер «{url}» не учтён, запрос использует {{baseUrl}}',
    openapi_trace_skipped: 'запрос TRACE {path} пропущен\u00a0— запросы TRACE не поддерживаются',
    openapi_callbacks_skipped: 'колбэки пропущены',
    openapi_query_style: 'стиль {style} у параметра запроса «{param}» не поддерживается и не применён',
    openapi_links_skipped: 'ссылки ответов (links) пропущены',
    openapi_schemes_together: 'схемы безопасности {schemes} требуются вместе\u00a0— импортирована только «{scheme}»',
    openapi_scheme_undefined: 'схема безопасности «{scheme}» не определена\u00a0— запросы с ней импортированы без авторизации',
    openapi_scheme_unsupported: 'схема безопасности «{scheme}» ({type}) не поддерживается\u00a0— запросы с ней импортированы без авторизации',
    openapi_scheme_location_unsupported: 'схема безопасности «{scheme}» ({type} в {in}) не поддерживается\u00a0— запросы с ней импортированы без авторизации',
    openapi_implicit_flow: 'схема безопасности «{scheme}»: implicit flow OAuth 2.0 не поддерживается, '
      + 'схема импортирована как Authorization Code без Token URL',

    har_assets_skipped: {
      one: '{n} ресурс страницы пропущен: изображения, скрипты, стили, шрифты и медиа не импортируются',
      few: '{n} ресурса страницы пропущены: изображения, скрипты, стили, шрифты и медиа не импортируются',
      many: '{n} ресурсов страницы пропущены: изображения, скрипты, стили, шрифты и медиа не импортируются',
      other: '{n} ресурса страницы пропущены: изображения, скрипты, стили, шрифты и медиа не импортируются',
    },
    har_entries_skipped: {
      one: '{n} запись {scheme} пропущена: импортируются только HTTP-запросы',
      few: '{n} записи {scheme} пропущены: импортируются только HTTP-запросы',
      many: '{n} записей {scheme} пропущены: импортируются только HTTP-запросы',
      other: '{n} записи {scheme} пропущены: импортируются только HTTP-запросы',
    },
    har_unreadable_skipped: {
      one: '{n} запись с нечитаемым URL пропущена: импортируются только HTTP-запросы',
      few: '{n} записи с нечитаемым URL пропущены: импортируются только HTTP-запросы',
      many: '{n} записей с нечитаемым URL пропущены: импортируются только HTTP-запросы',
      other: '{n} записи с нечитаемым URL пропущены: импортируются только HTTP-запросы',
    },
    har_cookies_dropped: 'заголовки Cookie не импортированы\u00a0— если запросам нужны cookie, добавьте их в Cookie Manager',
    har_credentials_plain: {
      one: '{n} запрос хранит учётные данные из HAR-файла в заголовках или параметрах запроса открытым текстом'
        + '\u00a0— перенесите их в секретные переменные, прежде чем делиться коллекцией',
      few: '{n} запроса хранят учётные данные из HAR-файла в заголовках или параметрах запроса открытым текстом'
        + '\u00a0— перенесите их в секретные переменные, прежде чем делиться коллекцией',
      many: '{n} запросов хранят учётные данные из HAR-файла в заголовках или параметрах запроса открытым текстом'
        + '\u00a0— перенесите их в секретные переменные, прежде чем делиться коллекцией',
      other: '{n} запроса хранят учётные данные из HAR-файла в заголовках или параметрах запроса открытым текстом'
        + '\u00a0— перенесите их в секретные переменные, прежде чем делиться коллекцией',
    },
    har_files_dropped: {
      one: '{n} поле-файл импортировано без файла\u00a0— выберите файл заново в теле запроса',
      few: '{n} поля-файла импортированы без файлов\u00a0— выберите файлы заново в теле запроса',
      many: '{n} полей-файлов импортированы без файлов\u00a0— выберите файлы заново в теле запроса',
      other: '{n} поля-файла импортированы без файлов\u00a0— выберите файлы заново в теле запроса',
    },
    har_bodies_left_out: {
      one: '{n} тело ответа не вошло в примеры: сохраняются только тела JSON, XML и обычного текста до {kb} КБ',
      few: '{n} тела ответов не вошли в примеры: сохраняются только тела JSON, XML и обычного текста до {kb} КБ',
      many: '{n} тел ответов не вошли в примеры: сохраняются только тела JSON, XML и обычного текста до {kb} КБ',
      other: '{n} тела ответов не вошли в примеры: сохраняются только тела JSON, XML и обычного текста до {kb} КБ',
    },
  },
}

export const IMPORT_WARNINGS: Record<Locale, ImportWarningCopy> = { en: EN, ru: RU }

function has<T extends object>(obj: T, key: string): key is Extract<keyof T, string> {
  return Object.prototype.hasOwnProperty.call(obj, key)
}

const GROUPED = ['n', 'max', 'kb']
const NAME_LISTS = ['requests', 'folders', 'names', 'examples']

function template(message: WarningMessage, n: string | undefined, locale: Locale): string {
  if (typeof message === 'string') return message
  if ('single' in message) return n === '1' ? message.single : message.several
  return plural(locale, Number(n), message)
}

// Mirrors portability.Warn in Go; warnings.test.ts holds both to the same samples.
export function importWarningText(w: ImportWarning, locale: Locale = currentLocale.value): string {
  const copy = IMPORT_WARNINGS[locale]
  if (!has(copy.messages, w.code)) return w.text
  const params = w.params
  const values: Record<string, string> = { ...params }
  for (const [key, value] of Object.entries(params)) {
    const more = params[`${key}More`]
    if (Number(more) > 0) values[key] = fill(copy.listMore, { list: value, n: more })
  }
  if (locale === 'ru') {
    for (const key of NAME_LISTS) {
      if (values[key]) values[key] = values[key].replace(/"([^"]*)"/g, '«$1»')
    }
  }
  for (const key of GROUPED) {
    if (/^\d+$/.test(params[key] ?? '')) values[key] = formatNumber(locale, Number(params[key]))
  }
  if (params.kind !== undefined) {
    if (!has(copy.subjects, params.kind)) return w.text
    values.kind = copy.subjects[params.kind].bare
  }
  const text = fill(template(copy.messages[w.code], params.n, locale), values)
  const kind = w.subject.kind
  if (kind === '') return text
  if (!has(copy.subjects, kind)) return w.text
  const subject = copy.subjects[kind]
  return `${w.subject.name ? fill(subject.named, { name: w.subject.name }) : subject.bare}: ${text}`
}
