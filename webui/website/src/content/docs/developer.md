---
date: '2026-08-22T00:00:00+02:00'
draft: false
title: 'Developer Documentation'
description: 'Understand the source tree, test Hister, extend extractors, and inspect its HTTP and MCP interfaces.'
---

This page is for people who contribute changes, add extractors, or integrate
directly with Hister HTTP and MCP interfaces.

The primary source repository is on
[GitHub](https://github.com/asciimoo/hister). It is also mirrored on
[Codeberg](https://codeberg.org/asciimoo/hister).

## Contributing to Hister

Read the complete
[contribution guide](https://github.com/asciimoo/hister/blob/main/CONTRIBUTING.md)
before starting work. Keep each pull request focused on one concern, explain
why the change is needed, and make sure relevant tests and checks pass. Open an
issue before implementing a large architectural change, broad refactor, or
development tooling change so the approach can be discussed first.

Ask questions when requirements or existing behavior are unclear. Review
feedback concerns the work and the shared goal of improving Hister. A useful
contribution should provide more value to the project than the time required to
review and maintain it.

### Project Standards

**Go code.** Follow the GolangCI Lint v2 configuration in `.golangci.toml`.
Formatting uses `goimports` and `gofumpt`. Run focused package tests during
development and `go test ./...` before submitting a broad change.

**Frontend code.** Prettier uses single quotes, trailing commas, a print width
of 100, and two space indentation. Prefer standard Tailwind scale classes.
Check the shared component library before creating a new component.

**Change scope.** Preserve compatibility unless the change explicitly requires
otherwise. Update tests and user documentation whenever behavior, command
flags, configuration, query syntax, or public interfaces change. Keep release
notes for the release process.

**Licensing.** Contributions are accepted under the project
[AGPLv3 license](https://github.com/asciimoo/hister/blob/main/LICENSE).

**AI use.** The contribution guide requires disclosure of every AI tool and the
extent of its use. The human contributor must understand the complete change
and remain its main author. Issue descriptions, pull request descriptions, and
discussion comments must be written by a human. AI tools must not be used for
good first issues. Coding agents should also follow the repository
[agent reference](https://github.com/asciimoo/hister/blob/main/AGENTS.md).

### Useful Development Links

| Resource                                                                                                           | Purpose                                                        |
| ------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------- |
| [Contribution guide](https://github.com/asciimoo/hister/blob/main/CONTRIBUTING.md)                                 | Submission standards, code style, and AI policy                |
| [`manage.sh`](https://github.com/asciimoo/hister/blob/main/manage.sh)                                              | Common dependency, test, build, and artifact commands          |
| [Go module](https://github.com/asciimoo/hister/blob/main/go.mod)                                                   | Go version, module path, and backend dependencies              |
| [Go lint configuration](https://github.com/asciimoo/hister/blob/main/.golangci.toml)                               | Enabled Go formatters and linters                              |
| [Frontend workspace](https://github.com/asciimoo/hister/blob/main/package.json)                                    | npm workspaces, formatting, and frontend checks                |
| [Frontend notes](https://github.com/asciimoo/hister/blob/main/webui/README.md)                                     | Web project structure, builds, previews, and shared components |
| [`webui/build.sh`](https://github.com/asciimoo/hister/blob/main/webui/build.sh)                                    | Embedded web application build process                         |
| [API route registry](https://github.com/asciimoo/hister/blob/main/server/api.go)                                   | HTTP endpoint declarations and generated API documentation     |
| [Extractor template](https://github.com/asciimoo/hister/tree/main/server/extractor/extractors/_extractor_template) | Starting point for a new extractor                             |
| [Issue tracker](https://github.com/asciimoo/hister/issues)                                                         | Bug reports, proposals, and feature discussions                |
| [Security policy](https://github.com/asciimoo/hister/blob/main/SECURITY.md)                                        | Private security reporting guidance                            |

## Source Layout

| Path                | Purpose                                                                 |
| ------------------- | ----------------------------------------------------------------------- |
| `hister.go`         | Program entry point                                                     |
| `cmd/`              | Command implementations and the terminal interface                      |
| `client/`           | HTTP client used by commands                                            |
| `config/`           | Configuration loading and validation                                    |
| `files/`            | Watched directory matching and file ownership                           |
| `server/`           | HTTP server, models, indexing, crawling, extraction, and vector storage |
| `webui/app/`        | Web application embedded in the Go binary                               |
| `webui/components/` | Shared Svelte components                                                |
| `webui/ext/`        | Browser extension                                                       |
| `webui/website/`    | Public website and documentation                                        |
| `scripts/`          | Dataset utilities                                                       |
| `contrib/`          | Service integration files                                               |

The Go module is `github.com/asciimoo/hister`. The root npm workspace includes
the web application, website, shared components, and browser extension.

## Development Commands

Install the frontend dependencies once before running frontend commands:

```bash
./manage.sh install_js_deps
```

Use these commands for common work:

| Goal                                  | Command                            |
| ------------------------------------- | ---------------------------------- |
| Build the Go program                  | `go build`                         |
| Run Go tests                          | `go test ./...`                    |
| Rebuild embedded frontend assets      | `go generate`                      |
| Build the complete Hister binary      | `./manage.sh build`                |
| Build the web application             | `npm run build -w @hister/app`     |
| Build this documentation website      | `npm run build -w @hister/website` |
| Build the browser extension           | `npm run build -w @hister/ext`     |
| Run the app development servers       | `npm run serve:app`                |
| Format and lint Go code               | `golangci-lint run --fix ./...`    |
| Check frontend formatting and linting | `npm run check`                    |
| Apply frontend formatting             | `npm run format`                   |

`go generate` runs `webui/build.sh`. That script builds `@hister/app`, replaces
`server/static/app`, and copies the new frontend build into the embedded asset
directory. Run it before building the Go binary whenever embedded frontend code
changes.

`npm run serve:app` starts the web application with hot reload and the Go
server with automatic rebuilds. It installs `air` when that tool is not already
available.

Use focused tests while developing. Run the complete Go suite before handing
off a broad or high risk change.

## HTTP API Development

The running server publishes its generated API documentation at `/api`. API
routes are declared as `Endpoint` values in `server/api.go`. Add new API routes
there so routing, authorization, CSRF handling, and API documentation remain in
sync.

Authenticated clients can send either of these headers:

```http
Authorization: Bearer <access-token>
```

```http
X-Access-Token: <access-token>
```

### Document Versions

```http
GET /api/versions?url=<url>
```

The endpoint returns stored version differences for the given URL, with the
newest record first. Each entry contains `id`, `created_at`, `url`, `user_id`,
`html_diff`, and `text_diff`. The difference values use
[diff match patch](https://github.com/google/diff-match-patch) patch strings.

### Current Profile

`GET /api/profile` returns information about the currently authenticated user:

```json
{
  "user_id": 1,
  "username": "alice",
  "is_admin": true
}
```

## Extractor Development

Extractors are components that enrich metadata, produce searchable content,
and render document previews. Indexing first runs every matching enricher in
registration order. It then tries matching content extractors in registration
order until one succeeds. Preview selection considers only extractors that
declare the preview capability.

### Extractor Decisions

`Extract` and `Preview` return explicit result values. Each result records one
of three decisions:

| Decision | Meaning                                                                                              |
| -------- | ---------------------------------------------------------------------------------------------------- |
| Success  | The extractor handled the document successfully.                                                     |
| Fallback | The extractor was inconclusive, so the next matching extractor should be tried.                      |
| Abort    | A fatal error occurred. The chain stops immediately and returns the error without trying a fallback. |

Use `Extracted`, `ExtractFallback`, or `AbortExtraction` for extraction
results. Use `Previewed`, `PreviewFallback`, or `AbortPreview` for preview
results. The result fields are private, so an implementation cannot return a
contradictory decision and error combination. The zero value is invalid.

When a caller names a preview extractor explicitly, that extractor must be
enabled, preview capable, and matched to the document. Selection starts there.
If it returns `PreviewFallback`, later matching preview extractors may still
provide the response.

If no extractor succeeds, `ErrNoExtractor` is returned.

### Extractor Interface

A custom extractor can depend on the focused
[`server/extractor/sdk`](https://github.com/asciimoo/hister/tree/main/server/extractor/sdk)
package instead of importing Hister configuration, document, and result
packages separately. It must implement this interface:

```go
type Extractor interface {
    // Name returns a human readable identifier used in logs and config.
    Name() string

    // Description returns a short human readable summary for clients.
    Description() string

    // Capabilities declares the extractor phases this implementation joins.
    Capabilities() sdk.Capabilities

    // Match reports whether this extractor applies to the given document.
    // Extract and Preview are only called when Match returns true.
    Match(*sdk.Document) bool

    // Extract rewrites the document before it is added to the index.
    // Return an explicit success, fallback, or abort result.
    Extract(*sdk.Document) sdk.ExtractResult

    // Preview returns a rendered representation suitable for display.
    // Return an explicit success, fallback, or abort result.
    Preview(*sdk.Document) sdk.PreviewResult

    // GetConfig returns the extractor's current configuration.
    // It must return sensible defaults before SetConfig is called.
    GetConfig() *sdk.Config

    // SetConfig applies user supplied configuration on top of defaults.
    // Return an error for any unrecognised option key.
    SetConfig(*sdk.Config) error
}
```

#### `Capabilities`

Capabilities keep metadata enrichment, searchable content extraction, and
preview rendering independent:

```go
type Capabilities struct {
    Enrich  bool
    Extract bool
    Preview bool
}
```

An enricher annotates every matching document and does not compete to select
the searchable body. A content extractor can populate the title and text. A
preview extractor can render the stored document. Most specialist extractors
declare both content and preview capabilities.

#### Result Types

`sdk.ExtractResult` and `sdk.PreviewResult` describe the outcome of each phase.
Construct them with these helpers:

```go
return sdk.Extracted()
return sdk.ExtractFallback(err)
return sdk.AbortExtraction(err)

return sdk.Previewed(sdk.PreviewResponse{Content: html})
return sdk.PreviewFallback(err)
return sdk.AbortPreview(err)
```

Fallback errors are optional diagnostics. Abort helpers always carry an error.
Passing `nil` to an abort helper produces a descriptive error rather than an
ambiguous result.

#### `Document`

`sdk.Document` aliases the complete
[`document.Document`](https://github.com/asciimoo/hister/blob/main/server/document/document.go)
type. It is passed to `Match`, `Extract`, and `Preview`.

#### `PreviewResponse`

`sdk.PreviewResponse` carries the output of `Preview`:

```go
type PreviewResponse struct {
    Content  string // HTML or plain text to render
    Template string // optional custom frontend template name, leave blank for default
}
```

### Register an Extractor

Built in extractors are constructed by `DefaultExtractors` in
[`server/extractor/registry.go`](https://github.com/asciimoo/hister/blob/main/server/extractor/registry.go).
Add an instance there before the generic fallbacks so it takes priority for
the pages it targets.

Applications can also create an isolated chain or extend a fresh default one:

```go
registry := extractor.NewDefaultRegistry()
if err := registry.RegisterBefore("Readability", &MyExtractor{}); err != nil {
    return err
}
if err := registry.Init(configuredExtractors); err != nil {
    return err
}
```

The package level extraction functions remain available and use
`extractor.DefaultRegistry()`.

### Write a New Extractor

A ready to use starting point lives at
[`server/extractor/extractors/_extractor_template/extractor.go`](https://github.com/asciimoo/hister/blob/main/server/extractor/extractors/_extractor_template/extractor.go).
The directory begins with `_` so the Go toolchain ignores it during normal
builds, but the file itself is valid and fully commented Go.

1. Copy `server/extractor/extractors/_extractor_template/` to
   `server/extractor/extractors/<myname>/` and remove the leading `_`.
2. Change the `package` declaration to match the new directory name.
3. Rename `TemplateExtractor` to something descriptive, such as
   `HackerNewsExtractor`.
4. Update `matchURLPrefix` and the `Match` function for the target site.
5. Update `Capabilities` to declare the phases the extractor supports.
6. Implement `Extract` to populate `d.Title`, `d.Text`, and optional metadata.
7. Implement `Preview` to return sanitized HTML, or return `PreviewFallback` to
   reuse the generic readability preview.
8. Add an import and an extractor entry to `DefaultExtractors` in
   `server/extractor/registry.go`, before the readability extractor.

### Extractor Configuration Support

Embed `sdk.ConfigSupport` to provide `GetConfig` and `SetConfig`. Its zero value
enables the extractor and rejects every option key:

```go
type MyExtractor struct {
    sdk.ConfigSupport
}
```

Use `sdk.NewConfigSupport` when an extractor has custom defaults. Keys present in
the default options map are accepted automatically:

```go
func NewMyExtractor() *MyExtractor {
    return &MyExtractor{
        ConfigSupport: sdk.NewConfigSupport(sdk.Config{
            Enable: true,
            Options: map[string]any{
                "timeout": 10,
            },
        }),
    }
}
```

Additional accepted keys without defaults can be passed after the default
configuration. An extractor can implement its own configuration methods when
applying configuration requires extra work.

Config merging from defaults and user values is performed automatically by
`Registry.Init` before `SetConfig` is called. `SetConfig` always receives the
fully resolved configuration.

### Extractor Guidelines

**Avoid additional HTTP requests.** Work with the HTML and metadata already
available in the `Document` struct wherever possible. Making extra requests
inside an extractor adds latency, increases network traffic, and can fail
silently in offline or restricted environments. Outbound requests also expose
the user IP address and browsing activity to external servers. Make such a
request only when there is no reasonable alternative.

**Avoid embedding third party content.** Strip or discard remote images,
videos, iframes, and other externally hosted media before returning content
from `Extract` or `Preview` wherever possible. Embedded content causes the
browser to contact an external server whenever a preview is opened, which can
expose the user IP address without their knowledge. When multimedia is
essential, prefer a placeholder button that loads it only after explicit user
action.

**Use custom preview templates when they add value.** If extracted content has
a clear structure, return a template name in `PreviewResponse` and build a
dedicated Svelte template for it. Code documentation, question and answer
threads, and recipes often benefit from a tailored layout.

### Test Extractors Against Live Websites

Live extractor cases are declared in
[`server/extractor/live_cases.yaml`](https://github.com/asciimoo/hister/blob/main/server/extractor/live_cases.yaml).
Each case identifies a public URL, crawler backend, extractor, and a set of
stable content or metadata expectations. Prefer semantic invariants such as the
page type, a durable title fragment, and minimum content length over exact page
snapshots.

Run every live case with:

```bash
./manage.sh run_extractor_tests
```

Pass a case name fragment to run a subset:

```bash
./manage.sh run_extractor_tests discourse
```

The runner fetches each page through the configured Hister crawler backend and
respects its robots rules. It checks the selected extractor directly, checks the
complete extractor chain unless `run_chain` is false, and checks previews when
preview expectations are present. Identical fetches are reused within a test
run.

The live suite uses the `live` Go build tag and is not part of a normal
`go test ./...` run. The manifest structure is still validated during normal
tests, so invalid fields and unknown extractor names fail without making
network requests. Validation also requires at least one positive live case for
every registered extractor.

When a live case fails, the fetched HTML, extracted documents, preview, and a
summary are saved under `/tmp/hister-live-extractors`. Set
`HISTER_LIVE_ARTIFACT_DIR` to use another directory. These artifacts help
distinguish a website structure change from a transient fetch failure.

## MCP Protocol Testing

The MCP endpoint uses Streamable HTTP at `POST /mcp`. Every interaction is a
`POST` request with a JSON RPC 2.0 body. The server responds with a JSON object.
The authorization header is required only when authentication is enabled in
the Hister configuration.

### Initialize

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}'
```

### List Tools

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
```

### Search

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search","arguments":{"query":"python async","limit":5}}}'
```

Include full text and language fields when testing optional result data:

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"search","arguments":{"query":"python async","limit":5,"fields":["text","language"]}}}'
```

### Preview

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -d '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_preview","arguments":{"url":"https://example.com/article"}}}'
```

### Indexed History

```bash
curl -s -X POST http://127.0.0.1:4433/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-access-token>" \
  -d '{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"get_history","arguments":{"mode":"indexed","limit":10}}}'
```

### Supported Methods

The endpoint implements MCP specification version `2025-06-18` with these
methods:

| Method                      | Description                                                 |
| --------------------------- | ----------------------------------------------------------- |
| `initialize`                | Capability negotiation, required before any other call      |
| `ping`                      | Liveness check                                              |
| `tools/list`                | Returns the list of available tools and their input schemas |
| `tools/call`                | Executes a tool by name with the provided arguments         |
| `notifications/initialized` | Acknowledged with 202 when sent as a notification           |
| `notifications/cancelled`   | Acknowledged with 202 when sent as a notification           |

Every tool advertises an `outputSchema`. Tool call results include the required
`content` field and a `structuredContent` object conforming to that schema.
Clients should use `structuredContent` and enforce the `untrusted_content`
trust markers when placing results into model context.

## Documentation Maintenance

Add documentation pages under `webui/website/src/content/docs` and register
their slugs in `webui/website/src/lib/docs-structure.ts`. The docs search index,
page routes, previous and next links, and sitemap are generated from this
content during the website build.

When changing query syntax, update query builder tests and the query language
guide. When changing configuration, update the generated configuration
reference data, examples, and related operator guidance. When adding an API
route, declare it in `server/api.go` so the built in API documentation remains
complete.
