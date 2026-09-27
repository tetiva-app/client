const grpcStatusNames: Record<number, string> = {
  0: 'OK',
  1: 'CANCELLED',
  2: 'UNKNOWN',
  3: 'INVALID_ARGUMENT',
  4: 'DEADLINE_EXCEEDED',
  5: 'NOT_FOUND',
  6: 'ALREADY_EXISTS',
  7: 'PERMISSION_DENIED',
  8: 'RESOURCE_EXHAUSTED',
  9: 'FAILED_PRECONDITION',
  10: 'ABORTED',
  11: 'OUT_OF_RANGE',
  12: 'UNIMPLEMENTED',
  13: 'INTERNAL',
  14: 'UNAVAILABLE',
  15: 'DATA_LOSS',
  16: 'UNAUTHENTICATED',
}

export function grpcStatusName(code: number): string {
  return grpcStatusNames[code] ?? 'UNKNOWN'
}

export function grpcStatusColor(code: number): string {
  if (code === 0) return 'var(--gc-success)'
  if (code >= 1 && code <= 7 || code === 16) return 'var(--gc-warning)'
  return 'var(--gc-error)'
}
