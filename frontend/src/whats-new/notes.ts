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
