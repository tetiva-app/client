import { grpcStatusName } from '@/constants/grpc-status'
import type { ExampleProtocol } from '@/types/example'

export function exampleStatusLabel(protocol: ExampleProtocol, code: number): string {
  return protocol === 'grpc' ? `${grpcStatusName(code)} (${code})` : String(code)
}

// A workspace that is not linked to a sync server shares its examples with no one.
export function exampleDeleteDescription(name: string, synced: boolean): string {
  return synced ? `"${name}" will be deleted for everyone in the workspace.` : `"${name}" will be deleted.`
}
