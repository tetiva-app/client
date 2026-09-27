import type { SnippetLanguage } from './types'

const LINE_BREAKERS = /[\u0000-\u001f\u007f\u0085\u2028\u2029]/g

export function commentLine(language: SnippetLanguage, text: string): string {
  let safe = text.replace(LINE_BREAKERS, ' ')
  // javac decodes \uXXXX even inside comments.
  if (language === 'java') safe = safe.replace(/\\/g, '\\\\')
  // `?>` leaves PHP mode even inside a // comment.
  if (language === 'php') safe = safe.replace(/\?>/g, '? >')
  return language === 'shell' || language === 'python' ? `# ${safe}` : `// ${safe}`
}
