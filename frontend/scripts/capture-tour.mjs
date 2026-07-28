// Stop-motion capture for the onboarding tour clips. One screenshot per beat,
// holds expanded later by assemble-tour.mjs — Playwright's recordVideo drops
// frames and softens the UI. See landing/docs/product-media-recipe.md for the original.
//
//   node scripts/capture-tour.mjs --scene=protocols --theme=dark [--url=http://localhost:5173]
//
// Writes f-0000.png… plus manifest.json (hold lengths) into
// ../../.tour-capture/<scene>-<theme>/, which assemble-tour.mjs turns into an mp4.

import { chromium } from '@playwright/test'
import { mkdir, writeFile, rm, readFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const args = Object.fromEntries(
  process.argv.slice(2).map((a) => a.replace(/^--/, '').split('=')),
)
const SCENE = args.scene ?? 'protocols'
const THEME = args.theme ?? 'dark'
const BASE_URL = args.url ?? 'http://localhost:5173'

const HERE = dirname(fileURLToPath(import.meta.url))
const OUT_DIR = resolve(HERE, '../../.tour-capture', `${SCENE}-${THEME}`)
const APP_VERSION = JSON.parse(
  await readFile(resolve(HERE, '../package.json'), 'utf8'),
).version

// Below ~1080 CSS the URL bar drops the Send button, so the window stays wide
// and the frame is cropped instead.
const VIEWPORT = { width: 1100, height: 700 }

// ActivityBar (48) + sidebar (208) = 256. Cropping them away is what makes the
// editor readable at the tour's 624px slot. Scenes about the tree keep them.
const EDITOR_CLIP = { x: 256, y: 0, width: 844, height: 512 }
// The scripts scene trades the URL bar for the console output at the bottom —
// it is what proves the pre-request phase ran at all.
const SCRIPTS_CLIP = { x: 256, y: 96, width: 844, height: 512 }
const SCENE_CLIP = { workspaces: { x: 0, y: 0, width: 560, height: 339 }, scripts: SCRIPTS_CLIP }

const NOW = '2026-01-01T00:00:00Z'
const WORKSPACE_ID = '00000000-0000-4000-a000-000000000001'

const PINIA = `document.querySelector('#app').__vue_app__.config.globalProperties.$pinia`

const RESPONSE_BODY = JSON.stringify(
  {
    users: [
      { id: 1, name: 'Alice', email: 'alice@example.com' },
      { id: 2, name: 'Bob', email: 'bob@example.com' },
    ],
    total: 2,
    page: 1,
  },
  null,
  2,
)

const makeRequest = (over) => ({
  collectionId: 'c-api', name: '', protocol: 'http', method: 'GET', url: '',
  headers: [], body: '', bodyType: 'none', authType: 'inherit', authData: '',
  preScript: '', postScript: '',
  grpcService: '', grpcMethod: '', grpcProtoPath: '', grpcMetadata: {},
  graphqlQuery: '', graphqlVariables: '', graphqlSchemaPath: '', graphqlOperation: '',
  sortOrder: 0, version: 1, createdAt: NOW, updatedAt: NOW, ...over,
})

const makeTabs = (reqs) => reqs.map((r) => ({
  id: `request:${r.id}`, type: 'request', requestId: r.id,
  name: r.name, method: r.method, protocol: r.protocol,
}))

const loadRequests = (page, reqs, activeId) => page.evaluate(
  ({ pinia, reqs, tabs, activeId }) => {
    const rs = eval(pinia)._s.get('requests')
    reqs.forEach((r) => rs.loadRequest(r))
    rs.openTabs = tabs
    rs.activeTabId = `request:${activeId}`
  },
  { pinia: PINIA, reqs, tabs: makeTabs(reqs), activeId },
)

const waitForText = (page, text) => page.waitForFunction(
  (t) => document.body.innerText.includes(t),
  text,
  { timeout: 8000 },
)

async function freeze(page) {
  await page.addStyleTag({
    content: `*,*::before,*::after{transition:none!important;animation:none!important}
              .cm-cursor{opacity:1!important}`,
  })
}

// Parking the pointer off any row keeps hover chrome out of the frames.
async function parkMouse(page) {
  await page.mouse.move(VIEWPORT.width - 120, VIEWPORT.height - 80)
  await page.waitForTimeout(150)
}

const scenes = {
  // The switcher is the whole story: one click swaps the project tree, and the
  // cloud icon marks which workspaces follow the account to other machines.
  async workspaces(page, shot) {
    const tree = (names) => names.map((name, i) => ({
      id: `c-${name.toLowerCase()}`, name, order: i,
    }))

    const setTree = async (names) => {
      await page.evaluate(
        ({ pinia, now, ws, cols }) => {
          const cs = eval(pinia)._s.get('collections')
          const map = new Map()
          for (const c of cols) {
            map.set(c.id, {
              id: c.id, workspaceId: ws, parentId: null, name: c.name, description: '',
              authType: 'none', authData: '{}', preScript: '', postScript: '',
              sortOrder: c.order, version: 1, createdAt: now, updatedAt: now,
            })
          }
          cs.collectionsMap = map
        },
        { pinia: PINIA, now: NOW, ws: WORKSPACE_ID, cols: names },
      )
      await page.waitForTimeout(500)
    }

    await page.evaluate(
      ({ pinia, now }) => {
        const store = eval(pinia)._s.get('workspaces')
        const ws = (id, name, remote) => ({
          id, name, isActive: false, remoteWorkspaceId: remote,
          version: 1, createdAt: now, updatedAt: now,
        })
        store.workspaces = [
          { ...ws('ws-payments', 'Payments API', null), isActive: true },
          ws('ws-internal', 'Internal Tools', null),
          ws('ws-billing', 'Team / Billing', 'remote-billing-1'),
        ]
      },
      { pinia: PINIA, now: NOW },
    )
    await setTree(tree(['Auth', 'Users', 'Payouts']))
    await freeze(page)
    await parkMouse(page)

    const holds = []
    holds.push([await shot(), 22]) // Payments API, local, its own tree

    await page.getByRole('button', { name: /Payments API/ }).first().click()
    await waitForText(page, 'Create workspace')
    await parkMouse(page)
    holds.push([await shot(), 26, 3]) // three workspaces, one of them cloud

    const billingRow = page.getByRole('button', { name: /Team \/ Billing/ }).first()
    await billingRow.hover()
    await page.waitForTimeout(200)
    holds.push([await shot(), 12])

    await billingRow.click()
    await page.waitForTimeout(300)
    // switchWorkspace would refetch through a mock that ignores workspaceId,
    // so the tree would never change. Flipping isActive and swapping the
    // collections plays the same thing honestly on screen.
    await page.evaluate(
      ({ pinia, id }) => {
        const store = eval(pinia)._s.get('workspaces')
        store.workspaces = store.workspaces.map((w) => ({ ...w, isActive: w.id === id }))
      },
      { pinia: PINIA, id: 'ws-billing' },
    )
    await page.waitForTimeout(400)
    await setTree(tree(['Invoices', 'Subscriptions', 'Webhooks']))
    await parkMouse(page)
    holds.push([await shot(), 34, 4]) // cloud workspace with a different tree

    holds.push([holds[0][0], 12, 3]) // loop seam

    return holds
  },

  // The method list comes out of the .proto, and picking one fills the body in
  // the same beat — only the values are typed by hand.
  async grpc(page, shot) {
    const load = (over) => makeRequest({
      id: 'r-grpc', name: 'ListUsers', protocol: 'grpc',
      method: 'POST', url: 'api.internal:50051', bodyType: 'json',
      grpcProtoPath: '/Users/ada/work/proto/user_service.proto',
      ...over,
    })

    // loadSchema only runs on mount when a service is already set, so the
    // request arrives complete and is then reset — the schema lives in the
    // component and survives, leaving the UI at "no method selected".
    await loadRequests(
      page,
      [load({ grpcService: 'example.v1.UserService', grpcMethod: 'ListUsers' })],
      'r-grpc',
    )
    await waitForText(page, 'Import .proto')
    await page.waitForTimeout(900)

    await page.evaluate(
      ({ pinia, req }) => eval(pinia)._s.get('requests').loadRequest(req),
      { pinia: PINIA, req: load({}) },
    )
    await page.waitForTimeout(400)
    await freeze(page)
    await parkMouse(page)

    const holds = []
    holds.push([await shot(), 14]) // idle: proto chip loaded, no method picked

    await page.getByRole('button', { name: /Select method/ }).click()
    await waitForText(page, 'ListUsers')
    await parkMouse(page)
    holds.push([await shot(), 24, 3]) // two services, five methods, from the schema

    const methodRow = page.getByRole('button', { name: /UNARY\s+ListUsers/ })
    await methodRow.hover()
    await page.waitForTimeout(200)
    holds.push([await shot(), 12])

    await methodRow.click()
    // CodeMirror's text is not reliably in innerText, so wait on the selector
    // label instead and give the generated body a beat to land.
    await waitForText(page, 'UserService / ListUsers')
    await page.waitForTimeout(700)
    // Selecting a method goes through updateField, which marks the tab dirty.
    // Re-syncing the snapshot with what is on screen clears the dot.
    await page.evaluate(
      ({ pinia, req }) => eval(pinia)._s.get('requests').loadRequest(req),
      {
        pinia: PINIA,
        req: load({
          grpcService: 'example.v1.UserService', grpcMethod: 'ListUsers',
          body: '{\n  "page": 0,\n  "pageSize": 0\n}',
        }),
      },
    )
    await page.waitForTimeout(300)
    await parkMouse(page)
    holds.push([await shot(), 26, 3]) // body generated from the schema

    // Editing the body would mean driving CodeMirror: loadRequest does not
    // reach an already mounted editor. The schema tab makes the same point —
    // this shape came from the .proto, not from typing.
    await page.getByRole('button', { name: /Schema/ }).click()
    await waitForText(page, 'ListUsersRequest')
    await parkMouse(page)
    holds.push([await shot(), 22, 3]) // the message definition behind the body

    await page.getByRole('button', { name: /^Body/ }).first().click()
    await page.waitForTimeout(300)
    await parkMouse(page)

    await page.evaluate(
      ({ pinia, body }) => {
        eval(pinia)._s.get('responses').setResponse('r-grpc', {
          status: 'success',
          data: {
            statusCode: 0, statusText: 'OK', url: 'api.internal:50051',
            headers: { 'content-type': ['application/grpc+proto'] },
            body, size: 186, durationMs: 21,
          },
        })
      },
      {
        pinia: PINIA,
        body: JSON.stringify({
          users: [
            { id: 'u_8f3c', name: 'Ada Lovelace', email: 'ada@example.com' },
            { id: 'u_2b71', name: 'Grace Hopper', email: 'grace@example.com' },
          ],
          total: 2,
        }, null, 2),
      },
    )
    await waitForText(page, 'Ada Lovelace')
    await parkMouse(page)
    holds.push([await shot(), 30, 3]) // OK (0) with the response

    holds.push([holds[0][0], 12, 3]) // loop seam

    return holds
  },

  // Scripts are invisible until something proves they ran, so the clip ends on
  // the Tests tab: two passing assertions and console output in one frame.
  async scripts(page, shot) {
    const preScript = `const ts = Date.now().toString()
pm.request.headers.upsert({ key: "Authorization", value: "Bearer " + pm.environment.get("auth_token") })
pm.request.headers.upsert({ key: "X-Timestamp", value: ts })
console.log("signed", pm.request.url)`

    const postScript = `pm.test("200 OK", () => pm.response.code === 200)
pm.test("body has users", () => pm.response.json().users.length > 0)

const data = pm.response.json()
pm.environment.set("user_id", data.users[0].id)
console.log("user_id =", data.users[0].id)`

    await loadRequests(page, [makeRequest({
      id: 'r-list', collectionId: 'c-users', name: 'List users',
      url: 'https://api.example.com/v2/users',
      preScript, postScript,
    })], 'r-list')
    await page.waitForTimeout(700)

    await page.getByRole('button', { name: /Scripts/ }).first().click()
    await waitForText(page, 'Pre-request')
    await page.waitForTimeout(500)
    await freeze(page)
    await parkMouse(page)

    const holds = []
    holds.push([await shot(), 22]) // pre-request script, no response yet

    await page.getByRole('button', { name: 'Post-response' }).click()
    await waitForText(page, 'body has users')
    await parkMouse(page)
    holds.push([await shot(), 26, 3])

    await page.evaluate(
      ({ pinia, body }) => {
        eval(pinia)._s.get('responses').setResponse('r-list', {
          status: 'success',
          data: {
            statusCode: 200, statusText: 'OK',
            url: 'https://api.example.com/v2/users',
            headers: {
              'content-type': ['application/json'],
              'x-request-id': ['7f3a1c8e'],
            },
            body, size: 214, durationMs: 142,
            scriptResult: {
              preConsole: ['signed https://api.example.com/v2/users'],
              postConsole: ['user_id = u_8f3c'],
              tests: [
                { name: '200 OK', passed: true },
                { name: 'body has users', passed: true },
              ],
              errors: [],
            },
          },
        })
      },
      {
        pinia: PINIA,
        body: JSON.stringify({
          users: [
            { id: 'u_8f3c', name: 'Ada Lovelace' },
            { id: 'u_2b71', name: 'Grace Hopper' },
          ],
          total: 2,
        }, null, 2),
      },
    )
    await waitForText(page, '142ms')
    await parkMouse(page)
    holds.push([await shot(), 24, 3]) // 200 OK with the green Tests badge

    await page.getByRole('button', { name: /Tests/ }).click()
    await waitForText(page, 'PASS')
    await parkMouse(page)
    holds.push([await shot(), 40, 3]) // four lines to read — the longest hold

    holds.push([holds[0][0], 12, 3]) // loop seam

    return holds
  },

  // One API through four protocols; the last beat returns to tab one so the
  // loop has no seam.
  async protocols(page, shot) {
    await loadRequests(page, [
      makeRequest({ id: 'r-users', name: 'Users', url: 'https://api.example.com/api/v2/users' }),
      makeRequest({
        id: 'r-getuser', name: 'GetUser', protocol: 'grpc', method: 'POST',
        url: 'api.internal:50051', bodyType: 'json', body: '{\n  "id": "u_8f3c"\n}',
        grpcService: 'example.v1.UserService', grpcMethod: 'GetUser',
      }),
      makeRequest({
        id: 'r-posts', name: 'Posts', protocol: 'graphql', method: 'POST',
        url: 'https://api.example.com/graphql', graphqlOperation: 'posts',
        graphqlQuery: 'query Posts {\n  posts {\n    id\n    title\n    author { name }\n  }\n}',
        graphqlVariables: '{}',
      }),
      makeRequest({
        id: 'r-orders', name: 'Orders', protocol: 'websocket',
        url: 'wss://stream.example.com/v1/orders',
      }),
    ], 'r-users')
    await page.waitForTimeout(600)

    // Mounting the editors registers the response store and warms the async
    // gRPC/GraphQL chunks, so later switches land on a finished frame.
    await page.evaluate(
      ({ pinia, httpBody, grpcBody, graphqlBody }) => {
        const responses = eval(pinia)._s.get('responses')
        responses.setResponse('r-users', {
          status: 'success',
          data: {
            statusCode: 200, statusText: 'OK',
            url: 'https://api.example.com/api/v2/users',
            headers: {
              'content-type': ['application/json'],
              'x-request-id': ['7f3a1c8e'],
              'cache-control': ['no-store'],
            },
            body: httpBody, size: 214, durationMs: 142,
          },
        })
        responses.setResponse('r-getuser', {
          status: 'success',
          data: {
            statusCode: 0, statusText: 'OK', url: 'api.internal:50051',
            headers: { 'content-type': ['application/grpc+proto'] },
            body: grpcBody, size: 134, durationMs: 18,
          },
        })
        responses.setResponse('r-posts', {
          status: 'success',
          data: {
            statusCode: 200, statusText: 'OK',
            url: 'https://api.example.com/graphql',
            headers: { 'content-type': ['application/json'] },
            body: graphqlBody, size: 198, durationMs: 96,
          },
        })
      },
      {
        pinia: PINIA,
        httpBody: RESPONSE_BODY,
        grpcBody: JSON.stringify({
          id: 'u_8f3c', name: 'Ada Lovelace', email: 'ada@example.com',
          role: 'admin', createdAt: '2026-03-07T12:41:09Z',
        }, null, 2),
        graphqlBody: JSON.stringify({
          data: {
            posts: [
              { id: 'p_101', title: 'Shipping API v2', author: { name: 'Ada Lovelace' } },
              { id: 'p_102', title: 'Rate limits explained', author: { name: 'Grace Hopper' } },
            ],
          },
        }),
      },
    )

    const activate = async (id, settleText) => {
      await page.evaluate(
        ({ pinia, id }) => {
          eval(pinia)._s.get('requests').activeTabId = `request:${id}`
        },
        { pinia: PINIA, id },
      )
      if (settleText) await waitForText(page, settleText)
      await page.waitForTimeout(350)
    }

    // Visit every tab once before filming: the async editors mount, their
    // schemas load, and CodeMirror stops marking the requests dirty.
    await activate('r-getuser', 'Import .proto')
    await activate('r-posts', 'query Posts')
    // Variables opens by default and eats 160px the query needs to be legible.
    await page.getByRole('button', { name: /Variables/ }).click()
    await page.waitForTimeout(200)
    await activate('r-orders')
    await activate('r-users', 'alice@example.com')
    await freeze(page)
    await parkMouse(page)

    const holds = []
    holds.push([await shot(), 28]) // HTTP

    await activate('r-getuser', 'Ada Lovelace')
    await parkMouse(page)
    holds.push([await shot(), 27, 3]) // gRPC

    await activate('r-posts', 'Shipping API v2')
    await parkMouse(page)
    holds.push([await shot(), 27, 3]) // GraphQL

    await activate('r-orders')
    await page.waitForTimeout(200)

    // The websocket log is the only motion in the clip, and motion is what
    // explains a stream without a word of copy.
    const wsFrames = [
      '{"event":"order.paid","id":"ord_1042"}',
      '{"event":"order.created","id":"ord_1043","total":76.00}',
      '{"event":"order.shipped","id":"ord_1041"}',
      '{"event":"order.created","id":"ord_1044","total":214.90}',
      '{"event":"order.paid","id":"ord_1043"}',
    ]
    await page.evaluate(
      ({ pinia }) => {
        const ws = eval(pinia)._s.get('websocket')
        const at = (s) => new Date(`2026-01-01T${s}`).getTime()
        ws.states = new Map([[
          'r-orders',
          {
            status: 'connected',
            connectionId: 'tour',
            messages: [
              { id: 'm1', dir: 'system', ts: at('12:04:31.002'), data: 'connected' },
              { id: 'm2', dir: 'out', ts: at('12:04:31.118'), data: '{"action":"subscribe","channel":"orders"}' },
              { id: 'm3', dir: 'in', ts: at('12:04:31.204'), data: '{"event":"subscribed","channel":"orders"}' },
              { id: 'm4', dir: 'in', ts: at('12:04:31.663'), data: '{"event":"order.created","id":"ord_1042","total":128.40}' },
            ],
          },
        ]])
      },
      { pinia: PINIA },
    )
    await waitForText(page, 'order.created')
    await parkMouse(page)
    holds.push([await shot(), 12, 3])

    for (const [i, data] of wsFrames.entries()) {
      await page.evaluate(
        ({ pinia, data, seq }) => {
          const ws = eval(pinia)._s.get('websocket')
          const state = ws.states.get('r-orders')
          const ts = state.messages[state.messages.length - 1].ts + 470
          ws.states = new Map([[
            'r-orders',
            { ...state, messages: [...state.messages, { id: `s${seq}`, dir: 'in', ts, data }] },
          ]])
        },
        { pinia: PINIA, data, seq: i },
      )
      await page.waitForTimeout(80)
      holds.push([await shot(), 4])
    }
    holds.push([holds[holds.length - 1][0], 10]) // let the full log settle

    holds.push([holds[0][0], 12, 3]) // loop seam

    return holds
  },
}

async function main() {
  const scene = scenes[SCENE]
  if (!scene) {
    console.error(`unknown scene: ${SCENE} (have: ${Object.keys(scenes).join(', ')})`)
    process.exit(1)
  }

  await rm(OUT_DIR, { recursive: true, force: true })
  await mkdir(OUT_DIR, { recursive: true })

  const browser = await chromium.launch()
  const ctx = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: 2,
    colorScheme: THEME,
  })
  // Otherwise the welcome screen we are filming the tour for covers the app —
  // and so does What's New, which shows whenever the seen version differs.
  await ctx.addInitScript((settings) => {
    try {
      localStorage.setItem('gophercourier.settings', settings)
    } catch {
      // private mode — the modal shows and the shot is obviously wrong
    }
  }, JSON.stringify({
    lastSeenWhatsNewVersion: APP_VERSION,
    onboardingCompletedAt: '2026-01-01T00:00:00.000Z',
  }))

  // gRPC and GraphQL carry an extra toolbar row, so their default split pushes
  // the response body below the crop. Set before navigation — the panels read
  // this on mount.
  await ctx.addInitScript(() => {
    const splits = {
      'reka:request-response-split': { layout: [38, 62] },
      'reka:grpc-request-response-split': { layout: [40, 60] },
      'reka:graphql-request-response-split': { layout: [45, 55] },
    }
    try {
      for (const [key, value] of Object.entries(splits)) {
        localStorage.setItem(key, JSON.stringify(value))
      }
    } catch {
      // private mode — panels keep their defaults and the crop shows less body
    }
  })

  const page = await ctx.newPage()

  let index = 0
  const shot = async () => {
    const n = index++
    await page.screenshot({
      path: `${OUT_DIR}/f-${String(n).padStart(4, '0')}.png`,
      clip: SCENE_CLIP[SCENE] ?? EDITOR_CLIP,
    })
    return n
  }

  try {
    await page.goto(BASE_URL, { waitUntil: 'networkidle' })
    await page.waitForSelector('#app')
    await page.waitForTimeout(700) // App onMounted fetches finish empty

    const holds = await scene(page, shot)
    await writeFile(
      `${OUT_DIR}/manifest.json`,
      JSON.stringify({ scene: SCENE, theme: THEME, frames: index, holds }, null, 2),
    )
    const total = holds.reduce((sum, [, copies]) => sum + copies, 0)
    console.log(`${OUT_DIR}: ${index} unique frames → ${total} @20fps = ${(total / 20).toFixed(1)}s`)
  } finally {
    await ctx.close()
    await browser.close()
  }
}

await main()
