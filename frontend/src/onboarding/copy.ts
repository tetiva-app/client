import { pickLocale } from '@/whats-new/notes'

export interface OnboardingChoiceCopy {
  title: string
  tagline: string
  points: string[]
}

export interface OnboardingWelcomeCopy {
  title: string
  subtitle: string
  local: OnboardingChoiceCopy
  account: OnboardingChoiceCopy
  tourLink: string
  hint: string
}

export interface OnboardingTourSlideCopy {
  title: string
  body: string
  alt: string
}

export interface OnboardingTourCopy {
  skip: string
  pause: string
  play: string
  back: string
  next: string
  done: string
  slides: OnboardingTourSlideCopy[]
}

// `sentTo` carries an {email} placeholder and `resendIn` a {seconds} one.
export interface OnboardingVerifyCopy {
  title: string
  sentTo: string
  localHint: string
  resend: string
  resendIn: string
  resentToast: string
  confirmed: string
  logout: string
  close: string
  pollingStopped: string
  rateLimited: string
  verifiedToast: string
  indicatorTooltip: string
}

export interface OnboardingCopy {
  welcome: OnboardingWelcomeCopy
  tour: OnboardingTourCopy
  verify: OnboardingVerifyCopy
}

// Only the first-launch screens and the email-confirmation screen are
// translated; the rest of the UI stays English by design.
export const ONBOARDING_COPY: Record<'ru' | 'en', OnboardingCopy> = {
  ru: {
    welcome: {
      title: 'Добро пожаловать в Tetiva',
      subtitle: 'Выберите, как начать работу',
      local: {
        title: 'Работать локально',
        tagline: 'Всё готово — отправляйте первый запрос прямо сейчас',
        points: [
          'HTTP, gRPC, GraphQL и WebSocket',
          'Коллекции, окружения, скрипты и тесты',
          'Данные остаются на этом компьютере',
        ],
      },
      account: {
        title: 'Подключить аккаунт',
        tagline: 'То же самое плюс синхронизация между устройствами',
        points: [
          'Коллекции на ноутбуке и на рабочем ПК',
          'Общие воркспейсы для команды',
          'Копия данных в облаке, если с компьютером что-то случится',
        ],
      },
      tourLink: 'Показать, что умеет Tetiva — около минуты',
      hint: 'Аккаунт можно подключить позже — иконка облака на панели слева',
    },
    tour: {
      skip: 'Пропустить',
      pause: 'Приостановить',
      play: 'Продолжить',
      back: 'Назад',
      next: 'Дальше',
      done: 'Готово',
      slides: [
        {
          title: 'Четыре протокола, одно окно',
          body: 'Один API, четыре вкладки: HTTP, gRPC, GraphQL и WebSocket. Переключаться между приложениями не нужно.',
          alt: 'Четыре вкладки одного API: HTTP, gRPC, GraphQL и живой поток WebSocket',
        },
        {
          title: 'gRPC без ручной сборки запроса',
          body: 'Импортируйте .proto — методы и тело запроса берутся из схемы. Остаётся подставить значения.',
          alt: 'Список методов gRPC из импортированного .proto и тело запроса, собранное по схеме',
        },
        {
          title: 'Скрипты до и после запроса',
          body: 'Подставить токен, проверить ответ, записать значение в окружение. Обычный JavaScript, без плагинов.',
          alt: 'Скрипты до и после запроса и их результат: два пройденных теста и вывод консоли',
        },
        {
          title: 'Воркспейсы и синхронизация',
          body: 'Проекты разложены по воркспейсам. С аккаунтом они появляются на других устройствах.',
          alt: 'Переключатель воркспейсов: два локальных и один синхронизируемый',
        },
      ],
    },
    verify: {
      title: 'Проверьте почту',
      sentTo: 'Мы отправили письмо на {email}',
      localHint: 'Синхронизация включится сразу после подтверждения. Работать локально можно уже сейчас.',
      resend: 'Отправить ещё раз',
      resendIn: 'Отправить ещё раз через {seconds} с',
      resentToast: 'Письмо отправлено',
      confirmed: 'Я подтвердил',
      logout: 'Выйти',
      close: 'Закрыть',
      pollingStopped: 'Автопроверка остановлена. Нажмите «Я подтвердил», когда перейдёте по ссылке из письма.',
      rateLimited: 'Слишком много писем. Попробуйте через несколько минут.',
      verifiedToast: 'Почта подтверждена',
      indicatorTooltip: 'Подтвердите почту, чтобы включить синхронизацию',
    },
  },
  en: {
    welcome: {
      title: 'Welcome to Tetiva',
      subtitle: 'Choose how you want to start',
      local: {
        title: 'Work locally',
        tagline: 'Everything is ready — send your first request right now',
        points: [
          'HTTP, gRPC, GraphQL and WebSocket',
          'Collections, environments, scripts and tests',
          'Your data stays on this machine',
        ],
      },
      account: {
        title: 'Connect an account',
        tagline: 'The same, plus sync across your devices',
        points: [
          'Your collections on the laptop and on the work desktop',
          'Shared workspaces for the team',
          'A copy in the cloud if you lose the machine',
        ],
      },
      tourLink: 'See what Tetiva can do — about a minute',
      hint: 'An account can be connected any time — the cloud icon in the left bar',
    },
    tour: {
      skip: 'Skip',
      pause: 'Pause',
      play: 'Play',
      back: 'Back',
      next: 'Next',
      done: 'Done',
      slides: [
        {
          title: 'Four protocols, one window',
          body: 'One API, four tabs: HTTP, gRPC, GraphQL and WebSocket. No jumping between apps.',
          alt: 'Four tabs of one API: HTTP, gRPC, GraphQL and a live WebSocket stream',
        },
        {
          title: 'gRPC without hand-building requests',
          body: 'Import a .proto file and both the methods and the request body come from the schema. Only the values are yours to fill in.',
          alt: 'The gRPC method list from an imported .proto and a request body built from the schema',
        },
        {
          title: 'Scripts before and after a request',
          body: 'Attach a token, assert the response, store a value in the environment. Plain JavaScript, no plugins.',
          alt: 'Pre- and post-request scripts and their outcome: two passing tests and console output',
        },
        {
          title: 'Workspaces and sync',
          body: 'Projects live in separate workspaces. With an account they show up on your other machines.',
          alt: 'The workspace switcher: two local workspaces and one synced',
        },
      ],
    },
    verify: {
      title: 'Check your email',
      sentTo: 'We sent a message to {email}',
      localHint: 'Sync turns on the moment you confirm. Local work is available right now.',
      resend: 'Send again',
      resendIn: 'Send again in {seconds}s',
      resentToast: 'Email sent',
      confirmed: 'I confirmed my email',
      logout: 'Sign out',
      close: 'Close',
      pollingStopped: 'Automatic checking stopped. Press “I confirmed my email” after you open the link from the email.',
      rateLimited: 'Too many emails. Try again in a few minutes.',
      verifiedToast: 'Email confirmed',
      indicatorTooltip: 'Confirm your email to turn sync on',
    },
  },
}

export function onboardingCopy(navLang: string): OnboardingCopy {
  return ONBOARDING_COPY[pickLocale(navLang)]
}
