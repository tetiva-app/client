import type { Request, HeaderItem } from '@/types/request'
import type { Result } from '@/types/common'
import type { ExecuteResponse } from '@/types/execute'
import type { GenerateCurlResponse, ParseCurlResponse } from '@/types/curl'
import type { GRPCSchema, GRPCConnectRequest, GRPCGenerateExampleRequest } from '@/types/grpc'
import type { GraphQLSchema, GraphQLExampleResponse, GraphQLIntrospectRequest, GraphQLGenerateExampleRequest, GraphQLGetTypeDefinitionRequest } from '@/types/graphql'

export interface CreateRequestReq {
  collectionId: string
  name: string
  description: string
  protocol: string
  method: string
  url: string
  headers: HeaderItem[]
  body: string
  bodyType: string
  authType: string
  authData: string
  preScript: string
  postScript: string
  grpcService?: string
  grpcMethod?: string
  grpcProtoPath?: string
  grpcMetadata?: Record<string, string[]>
  graphqlQuery?: string
  graphqlVariables?: string
  graphqlSchemaPath?: string
  graphqlOperation?: string
}

export interface EditRequestReq {
  id: string
  name: string
  // Required, not optional: edit.go assigns every field unconditionally, so a
  // caller that omits the description wipes it.
  description: string
  method: string
  url: string
  headers: HeaderItem[]
  body: string
  bodyType: string
  authType: string
  authData: string
  preScript: string
  postScript: string
  version: number
  grpcService?: string
  grpcMethod?: string
  grpcProtoPath?: string
  grpcMetadata?: Record<string, string[]>
  graphqlQuery?: string
  graphqlVariables?: string
  graphqlSchemaPath?: string
  graphqlOperation?: string
}

export interface DeleteRequestReq {
  id: string
  version: number
}

export interface ReorderRequestReq {
  id: string
  sortOrder: number
}

export interface MoveRequestReq {
  id: string
  targetCollectionId: string
  version: number
}

export interface PromoteDraftReq {
  id: string
  name: string
  targetCollectionId: string
  version: number
}

export interface RequestServiceAPI {
  list(collectionId: string): Promise<Result<Request[]>>
  getById(id: string): Promise<Result<Request>>
  create(req: CreateRequestReq): Promise<Result<Request>>
  edit(req: EditRequestReq): Promise<Result<Request>>
  delete(req: DeleteRequestReq): Promise<Result<boolean>>
  deleteDraft(id: string): Promise<Result<Record<string, never>>>
  reorder(req: ReorderRequestReq): Promise<Result<boolean>>
  execute(req: { requestId: string; workspaceId: string }): Promise<Result<ExecuteResponse>>
  generateCurl(req: { requestId: string; workspaceId: string }): Promise<Result<GenerateCurlResponse>>
  parseCurl(req: { text: string }): Promise<Result<ParseCurlResponse>>
  move(req: MoveRequestReq): Promise<Result<Request>>
  promoteDraft(req: PromoteDraftReq): Promise<Result<Request>>
  saveResponseToFile(tempPath: string, destPath: string): Promise<Result<string>>
  grpcListServices(req: GRPCConnectRequest): Promise<Result<GRPCSchema>>
  grpcGenerateExample(req: GRPCGenerateExampleRequest): Promise<Result<string>>
  grpcGetProtoDefinition(req: GRPCGenerateExampleRequest): Promise<Result<string>>
  graphqlIntrospect(req: GraphQLIntrospectRequest): Promise<Result<GraphQLSchema>>
  graphqlGenerateExample(req: GraphQLGenerateExampleRequest): Promise<Result<GraphQLExampleResponse>>
  graphqlGetTypeDefinition(req: GraphQLGetTypeDefinitionRequest): Promise<Result<string>>
}
