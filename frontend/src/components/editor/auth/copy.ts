import type { Locale, PluralForms } from '@/lib/locale'

export interface AuthCopy {
  typeLabel: string
  inheritOption: string
  noAuthOption: string
  httpOnly: string
  collection: {
    inherits: string
    noParent: string
    noneTitle: string
    noneText: string
  }
  request: {
    inherits: string
    noParent: string
    none: string
  }
  inherited: {
    configuredOn: string
    onCollection: string
    onParent: string
    manageToken: string
    resolveFailed: string
    emptyKey: string
    emptyToken: string
  }
  tokenStatus: {
    valid: string
    validUntil: string
    validUntilDay: string
    refreshable: string
    expired: string
    none: string
  }
  fields: {
    username: string
    password: string
    prefix: string
    token: string
    tokenPlaceholder: string
    tokenMissing: string
    key: string
    keyMissing: string
    value: string
    valuePlaceholder: string
    addTo: string
    addTokenTo: string
    inHeader: string
    inQuery: string
    queryParam: string
    headerPrefix: string
    show: string
    hide: string
    unsupported: string
    unsupportedHint: string
    invalidJson: string
  }
  digest: {
    algorithmNote: string
  }
  aws: {
    sessionTokenHint: string
    region: string
    service: string
  }
  jwt: {
    algorithm: string
    secret: string
    secretEncoding: string
    plainText: string
    base64: string
    privateKey: string
    claims: string
    claimsHint: string
    expiresIn: string
    expiresHint: string
    joseHeader: string
    joseHint: string
  }
  oauth2: {
    grant: string
    tokenUrl: string
    authUrl: string
    deviceAuthUrl: string
    clientAuth: string
    clientAuthBasic: string
    clientAuthBody: string
    clientAuthNone: string
    redirectPort: string
    redirectHint: string
    getToken: string
    clear: string
    waitingBrowser: string
    enterCode: string
    copyCode: string
    open: string
    openAgain: string
    copyLink: string
    cancel: string
    fieldsNeedAttention: PluralForms
    configChanged: string
  }
}

