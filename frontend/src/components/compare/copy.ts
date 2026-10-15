import type { Locale, PluralForms } from '@/lib/locale'

export interface CompareCopy {
  compare: string
  compareWithPrevious: string
  chooseBaseline: string
  popover: {
    title: string
    earlierSends: string
    savedExamples: string
    noExamples: string
    requestDiffered: string
    exampleTag: string
    allSends: PluralForms
    first: string
    firstHint: string
    loadFailed: string
    retry: string
  }
  side: {
    fromHistory: string
    example: string
    current: string
    now: string
  }
  bar: {
    swap: string
    changeOf: string
    noChanges: string
    prev: string
    next: string
    normalize: string
    normalizeHint: string
    split: string
    unified: string
    close: string
  }
  tabs: {
    body: string
    headers: string
    metadata: string
    request: string
    same: string
    sameHint: string
  }
  masked: string
  collapsed: {
    text: string
    show: string
  }
  state: {
    identical: string
    identicalNormalized: string
    showRaw: string
    binary: string
    tooLarge: string
    bothEmpty: string
    noResponse: string
    headersIdentical: string
    headersIdenticalNormalized: string
  }
  request: {
    field: string
    method: string
    url: string
    body: string
    variables: string
    none: string
    header: string
    query: string
    notCompared: string
    variablesNotKept: string
    exampleNoRequest: string
    currentMissing: string
  }
  pair: {
    title: string
    responseBody: string
    responseHeaders: string
    differ: PluralForms
    match: string
    summary: string
    bodyChanges: PluralForms
    bodySame: string
    bodyNotCompared: string
    headerChanges: PluralForms
    headersSame: string
    openBody: string
    authNote: string
  }
  latency: {
    slower: string
    faster: string
    slowerThanA: string
    fasterThanA: string
  }
  cm: {
    unchanged: string
  }
}

