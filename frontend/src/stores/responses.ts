import { ref } from 'vue'
import { defineStore } from 'pinia'
import type { ExecuteResponse } from '@/types/execute'

export type ResponseState =
  | { status: 'idle' }
  | { status: 'loading'; startedAt: number }
  | { status: 'success'; data: ExecuteResponse }
  | { status: 'error'; error: { title: string; detail: string; suggestions: string[] } }

export const useResponseStore = defineStore('responses', () => {
  const responseMap = ref<Map<string, ResponseState>>(new Map())

  function getResponseState(id: string): ResponseState {
    return responseMap.value.get(id) ?? { status: 'idle' }
  }

  function setResponse(id: string, state: ResponseState) {
    responseMap.value.set(id, state)
    responseMap.value = new Map(responseMap.value)
  }

  function cancelRequest(id: string) {
    setResponse(id, { status: 'idle' })
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
    deleteResponse,
  }
})
