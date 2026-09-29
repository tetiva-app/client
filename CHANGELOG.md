# Changelog

## [Unreleased]

### Added

- Postman collection variables come in with the collection: its root `variable` block becomes an environment named after the collection, created in the same import, with disabled variables off and `secret` ones secret. The import dialog names that environment and says to switch to it in the environment picker, since only one environment is active at a time; in a cloud workspace its warning says the environment is shared with the team too. The preview's host list fills in the non-secret variables instead of showing `{{baseUrl}}`, and a `variable` block that can't be read is skipped with a warning instead of failing the import
- A Postman environment or globals file picked with Import File… in the sidebar is imported as an environment at once: a toast names it and counts its variables, and its Open button opens Manage Environments with that environment selected
- `pm.collectionVariables` in scripts: `get`, `set` and `unset` work on the active environment, the same variables `pm.environment` sees, so Postman scripts that keep values in collection variables no longer fail on it

### Changed

- The update check tells the server which version and OS it runs on (`latest.json?v=1.2.0&os=darwin`), so we can see which releases are in use and whether a new one reaches people. Nothing else is added: no headers, identifiers, account or collections. Settings → Updates shows the exact request, and the Windows installer's first page says so too

### Fixed

- Importing a Postman environment keeps disabled variables disabled; they used to come in enabled. A variable without `enabled`, as older Postman exports write it, stays enabled
- An environment file with a number, boolean, `null` or object value imports instead of failing as invalid JSON: the value comes in as its JSON text, `null` as an empty string
- A failed environment import leaves nothing behind; a variable without a name used to stop it halfway and leave a half-filled environment. Such variables are now skipped with a warning
- An imported environment whose name is taken in the workspace gets a suffix, "Dev (2)", instead of a second "Dev" — the environment of a collection imported from share.tetiva.app too
- A Postman globals file becomes an environment named after the file, or "Globals", with a warning that Tetiva has no global variables; it used to turn into an environment without a word
- A Postman or Tetiva collection file picked in Manage Environments says "This is a collection, not an environment — import it from the sidebar" instead of "environment name is empty", and other environment import errors name what is wrong instead of printing the internal error chain

## [v1.2.0] — 2026-09-27 — Public collections

### Added

- Publish a collection as a read-only page on share.tetiva.app: "Publish…" in the context menu of a top-level collection, or the Publish tab in its overview. The page shows the folder tree, descriptions, requests of all four protocols, code snippets, response examples, the non-secret variables of the environment you pick and, unless you turn them off, scripts. Readers open the collection in Tetiva with one click or download it as Tetiva or Postman JSON. The link (`share.tetiva.app/<name>-<8 characters>`) is fixed at the first publication and survives renames
- Visibility: public, unlisted link (not indexed) or password. Free publishes one collection with a "Made with Tetiva" badge; Pro, Team and Business publish without a limit or badge and unlock unlisted and password pages — on Free those options are locked before you try them. Publishing needs a signed-in account with a confirmed email and a server that supports it: Tetiva Cloud does, and on an older or self-hosted server without it the Publish tab says publishing is unavailable. The Publish tab of an unpublished collection and the Publish dialog show a thumbnail of how the page looks, and on a plan with unlisted links suggest publishing one first to see the page before it can turn up in search
- Nothing is published without a preview: the number of folders, requests and examples, which variables go out and which stay hidden, every value that was masked, and warnings for strings that look like credentials (JWTs, AWS, GitHub, Slack, Stripe, Google and Telegram keys, private keys, literal Bearer tokens) or variables named like secrets but not marked secret, with a "Make secret" button. A masked value can be published as is with its own toggle; with warnings, Publish waits for "I've checked this". Over the 8 MiB limit the preview lists the largest examples
- Secrets are scrubbed before the snapshot leaves the device: secret auth fields, credential headers and gRPC metadata, secret query parameters and userinfo in every URL, secrets inside JSON, XML, form and GraphQL bodies, and secret variables (published empty). Cookies and OAuth tokens are never read, file paths shrink to a file name, and `{{variable}}` references always stay as written
- The Publish tab of a published collection shows its status and link, "Changed since publication", views, opens in Tetiva and downloads, the visibility, environment and scripts it was published with — an environment this device doesn't have is marked "not on this device" — and offers "Review & update…", Copy link, Open and Unpublish. A page blocked by the platform says so with the reason. Deleting a published collection takes its page down, and retries later if you are offline
- A Publications panel in the activity bar lists the published collections of the active workspace, those published from another device included, with their visibility and whether each page is up to date. A badge on its icon counts the pages whose collection has changed since publication. A row opens the collection's Publish tab; its hover buttons copy the link or open the page, and an out-of-date row adds "Update…". With nothing published the panel explains what a page gives and offers "Publish a collection…"; signed out it asks you to sign in, offline it shows the last known state, and after a refusal on the Free plan limit it says so and links to the plans
- "Publishing in this client" in Settings → Publishing: turned off, it hides the Publish tab, the publishing items of the collection menu and the Publications panel. Pages already published stay online. The section links to your pages in the web cabinet when the server tells the app where it is
- Import from a link: "Import from Link…" in the sidebar takes a share.tetiva.app link or its slug, and "Open in Tetiva" on the page launches the app through `tetiva://import?slug=…` — whether Tetiva is closed, running or still starting. A password-protected page asks for the password. On Linux under Wayland a link opened while Tetiva runs may raise a "Tetiva is ready" notification instead of the window; the confirmation is already open when you switch to it
- Every import of a published collection stops at a confirmation first: its name, number of requests, environment, the workspace it goes to (a cloud workspace shares it with the whole team), the hosts its requests call and its scripts with their text. Scripts come in only when you tick "Import scripts", which is off by default. File fields and binary bodies are never bound to paths on your disk, and secret variables arrive empty and marked secret. A Tetiva collection file downloaded from a page opens through the regular Import too
- The arrow next to Send, Query, Invoke and Connect opens a Copy as menu: Copy as cURL and a "Copy as" submenu with Python (requests), JavaScript (fetch), Go, Java (HttpClient and OkHttp), C# and PHP (Guzzle) for HTTP and GraphQL, Copy as gRPCurl for gRPC, Copy as websocat and Copy as JavaScript for WebSocket. One click copies the request as it is in the editor, unsaved edits included, with variables and secret values substituted; a toast confirms it and mentions any warnings. This works in a detached request window too, which now shows toasts
- "Generate code…" at the bottom of the menu opens a dialog with a language picker, a preview, the warnings and a Copy button, on the language you last copied — remembered separately for HTTP/GraphQL, gRPC and WebSocket; Details in a warning toast opens it as well. In the dialog "Resolve variables" (on by default) substitutes the active environment; unresolved `{{variables}}` stay as references, and secret ones too unless "Include secret values" is ticked
- Copy as cURL on an HTTP request is built as before, by the same pipeline as Send. Everything else comes from the snippet generator, which sends nothing: auth is applied — OAuth 2.0 only from a token already on hand, never fetched — and workspace cookies go into a `Cookie` header; the pre-request script does not run, and a warning says so; Digest and AWS Signature V4 are left as a comment; files named in the body are referenced by name, never read
- Response examples: "Save as example" in the status bar of an HTTP, GraphQL or gRPC response saves it under a name (by default the status, e.g. "200 OK"), and the Examples tab of the request lists, shows, edits, creates and deletes them. Drafts are kept until you save them, even across tabs and windows. The body is limited to 256 KB and binary responses cannot be saved; `Set-Cookie`, `Authorization`, `Proxy-Authorization` and other credential headers are masked when an example is saved, and an example that still looks like it holds credentials asks before saving
- Examples sync between your devices and with your team. It needs server 0.19: an older or self-hosted server without it keeps them on the device until it is updated. After the update the app fetches, once per workspace, the examples already in the cloud, and deleting a request deletes its examples everywhere. Clients 1.1.1 and older do not see examples
- Postman import and export carry examples (`response`), scripts (`prerequest` and `test` events), urlencoded and multipart form bodies — file fields without their paths — and auth inheritance. gRPC requests are skipped on export with a warning
- Language setting in Settings → Interface: System, English or Russian (beta), switched without a restart. Russian covers Settings, everything about publishing, the collection tree with its search, import and the context menus of collections, folders and requests with the dialogs they open, the Sync dialog, the Publications panel and the browser pages after sign-in and OAuth; the welcome screen and What's New, which followed the OS language, follow this setting now. The request editor and the rest of the app stay in English. Browser sign-in, registration and publishing pass the chosen language to the server instead of the OS one
- Settings is a wider dialog with sections — Interface, Editor, Publishing, AI agents (MCP), Updates and About — and a search that finds a setting by its English or Russian name. The theme is picked from thumbnails, Editor previews the font size and word wrap on a code sample, AI agents shows how an agent reaches your data through the MCP server, and Updates shows the request the update check sends, with no account, collections or settings in it
- After a browser sign-in, or when an OAuth 2.0 authorization code comes back, the browser tab shows a Tetiva page, light or dark after the system theme, instead of a line of plain text, in the language the app had when the flow started. An OAuth error gets its own text for `access_denied`, `invalid_scope`, `server_error` and `temporarily_unavailable` and a general one for any other code; nothing from the address is shown on the page. A repeated hit on the callback while the app is still listening gets a styled page instead of a bare error: "This link has already been used" after sign-in, "This authorization link was already handled" after OAuth
- A `.deb` package for ARM64 Linux (Ubuntu 22.04 and newer)
- The Windows installer's first page says how the update check works and where to turn it off

### Changed

- The app checks for a new version at most once a day instead of once every ten days
- Postman import leaves scripts out unless you tick "Import scripts" in the import dialog, which shows how many the file has: a collection-level `pm.environment.set` in someone else's file used to rewrite your `baseUrl` on the first Send
- One copy of the app runs per data directory: launching it again hands its arguments (a `tetiva://` link, say) to the copy already running and exits. `TETIVA_DATA_DIR` gives a copy its own directory

