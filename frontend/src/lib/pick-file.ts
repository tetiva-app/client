import type { Dialogs } from '@wailsio/runtime'
import { isWailsEnvironment } from '@/services'

// Wails beta.28 has no error codes, only these texts from the Windows dialog and the server build.
const CANCELLED = 'cancelled by user'
const NO_NATIVE_DIALOGS = 'file dialogs not available in server mode'

export async function pickPath(options: Dialogs.OpenFileDialogOptions, promptMessage: string): Promise<string | null> {
  if (!isWailsEnvironment()) return askForPath(promptMessage)
  try {
    const { Dialogs } = await import('@wailsio/runtime')
    return (await Dialogs.OpenFile({ ...options, AllowsMultipleSelection: false })) || null
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    if (message.includes(NO_NATIVE_DIALOGS)) return askForPath(promptMessage)
    if (!message.includes(CANCELLED)) console.error('[Tetiva] File dialog failed:', err)
    return null
  }
}

function askForPath(message: string): string | null {
  return prompt(message) || null
}
