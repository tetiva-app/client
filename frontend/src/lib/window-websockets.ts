import { getWebSocketService } from '@/services'

const RELEASE_TIMEOUT_MS = 1500

async function disconnectWindow() {
  try {
    const res = await (await getWebSocketService()).disconnectWindow()
    if (res.error) throw new Error(res.error.message)
  } catch (err) {
    console.error('Failed to close the websockets of the previous page load:', err)
  }
}

export async function releaseWindowWebSockets() {
  let timer: ReturnType<typeof setTimeout> | undefined
  const gaveUp = new Promise<void>(resolve => { timer = setTimeout(resolve, RELEASE_TIMEOUT_MS) })
  await Promise.race([disconnectWindow(), gaveUp])
  clearTimeout(timer)
}
