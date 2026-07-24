export interface ResultError {
  code: string
  message: string
  fields?: Record<string, string>
}

export interface Result<T> {
  data: T
  error?: ResultError
}