### Fixed

- Copy as cURL no longer lets curl read a local file: a body starting with `@` went out as `-d` and a text form field starting with `@` or `<` as `-F`, and curl read the file they named. They are written as `--data-raw` and `--form-string` now
- A pasted `curl` command with `--globoff` or `-g` keeps its URL; the flag used to swallow it
- Closing a detached request window saves its edits and example drafts first; Cmd+W, the close button or quitting from the main window used to drop them
- macOS: text fields no longer autocorrect or capitalize what you type — a header key or collection name `address` stays `address` instead of being offered as `Address`
- Linux: the in-window File/Edit/View/Window menu bar is gone — it was unreadable on a light GTK theme; Ctrl+W still closes the tab
- Request docs preview no longer wraps inline code in backticks, and a quoted block loses the curly quotes and the italics it was given
- The empty response pane names the shortcut your platform actually has: Ctrl Enter on Linux and Windows, ⌘ Enter on macOS
- Dragging the request/response splitter all the way up can no longer cut the Docs toolbar in half — in the HTTP, gRPC and GraphQL editors alike
- Environment menu items are left-aligned; "Manage Environments…" no longer wraps into a centred two-liner
- Sidebar rows and editor tabs reached with Tab draw a focus ring inside themselves instead of the thick system outline around them
- The "?" buttons next to section titles show the app tooltip instead of the system one, on hover or keyboard focus only — so it no longer pops up as the Sync dialog opens, where it also swallowed the first Escape
- Buttons, tabs, menu items and dialog close buttons show the hand pointer, and a disabled one shows the not-allowed cursor instead of the arrow
- In the dark theme the hovered item of a dropdown menu is highlighted; it used to blend into the menu
- A collection's section tabs look like the request editor's: an underline under a tab as wide as its name, instead of a box around the active one and the rest stretched across the pane
- A long URL fades out before the environment chip instead of being cut mid-character
- Linux: Ctrl+W closes a detached request window and the schema window, and such a window also closes on its own when you switch workspace or delete the request it holds
- Windows: the installer names the publisher Saveliy Yudin, and installing over 0.15.3–1.1.1 removes the copy those versions left under the old name; your data and settings stay
- A local workspace that sync links to the cloud for the first time uploads the collections, requests, examples and environments it already holds; before, only later edits went out, and the server refused requests added to a folder it had never received. A workspace linked before, under this account or another, uploads nothing again
- The Default environment every install starts with now syncs: it came with the same id on every install, so the server kept it for whoever uploaded it first and refused everyone else's variables ("parent environment not found"). A workspace linked for the first time gives it an id of its own and uploads it with its variables; a copy that came from the cloud and a workspace linked before keep theirs
- A folder edited while it waits to sync still reaches the cloud before the folders and requests inside it; before, the server could refuse those and they never uploaded
- An edit saved while its previous version was still being sent is no longer marked synced by that earlier confirmation, so a resync sends it again instead of skipping it
- What's New fits the window: its list scrolls between the title and the "Got it" button, which a long list used to push out of a 1280×800 window
- Copy as cURL copies the request as it is in the editor: it saves the request first instead of copying the last saved version, and copies nothing if the save fails
- Cancel in the GraphQL and gRPC editors stops waiting for the answer; it used to do nothing. An answer that arrives after Cancel, or after a newer run, no longer replaces the response on screen — in the HTTP editor too. The call itself is not aborted

## [v1.1.1] — 2026-09-14 — Docs editor fixes and synced request descriptions

### Added

- Request descriptions sync between devices and reach the server. Needs server 0.18 or newer; an edit coming from an older client leaves your local docs untouched instead of clearing them
- Descriptions autosave 1.5 s after you stop typing, and switching or closing a tab writes whatever is still unsaved in it — collection scripts and auth included. Closing the window tries one last save. Cmd+S still works
- Descriptions are limited to 16 KiB; a byte counter appears near the limit and a too-long text is rejected with a clear message instead of a silent sync stall
- Descriptions written before this version are offered to the server once on first launch and after a resync; the server keeps them unless a newer device deliberately cleared the field

### Fixed

- Cmd+S, Cmd+F, Cmd+A, Cmd+, and Cmd+[ / ] now work on Cyrillic and other non-Latin layouts
- AltGr combinations (AltGr+S and friends on Windows and Linux layouts) and Cmd/Ctrl+Shift no longer trigger the plain Cmd+S, Cmd+F, Cmd+A, Cmd+, and Cmd+[ / ]
- Closing a tab while a save was in flight could drop the edits typed during that save; a failed collection save no longer closes the tab
- The description editor ignored every second update arriving from sync and let Ctrl+Z revert synced text as if you had typed it
- Undo history in the Docs tab survives switching to Params and back (and Overview → Scripts on a collection); the gRPC, GraphQL and WebSocket editors keep it too
- Enter on an empty last table row leaves a blank line, so the next paragraph is not swallowed by the table
- Pipes inside inline code in a table are escaped when you press Tab or Enter in the table, switch to preview or leave the editor, so the preview keeps every cell
- Inline code, code block and link commands escape backticks, brackets and parentheses in the selection
- Postman export keeps empty folders and writes request docs where Postman reads them; import keeps HTML descriptions as text and reports when an item and its request carry different docs
- A rejected save says which field is wrong ("description: must be at most 16384 bytes") instead of "validation failed"
- Postman import no longer fails on a description over 16 KiB: it is truncated and the import reports it
- Renaming a request no longer fails when an autosave was in flight
- An item the server refuses as too large no longer stalls sync: it is parked with a notice and goes out again after your next edit to it. It is counted apart from the plan limit and shown on its own line, without an offer to upgrade that would not help

## [v1.1.0] — 2026-09-07 — Browser sign-in

### Added

- cURL import by paste: a `curl ...` command pasted into the URL bar is split into method, address, headers, body and auth. A toast lists what was imported. The Undo button appears only when the paste overwrote existing headers, body or auth — then the toast stays until dismissed and disappears as soon as you start editing the request, so Undo can no longer wipe out fresh changes. In an empty request there is nothing to undo and the toast fades on its own. Auth is mentioned only when the request had auth and the paste replaced or removed it
- Description (docs) on a request: a Markdown field next to the request itself, like the one collections have. It travels through Postman import and export and is available to agents over MCP (`create_request`/`update_request`/`get_request`)
- An access token for the MCP server: generated on first launch, passed as `Authorization: Bearer` or as `?token=`; without it the server answers 401. Settings show the token, reissue it and hold a "Require token" toggle; "Copy config" hands out a config with the token already in it
- Tables in a description can now be filled from the keyboard: Tab and Shift+Tab walk the cells, Enter moves to the next row and appends one at the end, Enter in an empty last row leaves the table, and columns align themselves. The "Table" button opens a size picker, as does the `/table` command; inside a table it turns into a "Table actions" menu — insert or delete a row or column, align, move
- Pasting from Excel or Numbers (tab-separated text) into a description turns into a Markdown table
- A WebSocket request is edited like any other: Params, Auth, Headers, Scripts and Docs tabs. The script runs before connecting (Pre-connect) — it can fetch a token into a variable, and that variable is substituted both into the address and into the handshake headers
- Workspace cookies go out with the WebSocket handshake, and `Set-Cookie` from the server's response is stored back. GraphQL requests and schema introspection now use the same container
- `{{Variables}}` are substituted in outgoing messages, not only in the address and headers
- Binary frames: the message format switches between JSON, Text and Binary. In Binary the field takes base64 and goes out as a binary frame; incoming binary frames show up in the log marked `bin`
- Saved messages: a composed message can be saved under a name and sent with one click. Double-click renames it, the cross deletes it
- Socket settings next to the Connect button: a keepalive ping every N seconds (0 disables it) so proxies do not drop an idle connection, and a list of subprotocols for the handshake
- WebSocket joined the history filter as a "WS" chip
- OAuth 2.0, JWT Bearer, Digest and AWS Signature V4 auth — on a request and on a collection that nested requests inherit from
- OAuth 2.0 with the client credentials and password grants: "Get token" fetches a token, its lifetime is shown next to it, and an expiring token is refreshed through refresh_token before the request goes out. Tokens live in the local database, are not synced and stay out of exports — on another device you fetch them again
- OAuth 2.0 authorization code with PKCE through the system browser: "Get token" opens the browser and a local listener on 127.0.0.1 catches the provider's response (the port is configurable, 0 picks a free one). Device code shows the code and the confirmation link. Both have a Cancel button and live status in the Auth tab of the request and the collection; reloading the window resumes the wait where it left off. The provider link can be reopened or copied if the browser did not open or the tab was closed, and configuration errors appear under the fields they belong to
- JWT is signed at send time — HS256/384/512, RSA, RSA-PSS, ECDSA and EdDSA. Claims and the JOSE header are edited as JSON, `iat` and `exp` are filled in when the claims do not carry them; the token goes into a header or a query parameter
- Digest: the server picks the algorithm (MD5, SHA-256, SHA-512-256). The challenge and the retry happen inside a single request, so history keeps one response, and the cookies the server binds the nonce to are stored in the workspace container. The `-sess` variants are not supported — they produce a specific error instead of an empty 401
- AWS Signature V4 signs `host`, every `x-amz-*` header, `content-type` and `content-md5` — enough for S3 with `x-amz-content-sha256` and for JSON APIs with `x-amz-target`; temporary STS keys (session token) work. Digest and SigV4 are HTTP-only and are not offered for gRPC, GraphQL or WebSocket
- Postman import and export understand oauth2, jwt, digest and awsv4, and `curl --digest` and `curl --aws-sigv4` are parsed on paste and emitted on copy. Schemes Tetiva does not have (hawk, ntlm and the rest) no longer vanish silently — the import reports what did not come over
- A request set to Inherit says which scheme the parent collection uses and how long its token is good for; the collection name links to its Auth tab
- Browser sign-in for Tetiva Cloud: "Sign in with browser" opens the site, where signing in, registering and confirming an email happen once for every device; the app picks up the session itself and shows a waiting panel with cancel, reopen link and copy. A self-hosted server is detected automatically: if it cannot do browser sign-in, the old email and password form stays
- Linux build: a `.deb` package for Ubuntu 22.04 and newer (amd64). It installs as `tetiva` in `/usr/bin` and shows up in the application menu; it needs GTK 3 and WebKit2GTK 4.1, which apt pulls in

