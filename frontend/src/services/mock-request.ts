import type { Request } from '@/types/request'
import type { Result } from '@/types/common'
import type { ExecuteResponse } from '@/types/execute'
import type { GenerateCurlResponse } from '@/types/curl'
import type { GRPCSchema, GRPCConnectRequest, GRPCGenerateExampleRequest } from '@/types/grpc'
import type { GraphQLSchema, GraphQLExampleResponse, GraphQLIntrospectRequest, GraphQLGenerateExampleRequest, GraphQLGetTypeDefinitionRequest } from '@/types/graphql'
import type {
  RequestServiceAPI,
  CreateRequestReq,
  EditRequestReq,
  DeleteRequestReq,
  ReorderRequestReq,
  MoveRequestReq,
  PromoteDraftReq,
} from './request-api'
import { makeError } from './makeError'

export class MockRequestService implements RequestServiceAPI {
  private requests = new Map<string, Request>()

  async list(collectionId: string): Promise<Result<Request[]>> {
    const items = Array.from(this.requests.values())
      .filter((r) => r.collectionId === collectionId)
      .sort((a, b) => a.sortOrder - b.sortOrder)
    return { data: items }
  }

  async getById(id: string): Promise<Result<Request>> {
    const r = this.requests.get(id)
    if (!r) {
      return makeError<Request>('not_found', `request not found: ${id}`)
    }
    return { data: r }
  }

  async create(req: CreateRequestReq): Promise<Result<Request>> {
    if (!req.name.trim()) {
      return makeError<Request>('validation', 'validation failed', { name: 'required' })
    }

    const now = new Date().toISOString()
    const collectionRequests = Array.from(this.requests.values())
      .filter((r) => r.collectionId === req.collectionId)

    const request: Request = {
      id: crypto.randomUUID(),
      collectionId: req.collectionId,
      name: req.name,
      protocol: (req.protocol as Request['protocol']) || 'http',
      method: (req.method as Request['method']) || 'GET',
      url: req.url || '',
      headers: req.headers || [],
      body: req.body || '',
      bodyType: (req.bodyType as Request['bodyType']) || 'none',
      authType: (req.authType as Request['authType']) || 'none',
      authData: req.authData || '{}',
      preScript: req.preScript || '',
      postScript: req.postScript || '',
      grpcService: req.grpcService || '',
      grpcMethod: req.grpcMethod || '',
      grpcProtoPath: req.grpcProtoPath || '',
      grpcMetadata: req.grpcMetadata || {},
      graphqlQuery: req.graphqlQuery || '',
      graphqlVariables: req.graphqlVariables || '',
      graphqlSchemaPath: req.graphqlSchemaPath || '',
      graphqlOperation: req.graphqlOperation || '',
      sortOrder: collectionRequests.length,
      version: 1,
      createdAt: now,
      updatedAt: now,
    }
    this.requests.set(request.id, request)
    return { data: request }
  }

  async edit(req: EditRequestReq): Promise<Result<Request>> {
    if (!req.name.trim()) {
      return makeError<Request>('validation', 'validation failed', { name: 'required' })
    }

    const existing = this.requests.get(req.id)
    if (!existing) {
      return makeError<Request>('not_found', `request not found: ${req.id}`)
    }
    if (existing.version !== req.version) {
      return makeError<Request>('conflict', `request version conflict: ${req.id}`)
    }

    const updated: Request = {
      ...existing,
      name: req.name,
      method: (req.method as Request['method']) || existing.method,
      url: req.url,
      headers: req.headers,
      body: req.body,
      bodyType: (req.bodyType as Request['bodyType']) || existing.bodyType,
      authType: (req.authType as Request['authType']) || existing.authType,
      authData: req.authData ?? existing.authData,
      preScript: req.preScript ?? existing.preScript,
      postScript: req.postScript ?? existing.postScript,
      grpcService: req.grpcService ?? existing.grpcService,
      grpcMethod: req.grpcMethod ?? existing.grpcMethod,
      grpcProtoPath: req.grpcProtoPath ?? existing.grpcProtoPath,
      grpcMetadata: req.grpcMetadata ?? existing.grpcMetadata,
      graphqlQuery: req.graphqlQuery ?? existing.graphqlQuery,
      graphqlVariables: req.graphqlVariables ?? existing.graphqlVariables,
      graphqlSchemaPath: req.graphqlSchemaPath ?? existing.graphqlSchemaPath,
      graphqlOperation: req.graphqlOperation ?? existing.graphqlOperation,
      version: existing.version + 1,
      updatedAt: new Date().toISOString(),
    }
    this.requests.set(req.id, updated)
    return { data: updated }
  }

