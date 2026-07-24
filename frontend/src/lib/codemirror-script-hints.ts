import { type CompletionContext, type CompletionResult, snippetCompletion } from '@codemirror/autocomplete'

const pmOptions = [
  { label: 'pm', type: 'namespace', info: 'Postman API' },
  { label: 'pm.environment', type: 'namespace', info: 'Environment variables' },
  snippetCompletion('pm.environment.get("${1:key}")', { label: 'pm.environment.get', type: 'function', info: 'Get variable' }),
  snippetCompletion('pm.environment.set("${1:key}", "${2:value}")', { label: 'pm.environment.set', type: 'function', info: 'Set variable' }),
  snippetCompletion('pm.environment.unset("${1:key}")', { label: 'pm.environment.unset', type: 'function', info: 'Unset variable' }),
  { label: 'pm.request', type: 'namespace', info: 'Request data' },
  { label: 'pm.request.method', type: 'property', info: 'Request method' },
  { label: 'pm.request.url', type: 'property', info: 'Request URL' },
  { label: 'pm.request.headers', type: 'namespace', info: 'Request headers' },
  snippetCompletion('pm.request.headers.upsert({key: "${1:name}", value: "${2:value}"})', { label: 'pm.request.headers.upsert', type: 'function', info: 'Add or update header' }),
  snippetCompletion('pm.request.headers.remove("${1:name}")', { label: 'pm.request.headers.remove', type: 'function', info: 'Remove header' }),
  { label: 'pm.response', type: 'namespace', info: 'Response data (Post-script only)' },
  { label: 'pm.response.code', type: 'property', info: 'Status code' },
  snippetCompletion('pm.response.json()', { label: 'pm.response.json', type: 'function', info: 'Parse response as JSON' }),
  snippetCompletion('pm.response.text()', { label: 'pm.response.text', type: 'function', info: 'Response as text' }),
  snippetCompletion('pm.test("${1:Test name}", function() {\n  ${2}\n});', { label: 'pm.test', type: 'function', info: 'Write a test (Post-script only)' }),
  snippetCompletion('console.log(${1});', { label: 'console.log', type: 'function', info: 'Print to console' })
]

export function scriptHintsSource(context: CompletionContext): CompletionResult | null {
  const word = context.matchBefore(/(pm|console)(\.\w*)*$/)
  if (!word) return null
  if (word.from == word.to && !context.explicit) return null

  return {
    from: word.from,
    options: pmOptions.filter(o => o.label.startsWith(word.text))
  }
}
