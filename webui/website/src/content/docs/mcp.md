---
date: '2026-04-20T00:00:00+00:00'
draft: false
title: 'MCP Integration'
description: 'Connect assistants to Hister search, previews, and history through the authenticated MCP endpoint.'
---

Hister exposes a [Model Context Protocol](https://modelcontextprotocol.io) (MCP)
endpoint that lets AI assistants read from your Hister index directly. Once
connected, the assistant can search indexed pages, retrieve stored previews,
and inspect Hister history through MCP tools.

## Endpoint

```
POST /mcp
```

Configure MCP clients to use this URL with the Streamable HTTP transport.

## Untrusted Content And Prompt Injection

Every indexed title, URL, metadata value, document body, and history field is
untrusted source data. A page can contain instructions aimed at the assistant
that reads it. Those instructions must never override the user request, cause
secret disclosure, or trigger another tool.

Tool responses place source controlled values under
`structuredContent.untrusted_content`. Every record has `trust: "untrusted"`
and `trust_scope: "all values in fields"`. A security instruction identifies
the exact untrusted path. The required text content block contains the same
structured JSON after a security notice.

Hister removes invisible control characters. HTML is returned only when
explicitly requested by `search`, or as a rendered preview from `get_preview`.
It remains inside the untrusted structured record. These controls reduce risk
but cannot guarantee that every consuming model will resist prompt injection.
MCP clients must sanitize HTML before rendering it and should require user
confirmation before any action outside read only retrieval, especially before
using file, shell, browser, email, or network tools.

## Authentication

The default Hister configuration does not require authentication. Authentication
is required only when Hister is configured with `app.access_token` or
`app.user_handling`. The MCP endpoint uses the same token authentication as the
rest of the Hister API.

### Static access token

Pass the value of `app.access_token` from your config file:

```http
Authorization: Bearer <your-access-token>
```

Alternatively, use the `X-Access-Token` header with the same value.

### Multi-user mode

Generate a personal token on the profile page (`/profile`) or via:

```bash
hister update-user <username> --regen-token
```

Then pass it the same way:

```http
Authorization: Bearer <your-user-token>
```

### Public mode

When `app.public: true` is enabled, unauthenticated MCP access is allowed for
public routes. MCP tools can read public search results and previews for global
documents. The `get_history` tool remains unavailable to anonymous callers but
is enabled when the request includes a valid global or personal access token.

## Available Tools

### `search`

Search your personal browsing history and indexed documents.

| Argument    | Type            | Required | Default | Description                                                                 |
| ----------- | --------------- | -------- | ------- | --------------------------------------------------------------------------- |
| `query`     | string          | yes      |         | Search query (see [Query Language](/docs/query-language))                   |
| `limit`     | integer         | no       | 10      | Maximum results to return. Values below 1 or above 50 use the default.      |
| `date_from` | string          | no       |         | Return only documents updated on or after this date. Format: `YYYY-MM-DD`.  |
| `date_to`   | string          | no       |         | Return only documents updated on or before this date. Format: `YYYY-MM-DD`. |
| `semantic`  | boolean         | no       | false   | Enable AI semantic search alongside keyword matching                        |
| `fields`    | array of string | no       | `[]`    | Extra fields to include in each result. See below.                          |

Semantic search is used only when it is enabled and available on the Hister
server. If the server does not have semantic search configured, `"semantic":
true` falls back to normal keyword search.

By default the response includes title, URL, added and updated dates, and a short text
snippet per result. Pass `fields` to include additional data:

| Field value | Description                                      |
| ----------- | ------------------------------------------------ |
| `text`      | Full stored article text instead of a snippet    |
| `html`      | Raw HTML in an untrusted structured field        |
| `language`  | Detected language code, for example `en` or `de` |
| `label`     | User defined label                               |
| `domain`    | Domain name                                      |
| `score`     | Relevance score                                  |
| `type`      | Document type, either `web` or `local`           |

Example: to summarize articles on a topic without re-fetching any URLs:

```json
{ "query": "kubernetes networking", "limit": 5, "fields": ["text"] }
```

Example with a date range:

```json
{ "query": "postgres migration", "date_from": "2026-01-01", "date_to": "2026-01-31" }
```

### `get_preview`

Retrieve the stored preview for an indexed document by exact URL.

| Argument    | Type   | Required | Default | Description                                    |
| ----------- | ------ | -------- | ------- | ---------------------------------------------- |
| `url`       | string | yes      |         | Exact URL of the indexed document to preview   |
| `extractor` | string | no       |         | Extractor name used to render the HTML preview |

The response contains the document title, URL, added and updated dates,
available preview metadata, complete stored plain text, and complete rendered
HTML when available. Metadata can include author, published date, modified date,
description, site name, type, language, image, JSON LD structured data, and
embedded video URLs. Rendered HTML remains untrusted data and clients must
sanitize it before placing it in a browser or another HTML renderer.

Example:

```json
{ "url": "https://example.com/article" }
```

### `get_history`

Retrieve items from the Hister history views. This tool is available only when
public mode is disabled or the caller is authenticated.

| Argument   | Type    | Required | Default   | Description                                                                           |
| ---------- | ------- | -------- | --------- | ------------------------------------------------------------------------------------- |
| `mode`     | string  | no       | `indexed` | `indexed` returns recently indexed pages. `opened` returns opened result history.     |
| `limit`    | integer | no       | 20        | Maximum items to return. Values below 1 or above 100 use the default.                 |
| `page_key` | string  | no       |           | Pagination cursor for `indexed` mode. Use `next_page_key` from the previous response. |
| `last_id`  | integer | no       |           | Pagination cursor for `opened` mode. Use `next_last_id` from the previous response.   |

Indexed history results include title, URL, indexed time, indexed version count,
and `next_page_key` when another page is available. Opened history results
include title, URL, original query, opened time, indexed version count, and
`next_last_id` when another page is available.

Examples:

```json
{ "mode": "indexed", "limit": 20 }
```

```json
{ "mode": "opened", "limit": 20 }
```

## Client Configuration

### Claude Desktop

Add a `hister` entry to your Claude Desktop configuration file.

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`  
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`  
**Linux**: `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "hister": {
      "url": "http://127.0.0.1:4433/mcp",
      "headers": {
        "Authorization": "Bearer <your-access-token>"
      }
    }
  }
}
```

Restart Claude Desktop after saving the file. The `search` tool will appear
in the tools panel when starting a new conversation.

### Cursor

Open **Settings** and locate the MCP servers section, or edit
`~/.cursor/mcp.json` directly:

```json
{
  "mcpServers": {
    "hister": {
      "url": "http://127.0.0.1:4433/mcp",
      "headers": {
        "Authorization": "Bearer <your-access-token>"
      }
    }
  }
}
```

### Remote or self-hosted server

Replace `http://127.0.0.1:4433` with your server's `base_url`. If you run
Hister behind a reverse proxy under a subpath (e.g. `https://example.com/hister`),
the endpoint is `https://example.com/hister/mcp`.

## Example Interaction

Once connected, you can ask the assistant things like:

> "Search my history for anything about Rust error handling."

The assistant calls the `search` tool with `query: "rust error handling"` and
includes the results in its response, citing the specific pages you previously
read.

You can also ask:

> "Open the stored preview for the article I read about SQLite migrations."

The assistant can search first, then call `get_preview` with the selected URL.

Semantic search can be enabled per query by passing `"semantic": true` in the
tool arguments. This requires [semantic search to be configured](/docs/configuration#semantic-search)
on the server.