  async delete(req: DeleteRequestReq): Promise<Result<boolean>> {
    const existing = this.requests.get(req.id)
    if (!existing) {
      return { data: false, error: { code: 'not_found', message: `request not found: ${req.id}` } }
    }
    this.requests.delete(req.id)
    return { data: true }
  }

  async deleteDraft(id: string): Promise<Result<Record<string, never>>> {
    // Mock: drafts only live in tabsStore (not persisted via mock-request).
    // Drop from in-memory map if present; never error — mirrors real backend's idempotent hard-delete.
    this.requests.delete(id)
    return { data: {} as Record<string, never> }
  }

  async reorder(req: ReorderRequestReq): Promise<Result<boolean>> {
    const existing = this.requests.get(req.id)
    if (!existing) {
      return { data: false, error: { code: 'not_found', message: `request not found: ${req.id}` } }
    }
    existing.sortOrder = req.sortOrder
    return { data: true }
  }

  async move(req: MoveRequestReq): Promise<Result<Request>> {
    const existing = this.requests.get(req.id)
    if (!existing) {
      return makeError<Request>('not_found', `request not found: ${req.id}`)
    }
    if (existing.version !== req.version) {
      return makeError<Request>('conflict', 'version conflict')
    }
    const updated: Request = {
      ...existing,
      collectionId: req.targetCollectionId,
      version: existing.version + 1,
      updatedAt: new Date().toISOString(),
    }
    this.requests.set(req.id, updated)
    return { data: updated }
  }

  async promoteDraft(req: PromoteDraftReq): Promise<Result<Request>> {
    if (!req.name.trim()) {
      return makeError<Request>('validation', 'validation failed', { name: 'required' })
    }
    const existing = this.requests.get(req.id)
    if (!existing) {
      return makeError<Request>('not_found', `request not found: ${req.id}`)
    }
    if (existing.version !== req.version) {
      return makeError<Request>('conflict', 'version conflict')
    }
    const promoted: Request = {
      ...existing,
      name: req.name,
      collectionId: req.targetCollectionId,
      isDraft: false,
      version: existing.version + 1,
      updatedAt: new Date().toISOString(),
    }
    this.requests.set(req.id, promoted)
    return { data: promoted }
  }

  async saveResponseToFile(_tempPath: string, _destPath: string): Promise<Result<string>> {
    // In browser mode, we can't save files — just return success
    return { data: '/mock/saved/response.bin' }
  }

  async execute(execReq: { requestId: string; workspaceId: string }): Promise<Result<ExecuteResponse>> {
    const { requestId: id } = execReq
    const req = this.requests.get(id)
    if (!req) {
      return makeError<ExecuteResponse>('not_found', `request not found: ${id}`)
    }

    const url = req.url.toLowerCase()

    if (url.includes('binary')) {
      return {
        data: {
          statusCode: 200,
          statusText: '200 OK',
          url: req.url,
          headers: { 'Content-Type': ['image/png'] },
          body: '',
          size: 1024,
          durationMs: 89,
          isBinary: true,
          binaryPath: '/tmp/gopher-response-mock',
          suggestedFilename: 'response.png',
        },
      }
    }

    if (url.includes('error')) {
      return makeError<ExecuteResponse>('request_error', 'Connection refused: could not connect to host')
    }

    if (url.includes('slow')) {
      await new Promise(resolve => setTimeout(resolve, 3000))
    }

    if (url.includes('404')) {
      return {
        data: {
          statusCode: 404,
          statusText: '404 Not Found',
          url: req.url,
          headers: { 'Content-Type': ['application/json'] },
          body: '{"error":"not found"}',
          size: 21,
          durationMs: 45,
        },
      }
    }

    if (url.includes('500')) {
      return {
        data: {
          statusCode: 500,
          statusText: '500 Internal Server Error',
          url: req.url,
          headers: { 'Content-Type': ['application/json'] },
          body: '{"error":"internal server error"}',
          size: 32,
          durationMs: 120,
        },
      }
    }

    const body = JSON.stringify({
      users: [
        { id: 1, name: 'Alice', email: 'alice@example.com' },
        { id: 2, name: 'Bob', email: 'bob@example.com' },
      ],
      total: 2,
      page: 1,
    }, null, 2)

    return {
      data: {
        statusCode: 200,
        statusText: '200 OK',
        url: req.url,
        headers: {
          'Content-Type': ['application/json'],
          'X-Request-Id': [crypto.randomUUID()],
          'Cache-Control': ['no-cache'],
        },
        body,
        size: body.length,
        durationMs: 142,
      },
    }
  }