### Changed

- Tab no longer moves focus out of the description editor: inside a list it indents, Shift+Tab outdents, Esc leaves the editor. Backticks close in pairs and inline code gets a background
- The app platform moved to Wails v3 beta.16 (from alpha.74, March 2026): WebSocket and sync events now arrive strictly in the order they were sent and no longer balloon memory on a stream of large messages; request bodies over 512 KB are passed to the backend in chunks, which lifts the WebView2 limit on Windows; macOS crashes on plugging and unplugging monitors are gone, as are Windows crashes on a display scale change and on resume from sleep
- The `/` menu in a description orders commands by how useful they are for documentation instead of alphabetically: heading, list, code, table, quote, link — it opens on `/heading`, not `/code`
- Request and collection descriptions are written in an editor rather than a bare text field: Markdown highlighting, lists continued on Enter, soft wrap, a toolbar with Bold, Italic, code, link, heading, lists, quote, code block and table, the Cmd/Ctrl+B, I, K shortcuts and a `/` menu at the start of a line. The buttons work with and without a selection, and neither the cursor nor the undo history jumps
- A description is rendered by default instead of shown as source: the "Edit" pencil is always visible, clicking the text itself no longer opens the editor (it used to break selection and link clicks), and an empty description shows an "Add description" button instead of an empty frame. Links in a description open in the system browser
- A rendered description finally looks rendered: headings, lists, tables and quotes used to draw as plain text because the styling classes were there but the styles themselves were missing from the build
- MCP `update_request` changes only the fields it was given: any call used to blank out everything the agent did not send — including real credentials it had read masked earlier
- The MCP server no longer lets anyone in: it used to listen on `127.0.0.1:9300` with no checks at all, and any process on the machine — a browser tab included — could read and change collections, requests and variables. Existing agent configs have to be reissued with "Copy config" after the update
- MCP masks more than `auth_data`: credential header values (`Authorization`, `Cookie`, `X-API-Key` and relatives) and query parameters such as `token`, `api_key` and `signature` in the URL and in the `send_request` response are replaced with `[redacted]`. `{{variable}}` substitutions are left alone — the agent needs them
- Substituting `{{variables}}` into auth data no longer breaks JSON: values go into the parsed document, not into its text, so quotes and backslashes in a password or key do not corrupt the request
- A scheme's draft survives a change of auth type: leave Digest for AWS and come back, and the login and password you typed are still there
- Replaying a request from history no longer carries credentials: neither the Authorization header nor the query parameters auth added end up in the new request
- "Copy as cURL" does not go to the network for a token: the command is built from the token already on hand, and without one it omits the Authorization header and warns
- Update the app on all your devices at once: an older version will not accept a collection with a new auth scheme, and sync stops with "Update required" until it is updated
- You have to sign in again after the update — sign-in moved to the browser, and old sessions are closed once. The app reminds you at first launch
- The email and password form for Tetiva Cloud is gone — sign-in is browser only; it stays for self-hosted servers that do not support it

### Fixed

- Replaying a request from history opened an empty tab: the draft was evicted from memory on the first refresh of the collection list, because the list does not return drafts, and the tab was left with no data. Drafts are no longer evicted, so the "Replay: …" tab shows the request and closing it deletes the draft, as intended
- Concurrent writes to the local database no longer fail with "database is locked": the lock wait timeout is set in the connection string and applies to every connection in the pool, not just one
- The server build (`-tags server`, the app in a browser on `:8080`) used to run silently on mock services even though the Go backend was answering: that build does not inject the runtime into the HTML, so it appears after the environment check. Opening `http://127.0.0.1:8080/?wails=1` is now enough
- Detached window and schema window positions reset once: macOS used to save coordinates in a different frame of reference (on Retina screens the numbers were twice the real ones), so the old values are discarded. Window size is kept as before
- A saved position outside the screen's working area no longer puts a window past the edge — after unplugging a second monitor, for instance, the window opens centred. The window prefs file is versioned now and corrupted entries are skipped
- The size and position of detached windows are saved when the app quits from the main window too, not only when the window itself is closed
- `wails3 dev` shows the UI again: the Vite dev server listens on `127.0.0.1`, because the Wails beta proxy reaches it over IPv4 only
- The `/` menu shows which command is selected again: the highlight came from the `--accent` token, which in both themes is nearly the menu's own background, so the arrow keys moved an invisible selection. The selected row is now filled with the brand purple and hover is dimmer so it does not compete with the keyboard selection
- The "Sync to server" checkbox no longer swallows a new workspace: with it checked the workspace did not appear at all — not in the cloud, not on disk. It is created locally first and in the cloud after that, and if the server refuses or is unreachable it stays on the device with an explanation. The notice waits to be dismissed and no longer confuses "no connection" with "the account was refused"
- Request descriptions are no longer wiped by server data: they are not part of the sync contract yet, and an incoming record overwrote them with an empty value
- Quitting the app with an MCP agent connected no longer panics on server shutdown and no longer hangs: a connected client could keep the app open indefinitely, and shutdown now fits in its time budget with the remaining connections closed
- MCP does not write the mask over credentials: only an exact match with `[redacted]` used to be checked, so `{"token":"[redacted]"}` or `Bearer [redacted]` from an agent overwrote the real secret. A mask inside a value is now rejected with an explanation, while the mask on its own still means "leave it as it was"
- MCP masks percent-encoded query parameters: `access%5Ftoken=...` was not recognised as a secret and went to the agent in full, even though the receiver reads it as `access_token`
- MCP `update_request` and `update_collection` no longer demand a name: they only change the fields they are given, but the schema marked `name` required
- Editing query parameters in the Params tab reaches the URL bar every time again: the variable-aware field dropped every second programmatic change
- Postman import no longer loses documentation: a description written on the request itself rather than on the collection item used to disappear, and a description given as a `{content, type}` object — a shape v2.1 allows alongside a plain string — broke the import outright
- An API key in requests imported from Postman goes where the collection says again: the import wrote the location under the name `in` while sending read `addTo`, so the key always ended up in a header instead of a query parameter. No re-import needed — sending and the Auth tab both understand the old name now
- WebSocket history keeps the real handshake: the response code (101, or 401 or 403 on refusal), request and response headers and the connection time. It used to hold a stub that said nothing about how the attempt ended
- The first frames a server sends right after connecting (a greeting, current state) no longer get lost: the client subscribes to messages before it opens the connection
- Headers set by a pre-request script now go through variable substitution too, in every protocol. The script used to receive headers with values already substituted while its own `{{variables}}` went to the server verbatim

## [v0.17.0] — 2026-08-16 — Devices and plan limits

### Added

- Links to the tetiva.app/docs documentation from the app: a "Documentation" item in Settings and contextual "?" marks next to MCP, scripts, the gRPC editor, sync and the environments window — each opens its own docs page
- A "Devices" section in the sync window: the devices signed in to your account, last activity, a marker for the current one, sign-out for a single device and "Sign out everywhere"
- Plan limit notices: when the server refuses a sync because of a quota (cloud collections, team members), the app explains why in a toast and offers to open the pricing page. Data stays on disk
- A "Sync paused — plan limit reached" state on the sync indicator: on a quota refusal the client keeps retrying instead of treating the server as unreachable
- An ID conflict message: when an entity already belongs to another cloud workspace, the app says so instead of skipping it silently
- The client introduces itself to the server with a User-Agent of the form `Tetiva/version (os; host)` — that is what identifies a device in the list

### Changed

- Signing out ends the session on the server too, not only on this device
- A plan limit is now visible all the time: the sync indicator shows how many changes have not reached the cloud, and the notice stays until dismissed and arrives once per episode instead of every five minutes

### Fixed

- After re-linking to a different cloud workspace, the sync cursor of the previous one no longer sticks around: the new one's contents load right away
- A plan limit is detected precisely: any server refusal with a `ResourceExhausted` code used to count — an oversized change batch, for example — and sync stopped "because of the plan" when the cause was something else
- Records refused on quota are no longer dropped from the queue for good: they stay in it and are retried every five minutes, so data reaches the cloud on its own after a plan change
- Device list: "Loading devices" while it loads, "No other devices" with a single device, and a readable message plus a "Retry" button in place of the raw server error. Sign-out buttons are disabled while a request is in flight
- A device in the list is labelled "host · OS · version" instead of a bare User-Agent; a session with no activity no longer shows "NaN" where the last activity time goes
- The sync indicator turned amber instead of grey on a plan limit: the stop is visible at a glance
- Signing out during a background token refresh no longer leaves someone else's refresh token in the database
- Server calls for the device list, single sign-out and sign-out-everywhere are time-limited and do not freeze the window when the server is unreachable
- A repeat plan-limit refusal shows a notice again: the "limit already shown" flag used to be cleared only by a successful push, so a client that came back with an empty queue said nothing about the next limit
- Records deferred on quota return to the queue by themselves after five minutes — they used to wait for the next local edit or a reconnect
- Records deferred on a plan limit are retried after a restart too: the wake-up lived only in process memory, so after a restart they waited for the next local edit in that workspace
- The unsent-changes counter includes records deferred until a retry: they no longer read as "everything is pushed"
- Sync window: the device label no longer gets cut off at the version (last activity moved to a second line), the connected state's content no longer runs past the right edge, and losing the backend connection no longer leaves the button stuck on "Connecting..."
- The sync window explains why changes are not reaching the cloud: hitting a plan limit shows an amber banner with the reason and a link to pricing, and the window itself no longer jumps when it opens
- Starting the app with the server unreachable no longer kills sync: the engine starts offline and reconnects on its own

