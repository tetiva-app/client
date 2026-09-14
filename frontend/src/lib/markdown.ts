import MarkdownIt from 'markdown-it'

// html stays off: raw HTML in a description would run inside the Wails webview with the bindings in reach.
export const MARKDOWN_OPTIONS = { html: false, linkify: true, breaks: true } as const

const md = new MarkdownIt(MARKDOWN_OPTIONS)

export function renderMarkdown(src: string): string {
  return md.render(src ?? '')
}
