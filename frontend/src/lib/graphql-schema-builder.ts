import { markRaw } from 'vue'
import { buildSchema } from 'graphql'
import type { GraphQLSchema as AppGraphQLSchema } from '@/types/graphql'

/**
 * Build a valid SDL string from the app's GraphQLSchema DTO: wraps bare
 * query/mutation definitions in root type blocks, drops introspection (__) types.
 */
export function buildSdlFromSchema(schema: AppGraphQLSchema): string {
  const parts: string[] = []

  const queries = schema.queries.filter(q => !q.name.startsWith('__'))
  if (queries.length > 0) {
    const fields = queries.map(q => `  ${q.definition}`).join('\n')
    parts.push(`type Query {\n${fields}\n}`)
  }

  const mutations = schema.mutations.filter(m => !m.name.startsWith('__'))
  if (mutations.length > 0) {
    const fields = mutations.map(m => `  ${m.definition}`).join('\n')
    parts.push(`type Mutation {\n${fields}\n}`)
  }

  for (const t of schema.types) {
    if (t.name.startsWith('__')) continue
    if (t.definition) parts.push(t.definition)
  }

  return parts.join('\n\n')
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