export const AUTH_COPY: Record<Locale, AuthCopy> = {
  en: {
    typeLabel: 'Authorization Type',
    inheritOption: 'Inherit',
    noAuthOption: 'No Auth',
    httpOnly: '{label} is supported for HTTP requests only; this request will fail to send.',
    collection: {
      inherits: 'Requests in this collection inherit auth from parent collections',
      noParent: 'No parent collection sets auth — requests here go without it unless they set their own',
      noneTitle: 'No authorization',
      noneText: 'Requests here go without auth unless they set their own',
    },
    request: {
      inherits: 'This request inherits authentication from its parent collection',
      noParent: 'No parent collection sets auth — this request goes without it',
      none: 'No authentication configured for this request',
    },
    inherited: {
      configuredOn: '{label} configured on',
      onCollection: 'collection “{name}”',
      onParent: 'the parent collection',
      manageToken: "{status} — manage the token on that collection's Auth tab",
      resolveFailed: "Couldn't resolve inherited auth",
      emptyKey: 'Its key name is empty, so nothing is added to this request.',
      emptyToken: 'Its token is empty, so no Authorization header is sent.',
    },
    tokenStatus: {
      valid: 'Valid',
      validUntil: 'Valid until {time}',
      validUntilDay: 'Valid until {day}, {time}',
      refreshable: 'Expired, will refresh',
      expired: 'Expired',
      none: 'No token',
    },
    fields: {
      username: 'Username',
      password: 'Password',
      prefix: 'Prefix',
      token: 'Token',
      tokenPlaceholder: 'Paste your token here',
      tokenMissing: 'Without a token no Authorization header is sent',
      key: 'Key',
      keyMissing: 'Without a key name nothing is added to the request',
      value: 'Value',
      valuePlaceholder: 'Your API key value',
      addTo: 'Add to',
      addTokenTo: 'Add token to',
      inHeader: 'Header',
      inQuery: 'Query Params',
      queryParam: 'Query Parameter',
      headerPrefix: 'Header Prefix',
      show: 'Show {label}',
      hide: 'Hide {label}',
      unsupported: '{value} (unsupported)',
      unsupportedHint: 'Stored as “{value}”, which this client cannot send — pick a supported value.',
      invalidJson: 'Not a JSON object — the last valid value is used.',
    },
    digest: {
      algorithmNote: 'The algorithm follows the server challenge: MD5, SHA-256 and SHA-512-256 are supported.',
    },
    aws: {
      sessionTokenHint: 'Only for temporary STS credentials.',
      region: 'Region',
      service: 'Service',
    },
    jwt: {
      algorithm: 'Algorithm',
      secret: 'Secret',
      secretEncoding: 'Secret Encoding',
      plainText: 'Plain text',
      base64: 'Base64 encoded',
      privateKey: 'Private Key',
      claims: 'Payload (claims)',
      claimsHint: 'iat and exp are added at send time unless the claims set them.',
      expiresIn: 'Expires In (seconds)',
      expiresHint: '0 signs a token without exp.',
      joseHeader: 'JOSE Header',
      joseHint: 'kid, typ, cty, x5t, x5u and jku only — alg comes from the algorithm above.',
    },
    oauth2: {
      grant: 'Grant Type',
      tokenUrl: 'Token URL',
      authUrl: 'Authorization URL',
      deviceAuthUrl: 'Device Authorization URL',
      clientAuth: 'Client Authentication',
      clientAuthBasic: 'Basic header',
      clientAuthBody: 'Request body',
      clientAuthNone: 'None (public client)',
      redirectPort: 'Redirect Port',
      redirectHint: 'Loopback port for the callback; 0 picks a free one.',
      getToken: 'Get token',
      clear: 'Clear',
      waitingBrowser: 'Waiting for the browser — approve the request there, then come back.',
      enterCode: 'Enter this code on the verification page:',
      copyCode: 'Copy code',
      open: 'Open',
      openAgain: 'Open again',
      copyLink: 'Copy link',
      cancel: 'Cancel',
      fieldsNeedAttention: {
        one: 'one field below needs attention',
        other: '{n} fields below need attention',
      },
      configChanged: 'Token request cancelled: the configuration changed. Press Get token to start again.',
    },
  },
  ru: {
    typeLabel: 'Тип авторизации',
    inheritOption: 'Наследовать',
    noAuthOption: 'Без авторизации',
    httpOnly: '{label} работает только в HTTP-запросах — этот запрос не отправится.',
    collection: {
      inherits: 'Запросы этой коллекции наследуют авторизацию родительских коллекций',
      noParent: 'В родительских коллекциях авторизация не задана — запросы уходят без неё, если у них нет своей',
      noneTitle: 'Авторизации нет',
      noneText: 'Запросы уходят без авторизации, если у них нет своей',
    },
    request: {
      inherits: 'Запрос наследует авторизацию родительской коллекции',
      noParent: 'В родительских коллекциях авторизация не задана — запрос уйдёт без неё',
      none: 'У этого запроса нет авторизации',
    },
    inherited: {
      configuredOn: 'Тип {label} задан в',
      onCollection: 'коллекции «{name}»',
      onParent: 'родительской коллекции',
      manageToken: '{status} — управлять токеном можно на вкладке «Авторизация» той коллекции, где она задана',
      resolveFailed: 'Не удалось узнать, какая авторизация наследуется',
      emptyKey: 'Имя ключа не задано, поэтому к запросам ничего не добавляется.',
      emptyToken: 'Токен пустой, поэтому заголовок Authorization не отправляется.',
    },
    tokenStatus: {
      valid: 'Действует',
      validUntil: 'Действует до {time}',
      validUntilDay: 'Действует до {day}, {time}',
      refreshable: 'Истёк, будет обновлён',
      expired: 'Истёк',
      none: 'Токена нет',
    },
    fields: {
      username: 'Имя пользователя',
      password: 'Пароль',
      prefix: 'Префикс',
      token: 'Токен',
      tokenPlaceholder: 'Вставьте токен',
      tokenMissing: 'Без токена заголовок Authorization не отправляется',
      key: 'Ключ',
      keyMissing: 'Без имени ключа к запросам ничего не добавляется',
      value: 'Значение',
      valuePlaceholder: 'Значение API-ключа',
      addTo: 'Где передавать',
      addTokenTo: 'Где передавать токен',
      inHeader: 'В заголовке',
      inQuery: 'В query-параметрах',
      queryParam: 'Имя query-параметра',
      headerPrefix: 'Префикс в заголовке',
      show: 'Показать: {label}',
      hide: 'Скрыть: {label}',
      unsupported: '{value} (не поддерживается)',
      unsupportedHint: 'Tetiva не умеет отправлять «{value}» — выберите другое значение.',
      invalidJson: 'Это не JSON-объект — используется последнее корректное значение.',
    },
    digest: {
      algorithmNote: 'Алгоритм задаёт сервер в ответе 401; поддерживаются MD5, SHA-256 и SHA-512-256.',
    },
    aws: {
      sessionTokenHint: 'Нужен только для временных учётных данных STS.',
      region: 'Регион',
      service: 'Сервис',
    },
    jwt: {
      algorithm: 'Алгоритм',
      secret: 'Секрет',
      secretEncoding: 'Кодировка секрета',
      plainText: 'Обычный текст',
      base64: 'Base64',
      privateKey: 'Закрытый ключ',
      claims: 'Полезная нагрузка (claims)',
      claimsHint: 'iat и exp добавляются при отправке, если claims их не задают.',
      expiresIn: 'Срок действия в секундах',
      expiresHint: 'При 0 токен подписывается без exp.',
      joseHeader: 'Заголовок JOSE',
      joseHint: 'Только kid, typ, cty, x5t, x5u и jku; alg берётся из алгоритма выше.',
    },
    oauth2: {
      grant: 'Способ получения токена',
      tokenUrl: 'URL получения токена',
      authUrl: 'URL авторизации',
      deviceAuthUrl: 'URL авторизации устройства',
      clientAuth: 'Аутентификация клиента',
      clientAuthBasic: 'Заголовок Basic',
      clientAuthBody: 'Тело запроса',
      clientAuthNone: 'Нет (публичный клиент)',
      redirectPort: 'Порт для редиректа',
      redirectHint: 'Локальный порт для callback; 0 — любой свободный.',
      getToken: 'Получить токен',
      clear: 'Сбросить',
      waitingBrowser: 'Ждём ответа в браузере: подтвердите доступ там и возвращайтесь.',
      enterCode: 'Введите этот код на странице подтверждения:',
      copyCode: 'Скопировать код',
      open: 'Открыть',
      openAgain: 'Открыть снова',
      copyLink: 'Скопировать ссылку',
      cancel: 'Отменить',
      fieldsNeedAttention: {
        one: 'проверьте поле ниже',
        few: 'проверьте {n} поля ниже',
        many: 'проверьте {n} полей ниже',
        other: 'проверьте {n} поля ниже',
      },
      configChanged: 'Запрос токена отменён: настройки изменились. Нажмите «Получить токен», чтобы начать заново.',
    },
  },
}