export const COMPARE_COPY: Record<Locale, CompareCopy> = {
  en: {
    compare: 'Compare',
    compareWithPrevious: 'Compare with the previous send',
    chooseBaseline: 'Compare with another response',
    popover: {
      title: 'Compare current response with…',
      earlierSends: 'Earlier sends of “{name}”',
      savedExamples: 'Saved examples',
      noExamples: 'None for this request.',
      requestDiffered: 'request differed',
      exampleTag: 'example',
      allSends: {
        one: 'All {n} sends in History, compare any two',
        other: 'All {n} sends in History, compare any two',
      },
      first: 'This is the first response of “{name}”.',
      firstHint: 'Every send is saved to History. Send once more, and the earlier response will appear here.',
      loadFailed: "Couldn't load History",
      retry: 'Try again',
    },
    side: {
      fromHistory: '{time} · from History',
      example: '{name} · example',
      current: 'Current response · {time}',
      now: 'Now',
    },
    bar: {
      swap: 'Swap sides',
      changeOf: 'Change {n} of {total}',
      noChanges: 'No changes',
      prev: 'Previous change',
      next: 'Next change',
      normalize: 'Normalize JSON',
      normalizeHint: 'Sort keys and mask volatile values before comparing',
      split: 'Split',
      unified: 'Unified',
      close: 'Close compare',
    },
    tabs: {
      body: 'Body',
      headers: 'Headers',
      metadata: 'Metadata',
      request: 'Request',
      same: 'same',
      sameHint: 'The compared fields match. Secret headers and parameters are not compared.',
    },
    masked: 'Masked: {list}',
    collapsed: {
      text: 'Request editor is collapsed while comparing',
      show: 'Show',
    },
    state: {
      identical: 'Bodies are identical',
      identicalNormalized: 'No differences once keys are sorted and volatile values are masked',
      showRaw: 'Compare without normalization',
      binary: 'Binary body: not compared ({a} → {b})',
      tooLarge: 'Bodies over 1 MB are not compared here ({a}, {b}). Headers and the request still are.',
      bothEmpty: 'Both bodies are empty',
      noResponse: 'No response: {error}',
      headersIdentical: 'Headers are identical',
      headersIdenticalNormalized: 'No differences apart from Date and X-Request-Id',
    },
    request: {
      field: 'Field',
      method: 'Method',
      url: 'URL',
      body: 'Body',
      variables: 'Variables',
      none: 'none',
      header: 'Header · {name}',
      query: 'Query · {name}',
      notCompared: 'secret, not compared',
      variablesNotKept: "Variables aren't kept in History",
      exampleNoRequest: "Saved examples don't keep the request",
      currentMissing: "The request of the current response isn't in History",
    },
    pair: {
      title: 'Compare · {name}',
      responseBody: 'Response body',
      responseHeaders: 'Response headers',
      differ: {
        one: 'The requests differ in {n} field, so differences in the response are expected.',
        other: 'The requests differ in {n} fields, so differences in the response are expected.',
      },
      match: 'The compared request fields match.',
      summary: 'Response: {a} {rel} {b} · {body} · {headers} · {msA} vs {msB}',
      bodyChanges: {
        one: 'body: {n} change',
        other: 'body: {n} changes',
      },
      bodySame: 'body: no changes',
      bodyNotCompared: 'body not compared',
      headerChanges: {
        one: 'headers: {n} change',
        other: 'headers: {n} changes',
      },
      headersSame: 'headers: no changes',
      openBody: 'Open response body',
      authNote: 'Secret headers and parameters are hidden and not compared.',
    },
    latency: {
      slower: '{n} ms slower than the previous send',
      faster: '{n} ms faster than the previous send',
      slowerThanA: '{n} ms slower than A',
      fasterThanA: '{n} ms faster than A',
    },
    cm: {
      unchanged: '$ unchanged lines',
    },
  },
  ru: {
    compare: 'Сравнить',
    compareWithPrevious: 'Сравнить с предыдущей отправкой',
    chooseBaseline: 'Сравнить с другим ответом',
    popover: {
      title: 'Сравнить текущий ответ с…',
      earlierSends: 'Прошлые отправки запроса «{name}»',
      savedExamples: 'Сохранённые примеры',
      noExamples: 'У этого запроса их нет.',
      requestDiffered: 'запрос был другим',
      exampleTag: 'пример',
      allSends: {
        one: '{n} отправка в истории: сравните любые две',
        few: '{n} отправки в истории: сравните любые две',
        many: '{n} отправок в истории: сравните любые две',
        other: '{n} отправки в истории: сравните любые две',
      },
      first: 'Это первый ответ запроса «{name}».',
      firstHint: 'Каждая отправка сохраняется в истории. Отправьте запрос ещё раз, и прошлый ответ появится здесь.',
      loadFailed: 'Не удалось загрузить историю',
      retry: 'Повторить',
    },
    side: {
      fromHistory: '{time} · из истории',
      example: '{name} · пример',
      current: 'Текущий ответ · {time}',
      now: 'Сейчас',
    },
    bar: {
      swap: 'Поменять местами',
      changeOf: 'Изменение {n} из {total}',
      noChanges: 'Изменений нет',
      prev: 'Предыдущее изменение',
      next: 'Следующее изменение',
      normalize: 'Нормализовать JSON',
      normalizeHint: 'Перед сравнением отсортировать ключи и не учитывать изменчивые значения',
      split: 'Две колонки',
      unified: 'Одна колонка',
      close: 'Закрыть сравнение',
    },
    tabs: {
      body: 'Тело',
      headers: 'Заголовки',
      metadata: 'Метаданные',
      request: 'Запрос',
      same: 'совпадает',
      sameHint: 'Сравниваемые поля совпадают. Секретные заголовки и параметры не сравниваются.',
    },
    masked: 'Не учитываются: {list}',
    collapsed: {
      text: 'Редактор запроса свёрнут на время сравнения',
      show: 'Показать',
    },
    state: {
      identical: 'Тела совпадают',
      identicalNormalized: 'Различий нет без учёта порядка ключей и изменчивых значений',
      showRaw: 'Сравнить без нормализации',
      binary: 'Двоичное тело не сравнивается ({a} → {b})',
      tooLarge: 'Тела больше 1 МБ здесь не сравниваются ({a}, {b}). Заголовки и запрос сравниваются.',
      bothEmpty: 'Оба тела пустые',
      noResponse: 'Ответа нет: {error}',
      headersIdentical: 'Заголовки совпадают',
      headersIdenticalNormalized: 'Различий нет без учёта Date и X-Request-Id',
    },
    request: {
      field: 'Поле',
      method: 'Метод',
      url: 'URL',
      body: 'Тело',
      variables: 'Переменные',
      none: 'нет',
      header: 'Заголовок · {name}',
      query: 'Параметр · {name}',
      notCompared: 'секрет, не сравнивается',
      variablesNotKept: 'Переменные в истории не сохраняются',
      exampleNoRequest: 'Примеры не хранят запрос',
      currentMissing: 'Запроса текущего ответа нет в истории',
    },
    pair: {
      title: 'Сравнение · {name}',
      responseBody: 'Тело ответа',
      responseHeaders: 'Заголовки ответа',
      differ: {
        one: 'Запросы различаются в {n} поле, поэтому разница в ответах ожидаема.',
        few: 'Запросы различаются в {n} полях, поэтому разница в ответах ожидаема.',
        many: 'Запросы различаются в {n} полях, поэтому разница в ответах ожидаема.',
        other: 'Запросы различаются в {n} полях, поэтому разница в ответах ожидаема.',
      },
      match: 'Сравниваемые поля запроса совпадают.',
      summary: 'Ответ: {a} {rel} {b} · {body} · {headers} · {msA} и {msB}',
      bodyChanges: {
        one: 'тело: {n} изменение',
        few: 'тело: {n} изменения',
        many: 'тело: {n} изменений',
        other: 'тело: {n} изменения',
      },
      bodySame: 'тело: без изменений',
      bodyNotCompared: 'тело не сравнивалось',
      headerChanges: {
        one: 'заголовки: {n} изменение',
        few: 'заголовки: {n} изменения',
        many: 'заголовки: {n} изменений',
        other: 'заголовки: {n} изменения',
      },
      headersSame: 'заголовки: без изменений',
      openBody: 'Открыть тело ответа',
      authNote: 'Секретные заголовки и параметры скрыты и не сравниваются.',
    },
    latency: {
      slower: 'На {n}\u00a0мс медленнее прошлой отправки',
      faster: 'На {n}\u00a0мс быстрее прошлой отправки',
      slowerThanA: 'На {n}\u00a0мс медленнее, чем A',
      fasterThanA: 'На {n}\u00a0мс быстрее, чем A',
    },
    cm: {
      unchanged: 'Строк без изменений: $',
    },
  },
}
