import type { Request } from '@/types/request'
import type { Result } from '@/types/common'
import type { ExecuteResponse } from '@/types/execute'
import type { GenerateCurlResponse, ParseCurlResponse } from '@/types/curl'
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
import { RequestService } from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails'
import {
  CreateRequestRequest as BindingCreateRequestRequest,
  EditRequestRequest as BindingEditRequestRequest,
  DeleteRequestRequest as BindingDeleteRequestRequest,
  DeleteDraftRequest as BindingDeleteDraftRequest,
  ReorderRequestRequest as BindingReorderRequestRequest,
  MoveRequestRequest as BindingMoveRequestRequest,
  PromoteDraftRequest as BindingPromoteDraftRequest,
  ExecuteRequestRequest as BindingExecuteRequestRequest,
  GenerateCurlRequest as BindingGenerateCurlRequest,
  ParseCurlRequest as BindingParseCurlRequest,
  GRPCConnectRequest as BindingGRPCConnectRequest,
  GRPCGenerateExampleRequest as BindingGRPCGenerateExampleRequest,
  GRPCGetProtoDefinitionRequest as BindingGRPCGetProtoDefinitionRequest,
  GraphQLIntrospectRequest as BindingGraphQLIntrospectRequest,
  GraphQLGenerateExampleRequest as BindingGraphQLGenerateExampleRequest,
  GraphQLGetTypeDefinitionRequest as BindingGraphQLGetTypeDefinitionRequest,
} from '../../bindings/github.com/tetiva-app/client/internal/adapters/wails/dto'
import { unwrap, type BindingResult } from './unwrap'

// Binding DTOs differ structurally from domain types — the casts are kept narrow
// (binding call result only) so a renamed Go DTO field still fails vue-tsc.
export class WailsRequestService implements RequestServiceAPI {
  async list(collectionId: string): Promise<Result<Request[]>> {
    return unwrap<Request[]>(await RequestService.List(collectionId) as unknown as BindingResult<Request[]>)
  }

  async getById(id: string): Promise<Result<Request>> {
    return unwrap<Request>(await RequestService.GetByID(id) as unknown as BindingResult<Request>)
  }

  async create(req: CreateRequestReq): Promise<Result<Request>> {
    return unwrap<Request>(await RequestService.Create(new BindingCreateRequestRequest({
      collectionId: req.collectionId,
      name: req.name,
      description: req.description,
      protocol: req.protocol,
      method: req.method,
      url: req.url,
      headers: req.headers,
      body: req.body,
      bodyType: req.bodyType,
      authType: req.authType,
      authData: req.authData,
      preScript: req.preScript,
      postScript: req.postScript,
      grpcService: req.grpcService || '',
      grpcMethod: req.grpcMethod || '',
      grpcProtoPath: req.grpcProtoPath || '',
      grpcMetadata: req.grpcMetadata || {},
      graphqlQuery: req.graphqlQuery || '',
      graphqlVariables: req.graphqlVariables || '',
      graphqlSchemaPath: req.graphqlSchemaPath || '',
      graphqlOperation: req.graphqlOperation || '',
    })) as unknown as BindingResult<Request>)
  }

  async edit(req: EditRequestReq): Promise<Result<Request>> {
    return unwrap<Request>(await RequestService.Edit(new BindingEditRequestRequest({
      id: req.id,
      name: req.name,
      description: req.description,
      method: req.method,
      url: req.url,
      headers: req.headers,
      body: req.body,
      bodyType: req.bodyType,
      authType: req.authType,
      authData: req.authData,
      preScript: req.preScript,
      postScript: req.postScript,
      version: req.version,
      grpcService: req.grpcService || '',
      grpcMethod: req.grpcMethod || '',
      grpcProtoPath: req.grpcProtoPath || '',
      grpcMetadata: req.grpcMetadata || {},
      graphqlQuery: req.graphqlQuery || '',
      graphqlVariables: req.graphqlVariables || '',
      graphqlSchemaPath: req.graphqlSchemaPath || '',
      graphqlOperation: req.graphqlOperation || '',
    })) as unknown as BindingResult<Request>)
  }

  async delete(req: DeleteRequestReq): Promise<Result<boolean>> {
    return unwrap<boolean>(await RequestService.Delete(new BindingDeleteRequestRequest({
      id: req.id,
      version: req.version,
    })) as unknown as BindingResult<boolean>)
  }

  async deleteDraft(id: string): Promise<Result<Record<string, never>>> {
    return unwrap<Record<string, never>>(await RequestService.DeleteDraft(new BindingDeleteDraftRequest({ id })) as unknown as BindingResult<Record<string, never>>)
  }

