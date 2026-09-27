import type { Locale, PluralForms } from '@/lib/locale'

export interface SyncCopy {
  title: string
  description: string
  connected: {
    to: string
    as: string
    workspaces: string
    noWorkspaces: string
    disconnect: string
  }
  notices: {
    planLimit: string
    updateRequired: string
    parkedQuota: PluralForms
    parkedTooLarge: PluralForms
    plans: string
  }
  devices: {
    title: string
    loading: string
    lastActive: string
    unknown: string
    current: string
    signOut: string
    signingOut: string
    none: string
    signOutOthers: string
    confirmSignOutOthers: string
    confirm: string
    errors: {
      load: string
      revoke: string
      signOutOthers: string
    }
  }
  errors: {
    notConnected: string
    transport: string
  }
  browser: {
    contacting: string
    waiting: string
    verifyEmail: string
    expiresIn: string
    openAgain: string
    copyLink: string
    copied: string
    anyone: string
    cancelled: string
    tryAgain: string
    signIn: string
    createAccount: string
    opensHost: string
  }
  server: {
    checking: string
    cloudUnreachable: string
    unreachable: string
    cloudOutdated: string
    custom: string
    url: string
    urlPlaceholder: string
    incomplete: string
    leaveEmpty: string
  }
  form: {
    loginTab: string
    registerTab: string
    email: string
    password: string
    name: string
    connect: string
    connecting: string
    register: string
    registering: string
  }
  retry: string
  cancel: string
}