## [v0.16.1] — 2026-08-12 — MCP and script sandbox security

### Security

- **MCP DevTools no longer opens a port to the outside.** The server listened on `:9300`, that is on every network interface, and handed workspaces, collections, requests and environment variables to anyone on the local network without authentication, and let them run requests as the user. The default address is now `127.0.0.1:9300`
- An address like `:9300` (no host) is normalised to `127.0.0.1:9300` at startup — that also fixes the saved settings of anyone who enabled MCP earlier. An explicit host (`0.0.0.0:9300`, a LAN address) is kept as is: that is a deliberate choice
- Starting on a non-loopback address writes a warning to the log, and Settings show a prominent warning next to the address field about unauthenticated network access
- MCP tools no longer hand out secrets: `auth_data` on requests and collections (bearer tokens, basic auth passwords) is replaced with `[redacted]`, keeping only whether auth is set and its type; values of variables flagged secret are not returned by `list_variables`, `get_environment` or in the response to creating a variable. Accepting values on write is unchanged
- **A script from someone else's collection can no longer freeze the app.** Regular expressions with backreferences and lookahead (`/(a+)+\1$/`, `/^(?=(a+)+$)a*$/`) run by backtracking and cannot be stopped on a timeout. A script now runs separately from the request: after five seconds the request fails and the window and the other tabs keep working
- gRPC metadata reaches a script as a copy. `pm.request.metadata.set()` in a pre-script still changes the metadata of the outgoing request, but the request in the collection is untouched, and the post-script sees the metadata that actually went to the server

### Fixed

- `pm.request.metadata.set()` on a request with no metadata could crash the app

## [v0.16.0] — 2026-07-27 — Welcome screen and email confirmation

### Added

- A welcome screen on first launch: choose "work locally" or "connect an account", with no forced sign-up. Copy in Russian and English by system locale
- An optional four-screen tour — protocols, gRPC from `.proto`, scripts and tests, workspaces. It opens from a link and can be reopened from Settings via "Show welcome"
- After sign-up the app says the email was sent and waits for confirmation: automatic status checks, resend, and "I confirmed" and "Sign out" buttons
- An indicator in the left panel shows the email is not confirmed and opens the sync window on click

### Changed

- Sync only starts after the email is confirmed. Local work is available immediately with no restrictions; with the server unreachable sync starts as before

### Fixed

- The refresh token is no longer wiped from the local database on sign-in and sign-up — the session survives a restart even when the system keychain is unavailable

## [v0.15.3] — 2026-07-16

### Fixed

- Windows: fixed WebView2 environment detection — the app sends real requests instead of showing demo data
- Windows: the installer and the exe properties said "My Product" instead of Tetiva

## [v0.15.2] — 2026-07-14

### Changed

- Improved stability and reliability of the cloud connection

## [v0.15.1] — 2026-07-13

### Fixed

- **The active workspace now connects to sync right after sign-in.** Connecting to the server used to mirror remote workspaces without linking the active local one, so the sync engine never started for it and the cloud icon stayed grey. On connect, sign-up and app start, an active workspace with no remote link is now created on the server and enrolled in sync
- In the Sync window the "Tetiva Cloud" label no longer sits off the line (the cloud icon is back on the text baseline)

## [v0.15.0] — 2026-07-12

### Added

- **Update notifications**: a dot badge on the settings icon when a new version is available; a check no more than once every 10 days with a "Check for updates automatically" toggle (on by default; with it off no requests are made); the manifest comes from our sync server
- **A "What's New" window** shown once after installing a version that has notes, and available from Settings at any time. The language (RU/EN) follows the system locale, the notes ship inside the app and work fully offline

## [v0.14.1] — 2026-07-12

### Fixed

- **Operation errors are no longer swallowed** — any failed create, delete or save shows a toast with the reason
- **Saving a request became safe**: text typed while a save is in flight is no longer overwritten by the server response; a second Cmd+S during a save does not create a competing parallel request; the tab stays open if the save failed
- **Cmd+S / Cmd+Enter no longer fire twice** on GraphQL and gRPC tabs and no longer leak through modals; Cmd+[ / Cmd+] no longer clash with indentation in the code editor
- **Cmd+W closes the active tab** instead of the whole app (the app menu is defined explicitly now)
- Create and rename dialogs are protected against double submit and stay open on an error
- Deleting a collection closes the tabs of every nested collection and request
- The native context menu is available in text fields again (copy/paste/spellcheck)

## [v0.14.0] — 2026-07-12

### Added

- Connecting to sync uses the Tetiva Cloud server by default — no address to type
- A self-hosted server goes behind a "Use custom server" toggle; the address is normalised automatically (`https://` and a trailing `/` are trimmed, `:443` is appended when there is no port)

### Changed

- In the Sync window a cloud connection shows as "Tetiva Cloud" instead of the technical server address

## [v0.13.1] — 2026-07-12

### Fixed

- **Sync no longer goes strange after the Mac sleeps.** The access token lifetime was measured on Go's monotonic clock, which stops during sleep on macOS: after waking, the client presented a long-expired token for up to ~13 minutes, got Unauthenticated and fell into a shared backoff of up to 5 minutes. `expiresAt` is now stored without the monotonic part (`Round(0)`), and an `Unauthenticated` answer on any sync RPC drops the cached token and forces a refresh on the next attempt.
- **A rejected refresh token no longer means a silent retry forever.** If the server answers Unauthenticated to Refresh itself (session revoked or lost), the syncer stops and moves to a new `auth_expired` state; the sync icon says "Session expired — sign in to resume sync" instead of an endless CloudOff.
- **The sync indicator stopped flickering.** The engine keeps one syncer per organization workspace, and a `sync:status` event from any background workspace flipped the global icon; polling put it back 5 seconds later. The indicator now only reacts to events from the active workspace.
- **The refresh token is always mirrored into the SQLite fallback.** The copy in the DB used to be updated only when the keychain failed; if a later load hit the 2-second keychain timeout (typical right after sleep), the fallback returned an empty or long-rotated token and sync went offline for a backoff cycle.

## [v0.13.0] — 2026-07-12

### Changed

- **Rebranding: the app is now called Tetiva.** Window title, welcome screen, settings window, tab titles, the MCP server ("Tetiva DevTools") and the MCP config presets (`mcpServers.tetiva`), the bundle (`productName: Tetiva`, `productIdentifier: yudinsv.com.Tetiva`).
- **Data migrates from the old name automatically — nothing to configure.** (1) Data directory: on first start `~/.gophercourier` is renamed to `~/.tetiva` (SQLite, WAL and settings move as they are); if the rename is not possible, the app keeps using the old directory. (2) Keyring: the sync login refresh token is read from the new `tetiva` service and, when absent, picked up from the legacy `gophercourier` service and migrated; Logout clears both. (3) Env variables: `TETIVA_DATA_DIR`/`TETIVA_MCP`/`TETIVA_MCP_ADDR` are the new names, and the old `GOPHERCOURIER_*` still work as a fallback.

## [v0.12.1] — 2026-06-16

### Fixed

- **Binary Office files (xlsx/docx/pptx) no longer render as gibberish.** Detecting a binary response by `Content-Type` moved from a naive substring search to proper media type parsing. A type like `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` was mistaken for text because `openxmlformats`/`spreadsheetml` contain `xml` — and the raw bytes of the ZIP archive were rendered as text instead of a "Save to file" button. The `+xml`/`+json` structured suffixes (RFC 6839) are honoured, so `image/svg+xml`, `application/atom+xml` and `application/ld+json` are still treated as text.
- **A `Content-Disposition: attachment` header now forces the response to be treated as a download** regardless of `Content-Type` (a `text/csv` with `attachment`, say, is offered for saving instead of shown inline).

## [v0.12.0] — 2026-06-15

### Added
- **WebSocket** support (Raw, RFC 6455): connect to `ws://`/`wss://`, send text messages (with Send or **Cmd/Ctrl+Enter**), a live log of incoming and outgoing messages with timestamps and direction, connection state, and an active environment selector. Environment variables (`{{var}}`) are substituted in the URL; headers and auth apply to the handshake. The log is ephemeral (it lives as long as the tab is open), while the connection itself is written to history.

## [v0.11.0] — 2026-06-14 — MCP settings in the settings window

### Added

- **An MCP / DevTools section in Settings.** Server status (Running/Stopped), the SSE endpoint URL and "Copy config" with presets for AI clients (Claude Desktop / Cursor / raw URL).
- **MCP server control from the UI:** an Enable toggle and the port. Both are saved and applied on the next launch ("Restart required to apply"). When MCP is set through env variables (`GOPHERCOURIER_MCP` / `GOPHERCOURIER_MCP_ADDR`) the fields are locked and show "Managed by environment variables".

### Technical details

- A new generic KV settings layer: an `app_settings` table, a `settings` domain usecase and a SQLite repository — the base for future settings.
- `internal/app/mcp.go` now always wires the MCP module; SSE starts in a lifecycle hook once the DB is ready, and env overrides the persisted values.
- A Wails `SettingsService` (`GetMCPSettings` / `SetMCPSettings`) plus a frontend service with a mock mode.
- Added the first `fx.ValidateApp` test — `go test` now catches DI graph errors.

## [v0.10.0] — 2026-06-13 — Settings: theme, editor, updates

### Added

- **A settings page (modal)** — unlocked in the ActivityBar, opened by the gear at the bottom of the panel or with `Cmd/Ctrl+,` (it works with no tabs open). Changes apply immediately, with no Save button.
- **Light / Dark / System colour theme.** The default is `System` (the theme follows the macOS setting, as in Bruno and Hoppscotch). The theme is applied by an inline script in `index.html` before the first paint — no flash at startup. `System` reacts to an OS scheme change through `matchMedia`.
- **Editor settings:** CodeMirror font size (12/13/14/16 px) and word wrap. Both apply to `CodeEditor` and `CodeViewer` without recreating the editor (CM6 `Compartment` plus the `--gc-editor-font-size` CSS variable).
- **A light syntax highlighting theme** for the editors — readable on white (One Dark used to be hardcoded and unreadable in the light theme).
- **An About section:** the app version and an update check through the GitHub Releases API. Any failure (private repo → 404, rate limit, network, timeout) shows a neutral "Couldn't check for updates" rather than a false "up to date".

