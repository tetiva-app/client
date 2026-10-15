import type { Locale, PluralForms } from '@/lib/locale'
import type { HistoryGroupBucket } from '@/constants/history'

export interface HistoryCopy {
  title: string
  clearHistory: string
  search: string
  filters: string
  filter: {
    protocol: string
    status: string
    error: string
    clear: string
    escToClose: string
  }
  filteredByRequest: string
  clearFilter: string
  buckets: Record<HistoryGroupBucket, string>
  replayAsDraft: string
  delete: string
  duration: string
  empty: {
    title: string
    text: string
    noMatches: string
    noMatchesText: string
    clearFilters: string
  }
  shown: string
  loadMore: string
  clearAll: {
    title: string
    text: PluralForms
    confirm: string
  }
  deleteRecord: {
    title: string
    text: string
    confirm: string
  }
  cancel: string
  viewer: {
    select: string
    replay: string
    request: string
    response: string
    error: string
    headers: string
    noHeaders: string
    body: string
    emptyBody: string
  }
  toasts: {
    replayed: string
    replayFailed: string
  }
}

export const HISTORY_COPY: Record<Locale, HistoryCopy> = {
  en: {
    title: 'History',
    clearHistory: 'Clear all history',
    search: 'Search URL...',
    filters: 'Filters',
    filter: {
      protocol: 'Protocol',
      status: 'Status',
      error: 'error',
      clear: 'Clear filters',
      escToClose: 'Esc to close',
    },
    filteredByRequest: 'Filtered by request',
    clearFilter: 'Clear filter',
    buckets: {
      Today: 'Today',
      Yesterday: 'Yesterday',
      'This week': 'This week',
      Earlier: 'Earlier',
    },
    replayAsDraft: 'Replay (open as draft)',
    delete: 'Delete',
    duration: '{n}ms',
    empty: {
      title: 'No request history yet',
      text: 'Execute a request to see it here.',
      noMatches: 'No matches',
      noMatchesText: 'Try clearing or relaxing your filters.',
      clearFilters: 'Clear filters',
    },
    shown: '{shown} of {total} records',
    loadMore: 'Load more',
    clearAll: {
      title: 'Clear all history?',
      text: {
        one: 'This deletes {n} record in workspace {workspace}. This cannot be undone.',
        other: 'This deletes {n} records in workspace {workspace}. This cannot be undone.',
      },
      confirm: 'Clear all',
    },
    deleteRecord: {
      title: 'Delete history record?',
      text: 'This cannot be undone.',
      confirm: 'Delete',
    },
    cancel: 'Cancel',
    viewer: {
      select: 'Select a history record to view details.',
      replay: 'Replay',
      request: 'Request',
      response: 'Response',
      error: 'Error',
      headers: 'Headers',
      noHeaders: '(no headers)',
      body: 'Body',
      emptyBody: '(empty)',
    },
    toasts: {
      replayed: 'Opened as draft — edit and Send, or save to a collection.',
      replayFailed: 'Replay failed',
    },
  },
  ru: {
    title: 'История',
    clearHistory: 'Очистить всю историю',
    search: 'Поиск по URL…',
    filters: 'Фильтры',
    filter: {
      protocol: 'Протокол',
      status: 'Статус',
      error: 'ошибка',
      clear: 'Сбросить фильтры',
      escToClose: 'Esc\u00a0— закрыть',
    },
    filteredByRequest: 'Фильтр по запросу',
    clearFilter: 'Сбросить фильтр',
    buckets: {
      Today: 'Сегодня',
      Yesterday: 'Вчера',
      'This week': 'На этой неделе',
      Earlier: 'Раньше',
    },
    replayAsDraft: 'Повторить (открыть как черновик)',
    delete: 'Удалить',
    duration: '{n}\u00a0мс',
    empty: {
      title: 'Истории пока нет',
      text: 'Отправленные запросы появятся здесь.',
      noMatches: 'Ничего не найдено',
      noMatchesText: 'Попробуйте изменить или сбросить фильтры.',
      clearFilters: 'Сбросить фильтры',
    },
    shown: 'Показано {shown} из {total}',
    loadMore: 'Загрузить ещё',
    clearAll: {
      title: 'Очистить всю историю?',
      text: {
        one: 'В пространстве «{workspace}» будет удалена {n} запись. Это действие нельзя отменить.',
        few: 'В пространстве «{workspace}» будут удалены {n} записи. Это действие нельзя отменить.',
        many: 'В пространстве «{workspace}» будет удалено {n} записей. Это действие нельзя отменить.',
        other: 'В пространстве «{workspace}» будут удалены {n} записи. Это действие нельзя отменить.',
      },
      confirm: 'Очистить',
    },
    deleteRecord: {
      title: 'Удалить запись из истории?',
      text: 'Это действие нельзя отменить.',
      confirm: 'Удалить',
    },
    cancel: 'Отмена',
    viewer: {
      select: 'Выберите запись в истории, чтобы посмотреть подробности.',
      replay: 'Повторить',
      request: 'Запрос',
      response: 'Ответ',
      error: 'Ошибка',
      headers: 'Заголовки',
      noHeaders: '(нет заголовков)',
      body: 'Тело',
      emptyBody: '(пусто)',
    },
    toasts: {
      replayed: 'Открыт черновик\u00a0— отправьте его или сохраните в коллекцию.',
      replayFailed: 'Не удалось повторить запрос',
    },
  },
}
