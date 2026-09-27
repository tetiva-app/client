import { describe, it, expect, vi, beforeEach } from 'vitest'
import { effectScope, nextTick, ref, type Ref } from 'vue'

vi.mock('@/lib/cabinet', () => ({ cabinetPublishedUrl: vi.fn() }))

import { cabinetPublishedUrl } from '@/lib/cabinet'
import { useCabinetLink } from './useCabinetLink'

const discover = vi.mocked(cabinetPublishedUrl)

function mount(visible: Ref<boolean>, offline: Ref<boolean> = ref(false)) {
  const scope = effectScope()
  const url = scope.run(() => useCabinetLink(() => visible.value, () => offline.value))!
  return { url, stop: () => scope.stop() }
}

async function settle() {
  await new Promise(resolve => setTimeout(resolve, 0))
}

beforeEach(() => { discover.mockReset() })

describe('useCabinetLink', () => {
  it('never asks while hidden', async () => {
    const { url, stop } = mount(ref(false))
    await settle()

    expect(discover).not.toHaveBeenCalled()
    expect(url.value).toBeNull()
    stop()
  })

  it('asks again each time the list comes back, as after an account change', async () => {
    discover.mockResolvedValueOnce('https://a.example/app/published').mockResolvedValueOnce('https://b.example/app/published')
    const visible = ref(true)
    const { url, stop } = mount(visible)
    await settle()
    expect(url.value).toBe('https://a.example/app/published')

    visible.value = false
    await nextTick()
    expect(url.value).toBeNull()
    visible.value = true
    await settle()

    expect(url.value).toBe('https://b.example/app/published')
    expect(discover).toHaveBeenCalledTimes(2)
    stop()
  })

  it('drops an answer that lands after the list was hidden', async () => {
    let answer!: (url: string | null) => void
    discover.mockReturnValueOnce(new Promise(resolve => { answer = resolve }))
    const visible = ref(true)
    const { url, stop } = mount(visible)

    visible.value = false
    await nextTick()
    answer('https://a.example/app/published')
    await settle()

    expect(url.value).toBeNull()
    stop()
  })

  it('finds the link once the connection is back while the list stays open', async () => {
    discover.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce('https://a.example/app/published')
    const offline = ref(true)
    const { url, stop } = mount(ref(true), offline)
    await settle()
    expect(url.value).toBeNull()

    offline.value = false
    await settle()

    expect(url.value).toBe('https://a.example/app/published')
    expect(discover).toHaveBeenCalledTimes(2)
    stop()
  })

  it('keeps a link found offline without asking again', async () => {
    discover.mockResolvedValueOnce('https://a.example/app/published')
    const offline = ref(true)
    const { url, stop } = mount(ref(true), offline)
    await settle()

    offline.value = false
    await settle()

    expect(url.value).toBe('https://a.example/app/published')
    expect(discover).toHaveBeenCalledTimes(1)
    stop()
  })

  it('shows no link when discovery fails', async () => {
    discover.mockRejectedValueOnce(new Error('offline'))
    const { url, stop } = mount(ref(true))
    await settle()

    expect(url.value).toBeNull()
    stop()
  })
})
