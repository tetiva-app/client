import type { InjectionKey } from 'vue'

export const SHOW_REQUEST_HISTORY: InjectionKey<(requestId: string) => void> = Symbol('showRequestHistory')
