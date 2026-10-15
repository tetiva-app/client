import type { Locale } from '@/lib/locale'

export interface EnvironmentCopy {
  modal: {
    title: string
    active: string
    setActive: string
    rename: string
    duplicate: string
    exportPostman: string
    delete: string
    copyName: string
    newEnvironment: string
    importFile: string
    variables: string
    key: string
    value: string
    variableName: string
    secret: string
    notSecret: string
    empty: string
  }
  deleteEnvironment: {
    title: string
    text: string
    confirm: string
    cancel: string
    failed: string
  }
  results: {
    noWorkspace: string
    exported: string
  }
  errors: {
    create: string
    duplicate: string
    edit: string
    delete: string
    setActive: string
    fetchVariables: string
    addVariable: string
    editVariable: string
    deleteVariable: string
  }
  picker: {
    none: string
    manage: string
  }
}

export const ENVIRONMENT_COPY: Record<Locale, EnvironmentCopy> = {
  en: {
    modal: {
      title: 'Manage Environments',
      active: 'Active environment',
      setActive: 'Set as active',
      rename: 'Rename',
      duplicate: 'Duplicate',
      exportPostman: 'Export as Postman',
      delete: 'Delete',
      copyName: '{name} (copy)',
      newEnvironment: 'New environment',
      importFile: 'Import a Postman or Hoppscotch environment',
      variables: 'Variables:',
      key: 'Key',
      value: 'Value',
      variableName: 'Variable name',
      secret: 'Secret (click to unset)',
      notSecret: 'Not secret (click to set)',
      empty: 'Select or create an environment',
    },
    deleteEnvironment: {
      title: 'Delete environment',
      text: 'Delete environment "{name}"? All variables in this environment will be lost.',
      confirm: 'Delete',
      cancel: 'Cancel',
      failed: 'Action failed. Please try again.',
    },
    results: {
      noWorkspace: 'No active workspace',
      exported: 'Exported to {path}',
    },
    errors: {
      create: 'Failed to create environment',
      duplicate: 'Failed to duplicate environment',
      edit: 'Failed to edit environment',
      delete: 'Failed to delete environment',
      setActive: 'Failed to set active environment',
      fetchVariables: 'Failed to fetch variables',
      addVariable: 'Failed to add variable',
      editVariable: 'Failed to edit variable',
      deleteVariable: 'Failed to delete variable',
    },
    picker: {
      none: 'No Environment',
      manage: 'Manage Environments...',
    },
  },
  ru: {
    modal: {
      title: 'Окружения',
      active: 'Активное окружение',
      setActive: 'Сделать активным',
      rename: 'Переименовать',
      duplicate: 'Дублировать',
      exportPostman: 'Экспорт в Postman',
      delete: 'Удалить',
      copyName: '{name} (копия)',
      newEnvironment: 'Новое окружение',
      importFile: 'Импорт окружения из Postman или Hoppscotch',
      variables: 'Переменные:',
      key: 'Ключ',
      value: 'Значение',
      variableName: 'Имя переменной',
      secret: 'Секретная (нажмите, чтобы сделать обычной)',
      notSecret: 'Обычная (нажмите, чтобы сделать секретной)',
      empty: 'Выберите или создайте окружение',
    },
    deleteEnvironment: {
      title: 'Удалить окружение',
      text: 'Удалить окружение «{name}»? Все его переменные будут потеряны.',
      confirm: 'Удалить',
      cancel: 'Отмена',
      failed: 'Не удалось удалить окружение. Попробуйте ещё раз.',
    },
    results: {
      noWorkspace: 'Нет активного пространства',
      exported: 'Экспортировано в {path}',
    },
    errors: {
      create: 'Не удалось создать окружение',
      duplicate: 'Не удалось дублировать окружение',
      edit: 'Не удалось переименовать окружение',
      delete: 'Не удалось удалить окружение',
      setActive: 'Не удалось сделать окружение активным',
      fetchVariables: 'Не удалось загрузить переменные',
      addVariable: 'Не удалось добавить переменную',
      editVariable: 'Не удалось изменить переменную',
      deleteVariable: 'Не удалось удалить переменную',
    },
    picker: {
      none: 'Без окружения',
      manage: 'Настроить окружения…',
    },
  },
}