### Technical details

- Settings live in `localStorage` (every access wrapped in try/catch) and are synced between app windows through the `storage` event.
- The version reaches the frontend through Vite `define` (`__APP_VERSION__`) from `package.json` — one source shared with Go's `AppVersion`.

## [v0.9.8] — 2026-06-10 — Browser mock mode fix and sidebar alignment

### Fixed

- **Browser mock mode broke after the first lazy import of the Wails runtime** (`frontend/src/services/index.ts`). `isWailsEnvironment()` checked `window._wails` on every call, but the `@wailsio/runtime` npm package sets `window._wails` as a side effect of the import itself — even in a plain browser. Any `await import('@wailsio/runtime')` (events in App.vue, dialogs, clipboard) turned the browser into a Wails environment, and services created after that (RequestService on the first request creation, for example) picked the Wails implementation → every call 404'd on `/wails/runtime`. The environment is now detected once when the services module loads: in a real Wails window the injected `runtime.js` sets `_wails` before app modules run, so the boot-time check is reliable and later imports no longer matter.
- **Collection and request names in the sidebar were centred instead of left-aligned** (`CollectionItem.vue`, `RequestItem.vue`). Tree rows are `<button>`s, and the default UA `text-align: center` was inherited by the text spans. Added `text-left`. The bug showed in the Wails window too.

## [v0.9.7] — 2026-05-20 — Keyring scoping per client_id

### Fixed

- **The macOS Keychain entry is now scoped by `client_id`** (account = `refresh_token:<client_uuid>` instead of a constant `refresh_token`). Two Wails clients running on one Mac under the same email used to overwrite each other's refresh token in the keychain — on refresh the server returned `client_id mismatch: unauthorized` and the second client lost its session. Each installation now uses its own keyring entry, isolated by a per-install `client_id`. The legacy unscoped entry is deleted best-effort on `Logout` for cleanup. The SQLite fallback is unchanged (already isolated per install through `DATA_DIR`).

## [v0.9.6] — 2026-05-20 — sync_queue.sending recovery at startup

### Fixed

- **`sync_queue` rows stuck in `status='sending'` are now reset to `pending` at app start** (`ResumeOnStartup` calls the already existing `SyncQueueRepo.ResetSending`). If the client crashed mid-push, rows stayed in `sending` forever — the push worker only reads `pending`, so those operations were lost and never retried. The reset runs **before** the sync configuration check, which guarantees recovery even if sync was switched off between sessions.

## [v0.9.5] — 2026-05-18 — MCP sync_pause / sync_resume / sync_disconnect_stream

### Added

- **Three new MCP tools** for simulating offline scenarios. LWW conflicts used to be hard to reproduce — after realtime delivery the version is bumped locally, and the client-side optimistic lock rejects a stale edit BEFORE it reaches the server's LWW resolver. Now a controlled offline window can be opened.

  | Tool | What it does |
  |---|---|
  | `sync_pause { workspace_id }` | Stops the push/pull/realtime applier for a workspace. The outbox queue accumulates local edits. Idempotent. |
  | `sync_resume { workspace_id }` | Restores it: drain queue → pull increments → re-subscribe. The same state transitions as a cold start (Pushing → Pulling → Subscribing → Connected). Idempotent. |
  | `sync_disconnect_stream { workspace_id }` | Drops the current subscribe stream once. The existing exponential backoff (5s–5min) in `subscribeLoop` takes over. For testing reconnect and provoking `ResyncRequired`. |

- **Engine API**: `engine.Pause(workspaceID)`, `engine.Resume(workspaceID)`, `engine.DisconnectStream(workspaceID)` — all idempotent and race-safe (test `PauseResume_ConcurrentSafe` under `-race`), returning clear errors on bogus IDs instead of panicking.

### Technical details

- **`Pause`** sets `paused bool` on `workspaceSyncer` under `ws.mu`, and `ws.cancel()` stops the goroutine. The syncer stays in `engine.workspaces` so `Resume` can read `remoteWorkspaceID` and `lastSyncSeq`.
- **`Resume`** removes the paused syncer from the map and calls `StartWorkspace` — a fresh goroutine with the same remote ID and last seq.
- **`DisconnectStream`** — every `subscribe()` now creates a per-stream child context (`streamCtx`) that `DisconnectStream` cancels. `subscribeLoop` sees a `nil` error and enters the existing backoff path.
- **`InjectRawSyncer`** — a test-only helper for white-box MCP tests without standing up gRPC.

### Testing

- 13 new unit tests in `engine_test.go` (idempotency, queue retention on Pause, state transitions, concurrent safety under `-race`).
- 5 smoke tests of the MCP tool handlers in `server_test.go` (unknown workspace → error, injected syncer → success).
- All tests plus `-race` are green.

## [v0.9.4] — 2026-05-18 — Sync wiring V2 and a reactive status indicator

### Fixed

- **Connecting to sync works again after the server-side workspaces V2 redesign.** Remote workspaces were not fetched and `StartWorkspace` was never called — the client sat in "Not connected" even with the server available. The full chain now runs: `OrgService.ListMine → WorkspaceService.ListByOrg → upsert workspace into SQLite → StartWorkspace per workspace`.
- **`SyncAuthManager.storeTokens` now saves `active_org_id`** from the login/register/refresh response into `sync_config`. Without it the server failed in `pickActiveOrg` on subscribe (no default active org → 400). The signature changed to `storeTokens(access, refresh, activeOrgID string)`.
- **A 2-second timeout on `keyring.Set/Get`** — the keyring inside the Wails sandbox sometimes hangs forever, especially on the macOS Keychain modal prompt. Without a timeout the whole sync startup stalled. A timeout falls back to an ephemeral access token, and the refresh token is lost until the next login (a deliberate trade-off — the UI does not freeze).
- **`SyncStatusIndicator` became reactive** — the TopBar icon used to stay grey for ~5s after a restart even when the engine was already `connected`. The cause: the component only polled `GetStatus()` every 5s and never subscribed to the `sync:status` event Go emits at `engine.go:254` on each state transition (`Pushing` → `Pulling` → `Subscribing` → `Connected`). Added `Events.On('sync:status', …)` in `onMounted` — state changes are instant now. Polling is kept only for the `pending` counter, which no event covers.

### Testing

- `internal/infrastructure/sync/auth_test.go` updated for the new `storeTokens` signature (passing `activeOrgID`).

## [v0.9.3] — 2026-05-07

### Fixed
- **Switching body type no longer clears the content** — changing Body Type (JSON → XML, say) used to make the current body disappear. It was in fact kept in a per-type draft, but it looked like everything had been wiped. Transitions inside the text family (`json` ↔ `xml` ↔ `raw`) now keep the same text and only change the parser, the highlighting and the Content-Type. Form and Binary still use separate drafts, since their structure differs.

### Added
- **JSONC comments in a JSON request body** (parity with Postman) — `// line` and `/* block */` comments can be used in a JSON body to disable lines without deleting them. Comments are stripped on the backend before sending (`request.stripJSONC`); History keeps the original text with comments. Strings containing `//` or `/*` are handled correctly, with no confusion inside string literals.
- The CodeMirror JSON linter no longer flags JSONC comments as syntax errors.

### Testing
- Go: `TestStripJSONC` (line, block, strings, EOF, unterminated block), `TestExecute_StripsJSONCCommentsFromWireBody` (wire body without comments and valid JSON, history with the original).
- Frontend: `useBodyDrafts.test.ts` — `isTextFamily`/`isTextFamilyTransition` plus a regression on saving and restoring drafts.

## [v0.9.2] — 2026-04-27

### Changed
- **Redesigned the History sidebar filters** — the inline panel took ~280px of vertical space, about half the sidebar, leaving room for one or two entries. The filters are compact now:
  - **A permanent ~36px toolbar row**: a URL search field (magnifier icon, debounced) plus a funnel button with a badge of active facets (Protocol+Status).
  - **A popover** (opened by the funnel button) holds only chip toggles for Protocol (HTTP/gRPC/GraphQL) and Status (2xx/3xx/4xx/5xx/error) plus Clear filters. URL search was pulled out as the most common case.
  - The popover closes on a second click on the button, on Escape and on click-outside.
  - The header became minimal: just the "History" title and a trash icon for Clear all (the filter button left the header).
- About 80% less vertical space: 280px → 36px idle.

## [v0.9.1] — 2026-04-26

### Fixed
- **Replay failed with `CHECK constraint failed: json_valid(auth_data)`** — `request.CreateDraftFromHistory` built a draft without an explicit `AuthData`, leaving the field as an empty string `""`, which is not valid JSON and violated the SQLite CHECK constraint on `requests.auth_data`. A draft is now created with safe defaults for every column under `json_valid(...)`: `AuthData = "{}"`, `GRPCMetadata = map{}`.

### Testing
- Added the unit test `TestCreateDraftFromHistory_PassesJSONCheckConstraints`, pinning that a draft carries correct defaults.
- Added the SQLite integration tests `TestRequestRepo_Create_RejectsEmptyAuthData` (proves the constraint really fires on an empty string) and `TestRequestRepo_Create_AcceptsDraftDefaults` (checks the draft entity passes every json_valid CHECK against the real schema).

## [v0.9.0] — 2026-04-26

