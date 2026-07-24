export interface GraphQLSchema {
  queries: GraphQLOperation[]
  mutations: GraphQLOperation[]
  types: GraphQLType[]
  source: string
}

export interface GraphQLOperation {
  name: string
  args: GraphQLArg[]
  returnType: string
  definition: string
}

export interface GraphQLArg {
  name: string
  type: string
  defaultValue: string
}

export interface GraphQLType {
  name: string
  kind: string
  fields: GraphQLField[]
  enumValues: string[]
  possibleTypes: string[]
  definition: string
}

export interface GraphQLField {
  name: string
  type: string
  args: GraphQLArg[]
}

export interface GraphQLExampleResponse {
  query: string
  variables: string
}

export interface GraphQLIntrospectRequest {
  endpoint: string
  schemaPath: string
  headers: Record<string, string>
}

export interface GraphQLGenerateExampleRequest extends GraphQLIntrospectRequest {
  operationName: string
}

export interface GraphQLGetTypeDefinitionRequest extends GraphQLIntrospectRequest {
  typeName: string
}
