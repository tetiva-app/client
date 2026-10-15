const LABELS: Record<string, string> = {
  'pre-script': 'Pre-request',
  'post-script': 'Post-response',
  'variable-persist': 'Saving variables',
}

export function scriptPhaseLabel(phase: string): string {
  return LABELS[phase] ?? phase
}

export function scriptBadge(...scripts: (string | undefined)[]): string {
  const count = scripts.filter(s => s?.trim()).length
  return count > 0 ? String(count) : ''
}