### Added
- **History** — a new ActivityBar section, previously a `Coming soon` stub. The sidebar shows the last 200 executions in the current workspace, sorted by `created_at DESC` and grouped by date (Today / Yesterday / This week / Earlier). Entries are denormalised — they carry method, URL, request and response headers and body, status, duration and error.
- **A filter popover** — filter by protocol (HTTP/gRPC/GraphQL), status range (2xx/3xx/4xx/5xx/Error) and a URL substring (case-insensitive). An active filter is marked with a dot on the icon.
- **A history viewer in the main pane** — a read-only view of one entry: Request (Headers, Body), Response (Headers, Body) and Error if there was one. It opens on click.
- **Replay** — a button on the entry card and in the viewer. It creates a temporary draft request (`is_draft=1`), opens it in a new tab and marks it with a `Draft` badge. It can be edited and sent, running as a normal request and adding a new history entry. Closing the tab hard-deletes the draft. Replay comes with a toast.
- **Save as request →** — in a draft tab the RequestEditor shows a banner whose button opens a collection and name dialog; on confirm the draft becomes a permanent request.
- **A History button in the RequestEditor** (UrlBar) — switches the section to History with the filter prefilled by `requestId`, showing only that request's entries. The History sidebar shows a "Filtered by request" chip with a reset button.
- **Clear all** — deletes the whole history of the current workspace through a ConfirmDialog. A single entry goes by the trash icon on hover. Available from the item row and from the sidebar header.
- **Load more** — a footer button (`200 of N records — Load more`) that raises the offset by 200.
- **Cleanup of leftover drafts at app start** — an fx OnStart hook deletes all drafts (`is_draft=1`) on launch, in case the app crashed with a draft tab open.

### Changed
- **`request.Repository.List` now filters `is_draft=0`** — drafts are excluded from ordinary lists, the sidebar tree, search and export. They are visible only through `GetByID`, needed to open a tab, and in tabs through `requestsMap`.
- **The `HistoryRepository` interface grew** — added `GetByID`, `List`, `Count`, `Delete` and `DeleteAll`. It only had `Create` before.
- **Wails: a new `HistoryService`** (5 methods: List, GetByID, Delete, Clear, Replay) and two new methods on `RequestService` (`DeleteDraft`, `PromoteDraft`).
- **`HistoryRepo.Create`** now writes `request_id = NULL` for orphan history, when the original request is already deleted — matching the `ON DELETE SET NULL` FK. It used to write an empty string and break the FK.

### Migration
- Migration `013_request_drafts.sql` adds an `is_draft INTEGER NOT NULL DEFAULT 0` column to `requests` plus a partial index `idx_requests_drafts WHERE is_draft = 1`. Existing rows get `is_draft=0` automatically.

### Known limitations
- Replay does not restore pre/post scripts or auth — `entities.History` stores only the resolved request and response. That is by design, to record what was actually sent. The Replay tooltip says so explicitly.
- StatusKind 2xx-5xx exclude rows where `error_message != ''` — a partial failure (server replied with 200 but the transport failed) is classified only as Error, never as 2xx. The contract is pinned by a test.
- Large response bodies (> 1 MB) are stored in SQLite as they are. That is not a problem at this stage; it will be reworked if it becomes a bottleneck.

## [v0.8.1] — 2026-04-26

### Changed
- **Internal refactor:** `Result[T]` / `ResultError` / `Empty` / `OK` / `Err` / `ErrCode*` moved from `internal/domain/` to `internal/adapters/wails/`. This restores the clean-architecture invariant of a domain without JSON tags — the Wails JSON-RPC envelope lived in the wrong layer. Behaviour and wire format are unchanged: the `data` / `error` / `code` / `message` / `fields` fields are byte-for-byte identical. The frontend and stored data are untouched.

### Testing
- Added contract tests pinning the exact JSON shape of `Result`/`Err`/`Empty` (8 cases with byte equality).
- Added happy and error coverage for every public method of all 9 Wails services (66 methods, 111 tests in a matrix) — a safety net for future signature changes.

## [v0.8.0] — 2026-04-25

### Added
- **A Cookies tab in the response viewer.** A read-only list of cookies for the current request, in two sections:
  - *Sent* — the cookies the client attached to this request through the jar.
  - *Received* — `Set-Cookie` from the response with all its attributes (Domain, Path, Expires, HttpOnly, Secure, SameSite).
- **A Cookie Manager (modal).** A two-panel UI: the domain list with counters on the left, the cookie table for the selected domain on the right. It supports adding a cookie by hand, editing (PUT semantics — all fields), deleting one at a time, clearing a whole domain and clearing the entire jar of a workspace. Triggered by the `Manage cookies →` link in the Cookies tab.

### Changed
- **The cookie jar is now persisted in SQLite and scoped per workspace.** Cookies survive restarts and are isolated between workspaces — switching workspace shows a different set. The jar used to be in-memory and process-wide: lost on restart and shared across workspaces.
- **`HTTPRequester` uses a workspace-scoped jar for every request.** A per-call `workspaceJar` adapter implements `http.CookieJar` on top of the SQLite repository and filters every read and write by `workspace_id`.
- **The response now carries the resolved URL** (`Response.URL`) — needed to match cookies in the tab when the URL uses env variables (`{{gateway_host}}/...`).

### Cookie semantics (RFC 6265)
- **Domain matching:** added a `host_only` flag. A cookie the server sent without a `Domain=` attribute matches the exact host (origin) only, not subdomains. With `Domain=foo.com` it matches the host and any subdomain.
- **Delete-cookie semantics:** a `Set-Cookie` with `Max-Age=0` (or `Max-Age<0`), or with `Expires` in the past, now really removes the existing entry from the jar. It used to be upserted as a session cookie with an empty value. This lets a server clear the session on logout.
- **Path default:** simplified semantics — cookies with no explicit `Path=` get `Path=/`. The RFC asks for directory-default-path; this is accepted as a known simplification.
- **Expiration:** prune-on-read — cookies with `expires_at < now` are left out; session cookies, which have no expiration, are kept until cleared by hand.

### Migration
- In the previous version the cookie jar was in-memory and reset on every start, so there is nothing to migrate. After the upgrade the jar is empty.

## [v0.7.5] — 2026-04-25

### Changed
- **"Copy as cURL" generation moved from the frontend to the backend.** curl used to be assembled in `frontend/src/lib/codegen/curl.ts` from raw request fields and saw only resolved env vars — the pre-script, `inherit` auth from collections, multipart with files and binary bodies were ignored, so the copied command could differ from what `Send` sends. curl now goes through the same pipeline as `Execute`: env vars → pre-script → auth resolver walking the collection hierarchy → applyAuth → automatic `Content-Type` → body encoding (multipart `-F 'k=@/path'` for files, `--data-binary '@/path'` for binary, `-d '<urlencoded>'` for non-file forms). Implemented as a new usecase method `request.BuildCurl` and a Wails endpoint `RequestService.GenerateCurl`. For gRPC and GraphQL the command returns a `ValidationError`, since curl is not supported for them.
- **The pre-script runs in dry-run mode when copying curl.** Header and variable changes the pre-script makes are reflected in the command, but **are not saved to the DB** and do not write a history entry. Copying curl stays a side-effect-free operation.
- **Cookies from the session jar now make it into the curl command.** If a pre-script or an earlier request such as `auth/login` put a `Set-Cookie` into the process-wide jar, the copied curl for the next request to the same domain gets `-b 'name=value; ...'` — matching what `Send` sends. Implemented through a new `request.CookieReader` interface, forward-compatible in that it already takes a `workspaceID` so a future persisted per-workspace jar drops in without breaking call sites.

## [v0.7.4] — 2026-04-25

### Changed
- **A new app icon** — `build/appicon.png` replaced with the final custom design, a gopher courier with a backpack on a dark rounded background. `build/darwin/icons.icns` and `build/windows/icon.ico` regenerated from the 1024×1024 PNG.
- **The Apple Icon Composer pipeline is off** — `build/appicon.icon/` (the Wails SVG template with gradient, shadows and translucency) and `build/darwin/Assets.car` were deleted, and `CFBundleIconName` was removed from `Info.plist` and `Info.dev.plist`. The icon is complete as it is, with its own background and rounding, and running it through Icon Composer would pile on extra effects. macOS now takes the icon from `CFBundleIconFile` → `icons.icns`. The `-iconcomposerinput` and `-macassetdir` flags were removed from `build/Taskfile.yml` (`generate:icons`).

## [v0.7.3] — 2026-04-24

### Added
- **The app version in the window title** — the window title is now `GopherCourier 0.7.3` instead of just `GopherCourier`, so one glance tells you which build is running. The version lives in `internal/constants/app.go` (the `AppVersion` constant plus an `AppTitle()` helper) and is bumped together with the CHANGELOG. `frontend/package.json` is kept on the same version.

## [v0.7.2] — 2026-04-24

### Fixed
- **Sidebar search: a matched folder could not be expanded in any useful way** — if a folder name matched the query but its children did not, expanding it showed nothing and looked broken. A matched folder is now treated as transparent: expanding it shows all nested folders and requests even if they did not match themselves. Items that made it into the tree only as context, not as direct matches, are rendered with `opacity-50` so real matches stand out. The `<mark>` highlight is still applied only to real matches through `useHighlight`.

## [v0.7.1] — 2026-04-24

### Added
- **A Copy button in the response toolbar** — a copy icon to the right of the search field in the body tab, copying the formatted response body through `navigator.clipboard.writeText`. On click the icon turns into a green checkmark for 1.5 seconds and the tooltip switches to `Copied!`. It does not need focus in the body and works the same for JSON, XML, HTML and text; it is not shown for binary. The context menu (Copy Body / Select All) remains as a fallback.

### Fixed
- **Cmd/Ctrl+A and Cmd/Ctrl+C did not work in the response body** — CodeMirror 6 was configured with both `EditorView.editable.of(false)` and `EditorState.readOnly.of(true)`. The first removed `contenteditable` from the content, so native selection and DOM copying did not work; the internal `state.selection` set by the manual Cmd+A handler was not synced with the DOM selection and only the first character reached the clipboard. Only `readOnly` is left — `contenteditable` is preserved and Cmd+A followed by Cmd+C copies natively.
- **Cmd/Ctrl+A copied only the visible part of a long response** — CodeMirror 6 virtualises large documents and renders only the current viewport, so a native Cmd+A inside `contenteditable` selected the visible ~60 lines instead of the whole buffer. Cmd+A is now bound to the CM6 `selectAll` command through `keymap.of(...)`: it sets `state.selection` over the whole document, and the CM6 copy handler writes the full text from `state.doc` to the clipboard rather than the DOM selection. Verified on a 329 KB mock response (2000 users).

