// Mirrors portability.FreeName in Go.
export function freeName(name: string, taken: Iterable<string>): string {
  const names = new Set(taken)
  let candidate = name
  for (let n = 2; names.has(candidate); n++) candidate = `${name} (${n})`
  return candidate
}
