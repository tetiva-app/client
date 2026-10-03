import type { Locale } from '@/lib/locale'

export interface RestartCopy {
  title: string
  restart: string
  cancel: string
  restarting: string
  requestRunning: string
  websockets: string
  unsaved: string
  exampleUnsaved: string
  windows: string
}

export const RESTART_COPY: Record<Locale, RestartCopy> = {
  en: {
    title: 'Restart Tetiva now?',
    restart: 'Restart',
    cancel: 'Cancel',
    restarting: 'Restarting…',
    requestRunning: 'A request is running',
    websockets: 'WebSocket connections will close: {n}',
    unsaved: 'Some changes couldn\'t be saved',
    exampleUnsaved: 'An example has unsaved changes',
    windows: 'Other Tetiva windows will be saved and closed: {n}',
  },
  ru: {
    title: 'Перезапустить Tetiva сейчас?',
    restart: 'Перезапустить',
    cancel: 'Отмена',
    restarting: 'Перезапуск…',
    requestRunning: 'Выполняется запрос',
    websockets: 'Закроется WebSocket-соединений: {n}',
    unsaved: 'Часть изменений не удалось сохранить',
    exampleUnsaved: 'В примере есть несохранённые изменения',
    windows: 'Другие окна Tetiva сохранятся и закроются: {n}',
  },
}