  async reorder(req: ReorderRequestReq): Promise<Result<boolean>> {
    return unwrap<boolean>(await RequestService.Reorder(new BindingReorderRequestRequest({
      id: req.id,
      sortOrder: req.sortOrder,
    })) as unknown as BindingResult<boolean>)
  }

  async execute(req: { requestId: string; workspaceId: string }): Promise<Result<ExecuteResponse>> {
    return unwrap<ExecuteResponse>(await RequestService.Execute(new BindingExecuteRequestRequest({
      requestId: req.requestId,
      workspaceId: req.workspaceId,
    })) as unknown as BindingResult<ExecuteResponse>)
  }

  async generateCurl(req: { requestId: string; workspaceId: string }): Promise<Result<GenerateCurlResponse>> {
    return unwrap<GenerateCurlResponse>(await RequestService.GenerateCurl(new BindingGenerateCurlRequest({
      requestId: req.requestId,
      workspaceId: req.workspaceId,
    })) as unknown as BindingResult<GenerateCurlResponse>)
  }

  async parseCurl(req: { text: string }): Promise<Result<ParseCurlResponse>> {
    return unwrap<ParseCurlResponse>(await RequestService.ParseCurl(new BindingParseCurlRequest({
      text: req.text,
    })) as unknown as BindingResult<ParseCurlResponse>)
  }

  async move(req: MoveRequestReq): Promise<Result<Request>> {
    return unwrap<Request>(await RequestService.Move(new BindingMoveRequestRequest({
      id: req.id,
      targetCollectionId: req.targetCollectionId,
      version: req.version,
    })) as unknown as BindingResult<Request>)
  }

  async promoteDraft(req: PromoteDraftReq): Promise<Result<Request>> {
    return unwrap<Request>(await RequestService.PromoteDraft(new BindingPromoteDraftRequest({
      id: req.id,
      name: req.name,
      targetCollectionId: req.targetCollectionId,
      version: req.version,
    })) as unknown as BindingResult<Request>)
  }

  async saveResponseToFile(tempPath: string, destPath: string): Promise<Result<string>> {
    return unwrap<string>(await RequestService.SaveResponseToFile(tempPath, destPath) as unknown as BindingResult<string>)
  }

  async grpcListServices(req: GRPCConnectRequest): Promise<Result<GRPCSchema>> {
    return unwrap<GRPCSchema>(await RequestService.GRPCListServices(new BindingGRPCConnectRequest({
      host: req.host,
      useTls: req.useTls,
      protoPath: req.protoPath,
    })) as unknown as BindingResult<GRPCSchema>)
  }

  async grpcGenerateExample(req: GRPCGenerateExampleRequest): Promise<Result<string>> {
    return unwrap<string>(await RequestService.GRPCGenerateExample(new BindingGRPCGenerateExampleRequest({
      host: req.host,
      useTls: req.useTls,
      protoPath: req.protoPath,
      service: req.service,
      method: req.method,
    })) as unknown as BindingResult<string>)
  }

  async grpcGetProtoDefinition(req: GRPCGenerateExampleRequest): Promise<Result<string>> {
    return unwrap<string>(await RequestService.GRPCGetProtoDefinition(new BindingGRPCGetProtoDefinitionRequest({
      host: req.host,
      useTls: req.useTls,
      protoPath: req.protoPath,
      service: req.service,
      method: req.method,
    })) as unknown as BindingResult<string>)
  }

  async graphqlIntrospect(req: GraphQLIntrospectRequest): Promise<Result<GraphQLSchema>> {
    return unwrap<GraphQLSchema>(await RequestService.GraphQLIntrospect(new BindingGraphQLIntrospectRequest({
      endpoint: req.endpoint,
      schemaPath: req.schemaPath,
      headers: req.headers,
      workspaceId: req.workspaceId,
    })) as unknown as BindingResult<GraphQLSchema>)
  }

  async graphqlGenerateExample(req: GraphQLGenerateExampleRequest): Promise<Result<GraphQLExampleResponse>> {
    return unwrap<GraphQLExampleResponse>(await RequestService.GraphQLGenerateExample(new BindingGraphQLGenerateExampleRequest({
      endpoint: req.endpoint,
      schemaPath: req.schemaPath,
      headers: req.headers,
      workspaceId: req.workspaceId,
      operationName: req.operationName,
    })) as unknown as BindingResult<GraphQLExampleResponse>)
  }

  async graphqlGetTypeDefinition(req: GraphQLGetTypeDefinitionRequest): Promise<Result<string>> {
    return unwrap<string>(await RequestService.GraphQLGetTypeDefinition(new BindingGraphQLGetTypeDefinitionRequest({
      endpoint: req.endpoint,
      schemaPath: req.schemaPath,
      headers: req.headers,
      workspaceId: req.workspaceId,
      typeName: req.typeName,
    })) as unknown as BindingResult<string>)
  }
}
