import { ref } from 'vue'
import { defineStore } from 'pinia'
import type { ExecuteResponse } from '@/types/execute'

export interface ResponseError {
  title: string
  detail: string
  suggestions: string[]
  console?: string[]
}

export type ResponseState =
  | { status: 'idle' }
  | { status: 'loading'; startedAt: number }
  | { status: 'success'; data: ExecuteResponse }
  | { status: 'error'; error: ResponseError }

export const useResponseStore = defineStore('responses', () => {
  const responseMap = ref<Map<string, ResponseState>>(new Map())
  const previousDurations = ref<Map<string, number>>(new Map())
  const lastDurations = new Map<string, number>()

  function getResponseState(id: string): ResponseState {
    return responseMap.value.get(id) ?? { status: 'idle' }
  }

  function setResponse(id: string, state: ResponseState) {
    if (state.status === 'success') {
      const last = lastDurations.get(id)
      if (last !== undefined) {
        previousDurations.value.set(id, last)
        previousDurations.value = new Map(previousDurations.value)
      }
      lastDurations.set(id, state.data.durationMs)
    }
    responseMap.value.set(id, state)
    responseMap.value = new Map(responseMap.value)
  }

  function previousDurationMs(id: string): number | undefined {
    return previousDurations.value.get(id)
  }

  function cancelRequest(id: string) {
    setResponse(id, { status: 'idle' })
  }

  function anyLoading(): boolean {
    return Array.from(responseMap.value.values()).some(s => s.status === 'loading')
  }

  function deleteResponse(id: string) {
    responseMap.value.delete(id)
    responseMap.value = new Map(responseMap.value)
  }

  return {
    responseMap,
    getResponseState,
    setResponse,
    cancelRequest,
    anyLoading,
    deleteResponse,
    previousDurationMs,
  }
})
