// A lexer just deep enough for the code httpsnippet prints: // comments and quoted strings.
// Everything outside strings and comments is the skeleton; a breakout shows up there.

export type LiteralLanguage = 'go' | 'java' | 'csharp' | 'php'

export interface Scan {
  literals: string[]
  skeleton: string
  errors: string[]
}

interface Read { value: string; end: number; errors: string[] }

const HEX = /^[0-9a-fA-F]+$/

export function scanLiterals(code: string, lang: LiteralLanguage): Scan {
  const src = lang === 'java' ? javaUnicodeEscapes(code) : code
  const literals: string[] = []
  const errors: string[] = []
  let skeleton = ''
  let i = 0
  while (i < src.length) {
    const c = src[i]
    if (c === '/' && src[i + 1] === '/') {
      const end = commentEnd(src, i, lang, errors)
      skeleton += '\n'
      i = end + 1
      continue
    }
    if (c === '"' || (c === "'" && lang === 'php')) {
      const r = readString(src, i, lang)
      literals.push(r.value)
      errors.push(...r.errors)
      skeleton += '""'
      i = r.end + 1
      continue
    }
    if (c === "'" || c === '`' || (lang === 'csharp' && (c === '@' || c === '$') && src[i + 1] === '"')) {
      errors.push(`unexpected ${c} at ${i}`)
    }
    skeleton += c
    i++
  }
  if (lang === 'php' && skeleton.slice(5).includes('?>')) errors.push('?> leaves PHP mode')
  return { literals, skeleton, errors }
}

// PHP ends a // comment at ?> as well as at the line end.
function commentEnd(src: string, from: number, lang: LiteralLanguage, errors: string[]): number {
  const nl = src.indexOf('\n', from)
  const end = nl < 0 ? src.length : nl
  if (lang === 'php' && src.slice(from, end).includes('?>')) errors.push(`?> in a comment at ${from}`)
  return end
}

// javac turns an eligible \uXXXX into its character before it looks for strings or comments.
function javaUnicodeEscapes(src: string): string {
  let out = ''
  for (let i = 0; i < src.length; i++) {
    if (src[i] === '\\') {
      let before = 0
      for (let j = i - 1; j >= 0 && src[j] === '\\'; j--) before++
      const m = before % 2 === 0 ? /^\\u+([0-9a-fA-F]{4})/.exec(src.slice(i, i + 16)) : null
      if (m) {
        out += String.fromCharCode(parseInt(m[1], 16))
        i += m[0].length - 1
        continue
      }
    }
    out += src[i]
  }
  return out
}

function readString(src: string, start: number, lang: LiteralLanguage): Read {
  const quote = src[start]
  if (lang === 'php') return quote === "'" ? readPhpSingle(src, start) : readPhpDouble(src, start)
  const simple = SIMPLE_ESCAPES[lang]
  const breaks = lang === 'csharp' ? /[\n\r\u0085\u2028\u2029]/ : /[\n\r]/
  const errors: string[] = []
  let value = ''
  let i = start + 1
  for (; i < src.length && src[i] !== quote; i++) {
    const c = src[i]
    if (breaks.test(c)) errors.push(`line break inside a string at ${i}`)
    if (c !== '\\') {
      value += c
      continue
    }
    const e = src[++i]
    if (e in simple) value += simple[e]
    else {
      const n = numericEscape(src, i, lang)
      if (!n) errors.push(`bad escape \\${e} at ${i}`)
      else {
        value += n.value
        i = n.end
      }
    }
  }
  if (i >= src.length) errors.push(`unterminated string at ${start}`)
  return { value, end: i, errors }
}

const SIMPLE_ESCAPES: Record<'go' | 'java' | 'csharp', Record<string, string>> = {
  go: { a: '\u0007', b: '\b', f: '\f', n: '\n', r: '\r', t: '\t', v: '\v', '\\': '\\', '"': '"' },
  java: { b: '\b', s: ' ', t: '\t', n: '\n', f: '\f', r: '\r', '"': '"', "'": "'", '\\': '\\' },
  csharp: { "'": "'", '"': '"', '\\': '\\', 0: '\0', a: '\u0007', b: '\b', f: '\f', n: '\n', r: '\r', t: '\t', v: '\v' },
}

// i points at the character after the backslash; end is the last character of the escape.
function numericEscape(src: string, i: number, lang: 'go' | 'java' | 'csharp'): { value: string; end: number } | null {
  const e = src[i]
  const fixed = (len: number, radix: number) => {
    const digits = src.slice(i + 1, i + 1 + len)
    if (digits.length !== len || (radix === 16 && !HEX.test(digits))) return null
    return { value: String.fromCodePoint(parseInt(digits, radix)), end: i + len }
  }
  if (lang === 'go') {
    if (e === 'x') return fixed(2, 16)
    if (e === 'u') return fixed(4, 16)
    if (e === 'U') return fixed(8, 16)
    if (/[0-7]/.test(e) && /^[0-7]{3}$/.test(src.slice(i, i + 3))) return { value: String.fromCharCode(parseInt(src.slice(i, i + 3), 8)), end: i + 2 }
    return null
  }
  if (lang === 'java') {
    const m = /^[0-3][0-7]{0,2}|^[4-7][0-7]?/.exec(src.slice(i, i + 3))
    return m ? { value: String.fromCharCode(parseInt(m[0], 8)), end: i + m[0].length - 1 } : null
  }
  if (e === 'u') return fixed(4, 16)
  if (e === 'U') return fixed(8, 16)
  if (e === 'x') {
    const m = /^[0-9a-fA-F]{1,4}/.exec(src.slice(i + 1, i + 5))
    return m ? { value: String.fromCharCode(parseInt(m[0], 16)), end: i + m[0].length } : null
  }
  return null
}

function readPhpSingle(src: string, start: number): Read {
  let value = ''
  let i = start + 1
  for (; i < src.length && src[i] !== "'"; i++) {
    if (src[i] === '\\' && (src[i + 1] === '\\' || src[i + 1] === "'")) i++
    value += src[i]
  }
  return { value, end: i, errors: i >= src.length ? [`unterminated string at ${start}`] : [] }
}

const PHP_ESCAPES: Record<string, string> = { n: '\n', t: '\t', r: '\r', v: '\v', e: '\u001b', f: '\f', '\\': '\\', $: '$', '"': '"' }

function readPhpDouble(src: string, start: number): Read {
  const errors: string[] = []
  let value = ''
  let i = start + 1
  for (; i < src.length && src[i] !== '"'; i++) {
    const c = src[i]
    if ((c === '$' && /[A-Za-z_{\u0080-￿]/.test(src[i + 1] ?? '')) || (c === '{' && src[i + 1] === '$')) {
      errors.push(`interpolation at ${i}`)
    }
    if (c !== '\\') {
      value += c
      continue
    }
    const rest = src.slice(i + 1)
    const m = /^[0-7]{1,3}/.exec(rest) ?? /^x[0-9a-fA-F]{1,2}/.exec(rest) ?? /^u\{[0-9a-fA-F]+\}/.exec(rest)
    if (rest[0] in PHP_ESCAPES) {
      value += PHP_ESCAPES[rest[0]]
      i++
    } else if (m) {
      const digits = m[0].replace(/^[xu]\{?|\}$/g, '')
      value += String.fromCodePoint(parseInt(digits, m[0][0] === 'x' || m[0][0] === 'u' ? 16 : 8))
      i += m[0].length
    } else value += c
  }
  if (i >= src.length) errors.push(`unterminated string at ${start}`)
  return { value, end: i, errors }
}
