export interface ScriptResult {
  preConsole: string[]
  postConsole: string[]
  tests: TestResult[]
  errors: ScriptError[]
}

export interface TestResult {
  name: string
  passed: boolean
  error?: string
}

export interface ScriptError {
  phase: string
  message: string
}

export interface ExecuteResponse {
  statusCode: number
  statusText: string
  url: string
  headers: Record<string, string[]>
  body: string
  size: number
  durationMs: number
  isBinary?: boolean
  binaryPath?: string
  suggestedFilename?: string
  scriptResult?: ScriptResult
}
