import { describe, it, expect, vi } from 'vitest'
import { printSchemaSdl } from './graphql-schema-builder'
import type { GraphQLSchema, GraphQLType } from '@/types/graphql'

const op = (name: string, definition: string) => ({ name, args: [], returnType: '', definition })
const type = (name: string, kind: string, definition: string): GraphQLType =>
  ({ name, kind, fields: [], enumValues: [], possibleTypes: [], definition })

function introspected(...extra: GraphQLType[]): GraphQLSchema {
  return {
    queries: [
      op('user', '"""Find a user by ID."""\nuser(id: ID!): User'),
      op('users', 'users(limit: Int = 20): [User!]!'),
      op('__type', '__type(name: String!): __Type'),
    ],
    mutations: [op('createUser', 'createUser(input: CreateUserInput!): User!')],
    types: [
      type('User', 'OBJECT', '"""A customer account."""\ntype User {\n  id: ID!\n  """\n  Display name.\n  Unique per workspace.\n  """\n  name: String!\n}'),
      type('ID', 'SCALAR', 'scalar ID'),
      type('String', 'SCALAR', 'scalar String'),
      type('Int', 'SCALAR', 'scalar Int'),
      type('Boolean', 'SCALAR', 'scalar Boolean'),
      type('DateTime', 'SCALAR', 'scalar DateTime'),
      type('CreateUserInput', 'INPUT_OBJECT', 'input CreateUserInput {\n  name: String!\n}'),
      ...extra,
    ],
    source: 'introspection',
  }
}

describe('printSchemaSdl', () => {
  it('puts root fields inside Query and Mutation', () => {
    const sdl = printSchemaSdl(introspected())

    expect(sdl).toContain('type Query {\n  """Find a user by ID."""\n  user(id: ID!): User\n  users(limit: Int = 20): [User!]!\n}')
    expect(sdl).toContain('type Mutation {\n  createUser(input: CreateUserInput!): User!\n}')
    expect(sdl).not.toContain('__type')
  })

  it('leaves out built-in scalars and keeps custom ones', () => {
    const sdl = printSchemaSdl(introspected())

    expect(sdl).not.toMatch(/^scalar (ID|String|Int|Float|Boolean)$/m)
    expect(sdl).toContain('scalar DateTime')
  })

  it('shows the definitions as they came when graphql-js rejects the schema', () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})

    const sdl = printSchemaSdl(introspected(type('Empty', 'OBJECT', 'type Empty {\n}')))

    expect(sdl).toContain('type Query {\n  """Find a user by ID."""\n  user(id: ID!): User')
    expect(sdl).toContain('type Empty {\n}')
    expect(sdl).toContain('"""A customer account."""\ntype User {')
    expect(sdl).not.toMatch(/^scalar String$/m)
  })
})
