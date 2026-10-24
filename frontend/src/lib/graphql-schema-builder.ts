import { markRaw } from 'vue'
import { buildSchema, printSchema, specifiedScalarTypes } from 'graphql'
import type { GraphQLSchema as AppGraphQLSchema, GraphQLOperation } from '@/types/graphql'

const builtInScalars = new Set(specifiedScalarTypes.map(t => t.name))

/**
 * Build a valid SDL string from the app's GraphQLSchema DTO: wraps bare
 * query/mutation definitions in root type blocks, drops introspection (__) types
 * and built-in scalars.
 */
export function buildSdlFromSchema(schema: AppGraphQLSchema): string {
  const parts: string[] = []

  const pushRoot = (name: string, ops: GraphQLOperation[]) => {
    const fields = ops.filter(op => !op.name.startsWith('__')).map(op => indent(op.definition))
    if (fields.length > 0) parts.push(`type ${name} {\n${fields.join('\n')}\n}`)
  }
  pushRoot('Query', schema.queries)
  pushRoot('Mutation', schema.mutations)

  for (const t of schema.types) {
    if (t.name.startsWith('__') || builtInScalars.has(t.name)) continue
    if (t.definition) parts.push(t.definition)
  }

  return parts.join('\n\n')
}

function indent(definition: string): string {
  return definition.split('\n').map(line => (line ? `  ${line}` : line)).join('\n')
}

/**
 * Convert the app's GraphQLSchema DTO to a graphql-js schema; null if invalid.
 * markRaw keeps Vue from deep-proxying it (cyclic references would freeze the UI).
 */
export function buildGqlJsSchema(schema: AppGraphQLSchema): ReturnType<typeof buildSchema> | null {
  try {
    const sdl = buildSdlFromSchema(schema)
    if (!sdl) return null
    return markRaw(buildSchema(sdl))
  } catch (err) {
    console.warn('[GraphQL] Failed to build schema from SDL:', err)
    return null
  }
}

export function printSchemaSdl(schema: AppGraphQLSchema): string {
  const gqlSchema = buildGqlJsSchema(schema)
  return gqlSchema ? printSchema(gqlSchema) : buildSdlFromSchema(schema)
}