## [v0.7.0] — 2026-04-24

### Added
- **Sidebar search** — an inline field in the `Collections` header that filters the tree by collection and request names. The search runs on the Go side through SQLite (UNION ALL over collections and requests, with Unicode-aware filtering in Go so Cyrillic works). Matches are highlighted with `<mark>` and the hierarchy is kept — ancestor folders of matched items expand for the duration of the search without clobbering the user's own expansion state (`userExpanded`). Requests from folders that were never expanded are rendered through an isolated preview store; destructive actions (delete, move, rename) and opening a tab first load the full data through `ensureFullyLoaded`. Esc, the × button or an empty field reset the search and the tree returns to its previous state. A "Showing top 200 results. Refine query" notice appears when the limit is exceeded. Race conditions are guarded by a sequence counter.
- **Cmd/Ctrl+F** — the shortcut focuses the sidebar search field. In CodeMirror editors (URL bar, body, scripts) CodeMirror's own search works as before — `searchKeymap` intercepts `Mod-f` and stops the event from bubbling.

### Changed
- The text label in the `Collections` header is gone — the search field took its place. The Import Postman and New Collection buttons stayed on the left.


### Fixed
- **Auto-commit on blur in add-row forms** — when filling a new row (env variable, query param, header, form-data field, gRPC metadata) the variable is now created when focus leaves the container, not only on Enter. Moving focus inside the add-row (Tab from key to value, clicking `+`) does not trigger creation. Enter, Tab and a click on `+` keep working as before.

## [v0.6.7] — 2026-04-23

### Added
- **Cmd/Ctrl+[ and Cmd/Ctrl+]** — previous and next tab navigation with wrap-around. Complements the existing Cmd+1..9.

## [v0.6.6] — 2026-04-23

### Added
- **An env variable tooltip with drill-through** — hovering `{{var}}` in any editor (URL, body, headers, scripts) shows a compact popover: the variable name, its value, a scope badge ("Environment" / "Undefined") and an action link. The link opens `EnvironmentModal`: for a defined variable it scrolls to the row, flashes it (1.2s, accent colour) and focuses the value field; for an undefined one it prefills the name in the add-row.
- **Cmd/Ctrl+Click on `{{var}}`** — a power-user shortcut that does the same without hovering the tooltip.

### Changed
- Opening `EnvironmentModal` moved from local `ref`s to the `useEnvModalUi` Pinia store, so a vanilla-TS CodeMirror extension can trigger it with context (focus, prefill).

## [v0.6.5] — 2026-04-23

### Fixed
- **Env variables were not highlighted or resolved in collection scripts** — `CollectionEditor.vue` did not pass `resolved-variables` and `secret-keys` to `ScriptEditor`, so the CodeMirror plugin got an empty object and every `{{var}}` showed as undefined (orange) even when the variable was defined in the active environment. Collection scripts now behave exactly like request scripts.

## [v0.6.4] — 2026-04-22

### Added
- **MCP DevTools — new tools and a wider schema**, covering the pain points from the review:
  - `list_workspaces`, `get_workspace`, `create_workspace` — no more digging in SQLite for a workspace UUID.
  - `update_collection`, `move_collection` — editing and reparenting without recreating or raw SQL.
  - `update_request`, `move_request` — editing body, url, method, headers, auth and scripts without delete+create.
  - `send_request` — runs a saved request (HTTP/gRPC/GraphQL) through `request.Execute` with env resolution and history.
- **`create_collection`**: added `parent_id`, `pre_script`, `post_script`, `auth_type` and `auth_data` — nested collections and inherited scripts or auth can be created in one call.
- **`create_request`**: added `protocol`, `headers`, `auth_type`, `auth_data`, `pre_script`, `post_script`, `grpc_service`/`grpc_method`/`grpc_proto_path`/`grpc_metadata`, `graphql_query`/`graphql_variables`/`graphql_schema_path`/`graphql_operation`. Closes the schema drift between the DB and the MCP API.
- **The full object in `create_*`/`update_*`/`move_*` responses** — no separate `get_*` needed for context after a change.

### Changed
- `mcpadapter.NewServer` now takes a `workspace.Usecase` (the `app/mcp.go` FX module and the standalone `cmd/mcp/main.go` were updated).

## [v0.6.3] — 2026-04-22

### Fixed
- **The sidebar did not scroll with a lot of collections** — `flex-1` on `ScrollArea` without `min-h-0` does not let a flex child shrink below its content. Added `min-h-0` in `CollectionTree` and 5 other places with the same pattern (EnvironmentModal, RequestEditor, ScriptEditor, CollectionEditor).
- **No chevron on subfolders that had requests** — `fetchByCollection` was only called on expand, so freshly rendered subfolders did not know about their requests. The fetch now happens on mount; lazy semantics are kept, since CollectionItem only mounts if its parent is open.

### Changed
- **A single toast system** instead of `alert()` — 9 places (collection and environment import/export) now use non-blocking notifications with animation and auto-hide.
- **Feedback when there is no active workspace** — import and export no longer stay silent and show an error through a toast.
- **A generic `RenameDialog`** — one component instead of two identical ones (`RenameCollectionDialog` and `RenameRequestDialog` deleted).
- **A `useConfirmDelete` composable** — a unified delete confirmation flow, used in `CollectionTree`, `EnvironmentModal` and `WorkspaceSwitcher`.
- **`as any` removed from `wails-portability.ts`** — typed DTO constructors from the generated bindings are used instead.

### Added
- **Playwright tests** for sidebar scrolling and the chevron on subfolders.

## [v0.6.2] — 2026-04-20

### Fixed
- **Collection and environment export did not work** — clicking "Export as Postman" did nothing on macOS. WKWebView does not trigger a download for a blob URL through `<a download>`. Export now opens the native Save File dialog (`app.Dialog.SaveFile`) and writes the file on the Go side.
- **HTTP cookies were not kept between requests** — `http.Client` was created without a `CookieJar`, so `Set-Cookie` from responses was not picked up or sent with later requests. That broke auth flows with an access token in the body plus a cookie, and refresh in a cookie only. Added a process-wide in-memory `cookiejar.New(nil)`.

## [v0.6.1] — 2026-03-23

### Added
- **Realtime sync** — the UI updates instantly when data arrives from other clients over the subscribe stream
  - `SyncEngine.EventEmitter` wired to the Wails Event System (`app.Event.Emit`)
  - Frontend subscription to `sync:changed` / `sync:entity_updated` in `App.vue` — automatic `fetchAll` for collections and environments
- **Workspace management through sync** — creating a remote workspace from the UI, auto-sync on connect
  - A `RemoteWorkspaceID` field in the entity, the DTO and the frontend type
  - `CreateRemoteWorkspace` — creates the workspace on the server and links it locally
  - `syncRemoteWorkspaces` — pulls remote workspaces automatically on connect and startup
  - Cloud and Monitor icons in WorkspaceSwitcher to tell local and synced apart
  - A "Sync to server" checkbox when creating a workspace
- **MCP DevTools server** — an HTTP SSE server for managing data over the Model Context Protocol
  - 21 tools: CRUD for collections, requests, environments and variables, plus sync operations
  - A standalone CLI (`cmd/mcp`) for testing without Wails
  - `GOPHERCOURIER_DATA_DIR` support for running several clients on one machine
- **Auto-reconnect at startup** — `ResumeOnStartup` restores the sync connection from stored credentials
  - A SQLite fallback for the refresh token when the OS keychain is unavailable (migration 011)

### Fixed
- **Nil pointer panic on soft-delete sync** — fixed the Go nil-interface pitfall in `readEntity` (a typed nil in `any` is not `nil`)
- **Soft delete did not sync** — `buildDeleteProto` builds a minimal proto for deleted rows that `GetByID` cannot find because of the `is_delete = 0` filter
- **Sync decorators did not trigger a push** — added `NotifyWrite` calls after a successful enqueue into sync_queue
- **SyncConnectModal** — simplified UI, manual workspace linking removed since it is automatic now

## [v0.6.0] — 2026-03-22

### Added
- **Sync client integration** — integration with the gRPC sync server for syncing data between devices
  - Repository decorator plus transactional outbox — every write lands in sync_queue inside the same SQLite TX
  - SyncEngine — a state machine with a push/pull/subscribe lifecycle, one goroutine per workspace
  - SyncAuthManager — JWT auth with the refresh token in the OS keychain (go-keyring), proactive refresh through singleflight
  - gRPC client — a wrapper over the auth, sync and workspace services with a proto mapper (domain ↔ SyncEntity)
  - 4 SyncedRepo decorators (collection, request, environment, variable) intercepting Create/Update/Delete
  - Context-based TX (WithTx, DBTXFromContext) — every existing repository became TX-aware
  - FX wiring — named deps to separate inner and decorated repositories
  - Frontend: SyncStatusIndicator (the cloud in the ActivityBar) and SyncConnectModal (Login/Register plus workspace linking)
  - Migration 009: the sync_config and sync_queue tables, and the is_synced, remote_workspace_id and last_sync_seq columns

## [v0.5.1] — 2026-03-22

### Added
- **GraphQL autocomplete** — completion of fields, types and arguments in the query editor based on the loaded schema (`cm6-graphql` + `graphql-js`)
  - Syntax highlighting for GraphQL (keywords, types, arguments)
  - Linting — invalid fields and queries are underlined
  - Cmd/Ctrl+Click on a field jumps to the Schema tab at that type's docs
  - An autocomplete popup styled for the dark and light themes