  async generateCurl(req: { requestId: string; workspaceId: string }): Promise<Result<GenerateCurlResponse>> {
    const r = this.requests.get(req.requestId)
    if (!r) {
      return makeError<GenerateCurlResponse>('not_found', `request not found: ${req.requestId}`)
    }
    const cmd = `curl \\\n  '${r.url}'`
    return { data: { command: cmd, scriptResult: null } }
  }

  async grpcListServices(_req: GRPCConnectRequest): Promise<Result<GRPCSchema>> {
    return {
      data: {
        services: [
          {
            fullName: 'example.v1.UserService',
            methods: [
              { name: 'GetUser', inputType: 'example.v1.GetUserRequest', outputType: 'example.v1.GetUserResponse', isServerStream: false, isClientStream: false, protoDefinition: 'message GetUserRequest {\n  string id = 1;\n}\n\nmessage GetUserResponse {\n  string id = 1;\n  string name = 2;\n  string email = 3;\n}\n\nservice UserService {\n  rpc GetUser (GetUserRequest) returns (GetUserResponse);\n}', exampleJson: '{\n  "id": ""\n}' },
              { name: 'ListUsers', inputType: 'example.v1.ListUsersRequest', outputType: 'example.v1.ListUsersResponse', isServerStream: false, isClientStream: false, protoDefinition: 'message ListUsersRequest {\n  int32 page = 1;\n  int32 pageSize = 2;\n}\n\nmessage ListUsersResponse {\n  repeated User users = 1;\n}\n\nservice UserService {\n  rpc ListUsers (ListUsersRequest) returns (ListUsersResponse);\n}', exampleJson: '{\n  "page": 0,\n  "pageSize": 0\n}' },
              { name: 'CreateUser', inputType: 'example.v1.CreateUserRequest', outputType: 'example.v1.CreateUserResponse', isServerStream: false, isClientStream: false, protoDefinition: 'message CreateUserRequest {\n  string name = 1;\n  string email = 2;\n}\n\nmessage CreateUserResponse {\n  string id = 1;\n}\n\nservice UserService {\n  rpc CreateUser (CreateUserRequest) returns (CreateUserResponse);\n}', exampleJson: '{\n  "name": "",\n  "email": ""\n}' },
            ],
          },
          {
            fullName: 'example.v1.OrderService',
            methods: [
              { name: 'GetOrder', inputType: 'example.v1.GetOrderRequest', outputType: 'example.v1.GetOrderResponse', isServerStream: false, isClientStream: false, protoDefinition: 'message GetOrderRequest {\n  string orderId = 1;\n}\n\nmessage GetOrderResponse {\n  string orderId = 1;\n  string status = 2;\n  double total = 3;\n}\n\nservice OrderService {\n  rpc GetOrder (GetOrderRequest) returns (GetOrderResponse);\n}', exampleJson: '{\n  "orderId": ""\n}' },
              { name: 'ListOrders', inputType: 'example.v1.ListOrdersRequest', outputType: 'example.v1.ListOrdersResponse', isServerStream: false, isClientStream: false, protoDefinition: 'message ListOrdersRequest {\n  string userId = 1;\n}\n\nmessage ListOrdersResponse {\n  repeated Order orders = 1;\n}\n\nservice OrderService {\n  rpc ListOrders (ListOrdersRequest) returns (ListOrdersResponse);\n}', exampleJson: '{\n  "userId": ""\n}' },
            ],
          },
        ],
        source: 'reflection',
      },
    }
  }

  async grpcGenerateExample(req: GRPCGenerateExampleRequest): Promise<Result<string>> {
    const schema = (await this.grpcListServices(req)).data
    if (schema) {
      for (const svc of schema.services) {
        if (svc.fullName === req.service) {
          for (const m of svc.methods) {
            if (m.name === req.method) {
              return { data: m.exampleJson }
            }
          }
        }
      }
    }
    return { data: '{}' }
  }

