// Mirrors domain.MaxExampleBodyLen on the Go side.
export const MAX_EXAMPLE_BODY_BYTES = 256 * 1024

export function exampleBodyTooLarge(body: string): boolean {
  // UTF-8 never takes fewer bytes than UTF-16 code units, so a long string needs no encoding.
  if (body.length > MAX_EXAMPLE_BODY_BYTES) return true
  return new TextEncoder().encode(body).length > MAX_EXAMPLE_BODY_BYTES
}