- **A Browse/SDL Schema tab** — an interactive type browser with search instead of a separate docs panel
  - An operation detail view (arguments and return type) when an operation is selected
  - An "All operations" reset button in the operation selector
- **Search in a GraphQL response** — the same search as in HTTP
- **An interactive schema in a detached window** — "Open in Window" shows the Browse view instead of raw SDL

### Fixed
- **The GQL badge in the sidebar** — GraphQL requests show "GQL" (pink `#E535AB`) instead of "POST"
- **One GraphQL colour** — the URL bar, the sidebar and the new-request dialog use the same colour
- **Saving GraphQL fields** — `wails-request.ts` did not pass `graphqlQuery`, `graphqlVariables`, `graphqlSchemaPath` or `graphqlOperation` on save or edit, so the data was lost on Cmd+S
- **The variables panel** — its content disappeared when collapsed and expanded again
- **The new-request dialog** — the GraphQL button had a different style and no pointer cursor
- **URL bar selection with variables** — selecting text in a URL containing `{{var}}` works correctly now

## [v0.5.0] — 2026-03-15

### Added
- **GraphQL requester** — GraphQL support for queries and mutations
  - Introspection plus importing `.graphql` schema files
  - An operation selector with generated example queries (depth 3)
  - A split query editor with a docs panel and variables
  - A dedicated response viewer that separates Data and Errors
  - A schema viewer with filtering by operation
  - An Auth tab, shared with HTTP
  - Pre/post script support (`pm.request.graphqlVariables`, `pm.request.graphqlOperation`)
  - Detachable window support (the schema viewer with a language param)
  - Postman import/export compatibility (body.mode="graphql")

## [v0.4.0] — 2026-03-15

### Added
- **Detachable windows** — a system for opening tabs in separate native windows (Wails v3)
- **Detach request** — right-click a tab → "Open in Window" to open an HTTP or gRPC request in its own window
- **Schema viewer window** — an "Open in Window" button in GRPCSchemaViewer to view a proto schema in a separate window
- **Cross-window sync** — Wails Events keep environments, collections and the workspace in step between windows
- **Double-open prevention** — clicking a detached request in the sidebar focuses the existing window
- **Window prefs** — window size and position saved automatically to `~/.gophercourier/window-prefs.json`
- **Auto-save** — saved automatically when any window closes
- **Cascade close** — closing the main window closes every child window

## [v0.3.0] — 2026-03-14

### Added
- **gRPC support** — unary calls, server reflection, importing .proto files (a file or a directory)
- **Service and method picker** — a combined dropdown with search and keyboard navigation
- **A Schema tab** — proto definitions with syntax highlighting (CodeMirror plus protobuf mode)
- **Generate Example** — generates an example JSON body from the proto schema
- **A metadata editor** — gRPC metadata through KeyValueEditor
- **Metadata inheritance** — collections pass default metadata to nested gRPC requests
- **Pre/post scripts for gRPC** — `pm.request.metadata`, `pm.request.grpcMethod`, `pm.request.protocol`, `pm.response.statusText`
- **A protocol badge** — HTTP (blue) and gRPC (purple) in the sidebar
- **A protocol choice** — an HTTP / gRPC toggle when creating a request

## [v0.2.5] — 2026-03-14

### Added
- **Rename request** — renaming requests from the sidebar context menu

## [v0.2.4] — 2026-03-14

### Fixed
- **Copy as cURL** — a duplicated Content-Type header when it was already set by hand in Headers

## [v0.2.3] — 2026-03-14

### Added
- **Environment variables in auth fields** — `{{variable}}` highlighting, autocomplete and a hover tooltip in the Bearer Token, Basic Auth and API Key fields
- **A VariableInput component** — a reusable input/textarea with environment variable support (overlay highlighting, autocomplete, tooltip)

## [v0.2.2] — 2026-03-14

### Fixed
- **Move to... for single items** — "Move to..." is now in the context menu of an individual collection or request, not only in a multi-select

## [v0.2.1] — 2026-03-14

### Fixed
- **Secret variables in the body** — the tooltip and autocomplete in CodeMirror now mask secret env variables (`••••••••`)
- **False JSON errors in the body** — `{{variable}}` patterns are no longer marked red as syntax errors
- **An empty body** — an empty JSON editor no longer shows a validation error

## [v0.2] — 2026-03-14

### Added
- **Workspaces** — isolated containers for collections, environments and history
  - Workspace CRUD (create, rename, delete)
  - A workspace switcher in the sidebar with a dropdown menu
  - Another workspace is activated automatically when the active one is deleted
  - An automatic switch to a newly created workspace
  - The last workspace cannot be deleted
  - Data isolation: collections, environments and history belong to a workspace
  - 20 new Go tests (6 repo, 14 usecase)
- **A "..." button in the sidebar** — an overflow menu for collections and requests on hover, mirroring right-click
- **The native context menu is blocked** — the system menu (Look Up, Translate) removed in favour of custom ones

### Fixed
- Empty state copy: removed the incorrect Cmd+N hint

## [v0.1] — 2026-03-11

### Added
- **Collection detail backend** — persistence for collection descriptions and auth
  - A collection description (Markdown) with a preview
  - Collection-level auth (Basic, Bearer, API Key)
  - Auth inheritance: requests set to `inherit` take auth from the parent collection
  - AuthResolver walks the collection hierarchy, up to 50 levels
  - Postman import and export of collection descriptions and auth
  - Unified dirty tracking for description, auth and scripts in CollectionEditor
  - 14 new Go tests (AuthResolver, Create/Edit validation, Postman import/export)
- **Saving binary responses** — binary HTTP responses detected by Content-Type, with an offer to save the file
  - Automatic binary content detection (image/*, audio/*, video/*, application/pdf, zip and others)
  - Raw bytes written to a temp file instead of being converted to a string
  - A "Save to file" button with the native save dialog
  - The filename parsed from Content-Disposition, including RFC 5987 encoding
  - Path traversal protection on save
  - 45 new Go tests
- **Header enabled/disabled persistence** — a header's enabled state is now stored in the DB
  - A `HeaderItem` struct with key, value and enabled instead of `map[string][]string`
  - Disabled headers are not sent on execute and do not reach cURL
  - Backward compatible: the old JSON format is migrated on read
  - Postman import and export keep the disabled state
- **File fields in form data** — a Text/File type for form-data fields
  - A field type switch in FormEditor with a Browse button for files
  - Automatic multipart/form-data encoding when file fields are present
  - Postman import now includes file fields, which used to be skipped
- **Multi-select in the sidebar** — Cmd+Click to add to the selection, Shift+Click for a range
- **Bulk delete** — Delete and Backspace remove the selected items after a confirmation dialog
- **Move to...** — moves selected collections and requests to another folder through a tree dialog
- **Collection move** — backend: moving collections with cycle validation (5 tests)
- **Request move** — backend: moving requests between collections (3 tests)
- **Pre/post-request scripts** in JavaScript (Goja engine)
  - Partial Postman compatibility: pm.environment, pm.request, pm.response, pm.test, console.log
  - Collection-level scripts inherited down the hierarchy (Request → Collection → Parent)
  - Pre and post scripts inherit independently
  - A "Scripts" tab in the request editor with a Pre-request / Post-response switch
  - An "Edit Scripts" context menu item for collections, with a modal editor
  - A "Tests" tab in the response viewer with test results and console output
  - Variables set by scripts are saved to the active environment automatically
  - JavaScript syntax highlighting in CodeMirror 6
- **Environment variables** — environments (Dev/Staging/Prod) with variables
  - CRUD for environments and variables (Go usecase, SQLite, 28 new tests)
  - Secret variables (`is_secret`) — masked in the UI, never synced to the server
  - `{{variable}}` substitution in URL, headers, body and auth on Execute
  - An environment selector dropdown in the URL bar — one click to switch
  - A management modal (split layout) — the env list plus a variable editor
  - `{{var}}` highlighting in the URL bar — purple for resolved, orange for undefined
  - A hover tooltip showing the resolved value, with secrets masked
  - An Environments icon in the Activity Bar that opens the modal
  - A mock service with pre-populated data (base_url, auth_token, api_version)
- **KeyValueEditor** — a shared component extracted from HeadersEditor, ParamsEditor and FormEditor (-158 lines)
- **parseTime** — SQLite datetime format support in the repositories
- **Params editor** — query parameters in a table, two-way synced with the URL
- **Body editor** — a full request body editor (JSON/XML/Raw in CodeMirror, form URL encoded, a binary file picker)
- **Auth** — Basic Auth, Bearer Token and API Key (header or query)
- **Auth in Execute** — auth applied automatically when sending, mapped into headers or the URL
- **Auto Content-Type** — Content-Type set automatically when a body type is picked (JSON → application/json and so on)
- **Form encoding** — form fields serialised to application/x-www-form-urlencoded on Execute
- **Binary body** — sending files, with an existence check on Execute
- **Pretty print** — a JSON/XML formatting button in the body editor

### Added (earlier)
- **HTTP requester** — sending HTTP requests with the Send button or Cmd+Enter
- **Response viewer** — responses with syntax highlighting (CodeMirror 6, One Dark Pro theme)
- **Response headers** — a tab with a table of response headers
- **History** — every executed request saved to history automatically (SQLite)
- **JSON auto-prettify** — JSON responses formatted automatically
- **Status badge** — a coloured badge for status codes (2xx green, 4xx orange, 5xx red)
- **Error state** — structured error display with hints (connection refused, timeout, DNS)
- **Cmd+1..9** — switching between open tabs by shortcut
- **Custom context menu** — a custom context menu for the response body (Copy Body, Select All)
- **Mock service** — URL-based simulation of every state for testing (200, 404, 500, error, slow)

### Fixed
- Wails v3 environment detection (window._wails instead of __wails__)
- Closing a tab when its request is deleted
- Hover in the sidebar context menu (visible contrast in the dark theme)
- Pointer cursor on the Send button and context menu items