export const SYNC_COPY: Record<Locale, SyncCopy> = {
  en: {
    title: 'Sync',
    description: 'Connect to sync your workspaces across devices.',
    connected: {
      to: 'Connected to',
      as: 'as',
      workspaces: 'Synced workspaces',
      noWorkspaces: 'No synced workspaces. Remote workspaces will appear automatically.',
      disconnect: 'Disconnect',
    },
    notices: {
      planLimit: "Sync paused — the team exceeds its plan's member limit. Ask the owner to update the plan or remove members.",
      updateRequired: 'Sync stopped — update the app to read the newest changes from your team.',
      parkedQuota: {
        one: '{n} change not synced — cloud collection limit reached on your plan. They will sync automatically after an upgrade.',
        other: '{n} changes not synced — cloud collection limit reached on your plan. They will sync automatically after an upgrade.',
      },
      parkedTooLarge: {
        one: '{n} item too large for the server — edit it to retry.',
        other: '{n} items too large for the server — edit them to retry.',
      },
      plans: 'See plans',
    },
    devices: {
      title: 'Devices',
      loading: 'Loading devices...',
      lastActive: 'last active {when}',
      unknown: 'Unknown device',
      current: 'This device',
      signOut: 'Sign out',
      signingOut: 'Signing out...',
      none: 'No other devices.',
      signOutOthers: 'Sign out everywhere',
      confirmSignOutOthers: 'Sign out all other devices?',
      confirm: 'Confirm',
      errors: {
        load: "Couldn't load devices",
        revoke: "Couldn't sign that device out",
        signOutOthers: "Couldn't sign the other devices out",
      },
    },
    errors: {
      notConnected: 'Not connected to the sync server',
      transport: "Couldn't reach the app backend. Try again.",
    },
    browser: {
      contacting: 'Contacting {host}…',
      waiting: 'Waiting for you to finish in the browser — approve the request there, then come back.',
      verifyEmail: 'Confirm your email in the browser to finish signing in.',
      expiresIn: 'Expires in {time}',
      openAgain: 'Open again',
      copyLink: 'Copy link',
      copied: 'Copied',
      anyone: 'Anyone with this link can approve the sign-in.',
      cancelled: 'Sign-in cancelled.',
      tryAgain: 'Try again',
      signIn: 'Sign in with browser',
      createAccount: 'Create account',
      opensHost: 'Opens {host} in your browser',
    },
    server: {
      checking: 'Checking the server…',
      cloudUnreachable: 'Cannot reach Tetiva Cloud.',
      unreachable: 'Cannot reach this server.',
      cloudOutdated: 'This app needs a newer Tetiva Cloud to sign in.',
      custom: 'Use custom server',
      url: 'Server URL',
      urlPlaceholder: 'host:port — e.g. localhost:50051',
      incomplete: 'Enter host:port to check this server.',
      leaveEmpty: 'Leave empty to use {server}.',
    },
    form: {
      loginTab: 'Login',
      registerTab: 'Register',
      email: 'Email',
      password: 'Password',
      name: 'Name',
      connect: 'Connect',
      connecting: 'Connecting...',
      register: 'Register',
      registering: 'Registering...',
    },
    retry: 'Retry',
    cancel: 'Cancel',
  },
  ru: {
    title: 'Синхронизация',
    description: 'Подключитесь, чтобы пространства синхронизировались между устройствами.',
    connected: {
      to: 'Подключено к',
      as: 'под аккаунтом',
      workspaces: 'Синхронизируемые пространства',
      noWorkspaces: 'Синхронизируемых пространств нет. Облачные пространства появятся здесь сами.',
      disconnect: 'Отключиться',
    },
    notices: {
      planLimit: 'Синхронизация на паузе — в команде больше участников, чем позволяет тариф. Попросите владельца сменить тариф или удалить участников.',
      updateRequired: 'Синхронизация остановлена — обновите приложение, чтобы получить последние изменения команды.',
      parkedQuota: {
        one: '{n} изменение не синхронизировано — достигнут лимит облачных коллекций тарифа. Изменения синхронизируются сами после смены тарифа.',
        few: '{n} изменения не синхронизированы — достигнут лимит облачных коллекций тарифа. Изменения синхронизируются сами после смены тарифа.',
        many: '{n} изменений не синхронизировано — достигнут лимит облачных коллекций тарифа. Изменения синхронизируются сами после смены тарифа.',
        other: '{n} изменения не синхронизированы — достигнут лимит облачных коллекций тарифа. Изменения синхронизируются сами после смены тарифа.',
      },
      parkedTooLarge: {
        one: '{n} элемент слишком велик для сервера — измените его, чтобы отправить снова.',
        few: '{n} элемента слишком велики для сервера — измените их, чтобы отправить снова.',
        many: '{n} элементов слишком велики для сервера — измените их, чтобы отправить снова.',
        other: '{n} элемента слишком велики для сервера — измените их, чтобы отправить снова.',
      },
      plans: 'Тарифы',
    },
    devices: {
      title: 'Устройства',
      loading: 'Загружаем устройства…',
      lastActive: 'последняя активность {when}',
      unknown: 'Неизвестное устройство',
      current: 'Это устройство',
      signOut: 'Выйти',
      signingOut: 'Выходим…',
      none: 'Других устройств нет.',
      signOutOthers: 'Выйти на других устройствах',
      confirmSignOutOthers: 'Выйти на всех других устройствах?',
      confirm: 'Подтвердить',
      errors: {
        load: 'Не удалось загрузить устройства',
        revoke: 'Не удалось выйти на этом устройстве',
        signOutOthers: 'Не удалось выйти на других устройствах',
      },
    },
    errors: {
      notConnected: 'Нет подключения к серверу синхронизации',
      transport: 'Приложение не ответило. Попробуйте ещё раз.',
    },
    browser: {
      contacting: 'Связываемся с {host}…',
      waiting: 'Ждём, пока вы закончите в браузере — подтвердите вход там и вернитесь сюда.',
      verifyEmail: 'Подтвердите почту в браузере, чтобы завершить вход.',
      expiresIn: 'Истекает через {time}',
      openAgain: 'Открыть снова',
      copyLink: 'Копировать ссылку',
      copied: 'Скопировано',
      anyone: 'Подтвердить вход может любой, у кого есть эта ссылка.',
      cancelled: 'Вход отменён.',
      tryAgain: 'Попробовать снова',
      signIn: 'Войти через браузер',
      createAccount: 'Создать аккаунт',
      opensHost: 'Откроется {host} в браузере',
    },
    server: {
      checking: 'Проверяем сервер…',
      cloudUnreachable: 'Не удаётся связаться с Tetiva Cloud.',
      unreachable: 'Не удаётся связаться с этим сервером.',
      cloudOutdated: 'Для входа этому приложению нужна более новая версия Tetiva Cloud.',
      custom: 'Свой сервер',
      url: 'Адрес сервера',
      urlPlaceholder: 'хост:порт — например, localhost:50051',
      incomplete: 'Введите хост:порт, чтобы проверить сервер.',
      leaveEmpty: 'Оставьте пустым, чтобы подключиться к {server}.',
    },
    form: {
      loginTab: 'Вход',
      registerTab: 'Регистрация',
      email: 'Почта',
      password: 'Пароль',
      name: 'Имя',
      connect: 'Подключиться',
      connecting: 'Подключаемся…',
      register: 'Зарегистрироваться',
      registering: 'Регистрируем…',
    },
    retry: 'Повторить',
    cancel: 'Отмена',
  },
}
