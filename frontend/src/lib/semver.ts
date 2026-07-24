interface ParsedSemver {
  core: [number, number, number]
  prerelease: string | null
}

function parseSemver(input: string): ParsedSemver | null {
  const cleaned = input.trim().replace(/^v/i, '')
  const m = cleaned.match(/^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?/)
  if (!m) return null
  return {
    core: [Number(m[1]), Number(m[2]), Number(m[3])],
    prerelease: m[4] ?? null,
  }
}

// True if `latest` is strictly newer than `current`. Accepts a leading "v";
// a prerelease `latest` is never offered as an update; malformed input → false.
export function isNewerVersion(latest: string, current: string): boolean {
  const a = parseSemver(latest)
  const b = parseSemver(current)
  if (!a || !b) return false
  if (a.prerelease) return false
  for (let i = 0; i < 3; i++) {
    if (a.core[i] > b.core[i]) return true
    if (a.core[i] < b.core[i]) return false
  }
  // cores are equal; stable latest is newer than a prerelease current
  return b.prerelease !== null
}