  async grpcGetProtoDefinition(req: GRPCGenerateExampleRequest): Promise<Result<string>> {
    const schema = (await this.grpcListServices(req)).data
    if (schema) {
      for (const svc of schema.services) {
        if (svc.fullName === req.service) {
          for (const m of svc.methods) {
            if (m.name === req.method) {
              return { data: m.protoDefinition }
            }
          }
        }
      }
    }
    return { data: '' }
  }

  async graphqlIntrospect(_req: GraphQLIntrospectRequest): Promise<Result<GraphQLSchema>> {
    return {
      data: {
        queries: [
          { name: 'users', args: [{ name: 'limit', type: 'Int', defaultValue: '' }], returnType: '[User!]!', definition: 'users(limit: Int): [User!]!' },
          { name: 'user', args: [{ name: 'id', type: 'ID!', defaultValue: '' }], returnType: 'User', definition: 'user(id: ID!): User' },
          { name: 'posts', args: [], returnType: '[Post!]!', definition: 'posts: [Post!]!' },
        ],
        mutations: [
          { name: 'createUser', args: [{ name: 'input', type: 'CreateUserInput!', defaultValue: '' }], returnType: 'User!', definition: 'createUser(input: CreateUserInput!): User!' },
          { name: 'updateUser', args: [{ name: 'id', type: 'ID!', defaultValue: '' }, { name: 'input', type: 'UpdateUserInput!', defaultValue: '' }], returnType: 'User!', definition: 'updateUser(id: ID!, input: UpdateUserInput!): User!' },
        ],
        types: [
          { name: 'User', kind: 'OBJECT', fields: [{ name: 'id', type: 'ID!', args: [] }, { name: 'name', type: 'String!', args: [] }, { name: 'email', type: 'String', args: [] }], enumValues: [], possibleTypes: [], definition: 'type User {\n  id: ID!\n  name: String!\n  email: String\n}' },
          { name: 'Post', kind: 'OBJECT', fields: [{ name: 'id', type: 'ID!', args: [] }, { name: 'title', type: 'String!', args: [] }, { name: 'author', type: 'User!', args: [] }], enumValues: [], possibleTypes: [], definition: 'type Post {\n  id: ID!\n  title: String!\n  author: User!\n}' },
          { name: 'CreateUserInput', kind: 'INPUT_OBJECT', fields: [{ name: 'name', type: 'String!', args: [] }, { name: 'email', type: 'String!', args: [] }], enumValues: [], possibleTypes: [], definition: 'input CreateUserInput {\n  name: String!\n  email: String!\n}' },
          { name: 'UpdateUserInput', kind: 'INPUT_OBJECT', fields: [{ name: 'name', type: 'String', args: [] }, { name: 'email', type: 'String', args: [] }], enumValues: [], possibleTypes: [], definition: 'input UpdateUserInput {\n  name: String\n  email: String\n}' },
        ],
        source: 'introspection',
      },
    }
  }

  async graphqlGenerateExample(req: GraphQLGenerateExampleRequest): Promise<Result<GraphQLExampleResponse>> {
    const schema = (await this.graphqlIntrospect(req)).data
    if (schema) {
      const allOps = [...schema.queries, ...schema.mutations]
      const op = allOps.find(o => o.name === req.operationName)
      if (op) {
        const argsStr = op.args.length > 0
          ? `(${op.args.map(a => `$${a.name}: ${a.type}`).join(', ')})`
          : ''
        const query = `query ${op.name}${argsStr} {\n  ${op.name}${op.args.length > 0 ? `(${op.args.map(a => `${a.name}: $${a.name}`).join(', ')})` : ''} {\n    id\n  }\n}`
        const variables = op.args.length > 0
          ? JSON.stringify(Object.fromEntries(op.args.map(a => [a.name, ''])), null, 2)
          : '{}'
        return { data: { query, variables } }
      }
    }
    return { data: { query: '{\n  \n}', variables: '{}' } }
  }

  async graphqlGetTypeDefinition(req: GraphQLGetTypeDefinitionRequest): Promise<Result<string>> {
    const schema = (await this.graphqlIntrospect(req)).data
    if (schema) {
      const t = schema.types.find(t => t.name === req.typeName)
      if (t) return { data: t.definition }
    }
    return { data: '' }
  }
}
