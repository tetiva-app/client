import type { HeaderItem } from './request'

export type ExampleProtocol = 'http' | 'graphql' | 'grpc'

export interface Example {
  id: string
  requestId: string
  name: string
  statusCode: number
  statusText: string
  headers: HeaderItem[]
  body: string
  contentType: string
  protocol: ExampleProtocol
  sortOrder: number
  version: number
  createdAt: string
  updatedAt: string
}

export interface ExampleInput {
  name: string
  statusCode: number
  statusText: string
  headers: HeaderItem[]
  body: string
  contentType: string
}

export interface CreateExampleInput extends ExampleInput {
  requestId: string
  protocol: ExampleProtocol
}
