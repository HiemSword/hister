---
date: '2026-02-13T10:59:19+01:00'
draft: false
title: 'Terminal Client'
description: 'Use Hister commands, global flags, terminal search, keyboard controls, mouse controls, and themes.'
---

See also [the configuration documentation](configuration).

## Global Flags

These flags are accepted on every command for compatibility. Subcommand help only displays the
flags that affect that command.

| Flag                       | Description                                                                                     |
| -------------------------- | ----------------------------------------------------------------------------------------------- |
| `--config PATH`            | Path to config file (default: `./config.yml`, `~/.histerrc`, or `~/.config/hister/config.yml`)  |
| `--server-url URL` / `-u`  | Hister server URL (overrides `server.base_url` from config)                                     |
| `--token TOKEN` / `-t`     | Access token for server authentication (overrides `app.access_token` from config)               |
| `--log-level LEVEL` / `-l` | Log level: `error`, `warning`, `info`, `debug`, `trace` (default: `info`)                       |
| `--search-url URL` / `-s`  | Default search engine URL with `{query}` placeholder                                            |
| `--client-timeout N`       | HTTP client timeout in seconds for server communication (0 = no timeout; default if unset: 10s) |

Example: index a slow extractor, such as `yt-dlp`, with a longer timeout:

```bash
hister --client-timeout 20 index https://example.com
```

## Execution Scope

Root help groups Hister operations by scope, and each operation help page identifies where the
command operates:

**Local** commands use configured files or local Hister data directly and do not contact the
configured Hister HTTP server. Examples include `hister crawl list`, `hister list-files`, and most
user administration commands.

**Remote** commands use the configured Hister HTTP server without opening the local Hister
database or search index. Examples include `hister search`, `hister export`, and `hister cleanup`.

**Hybrid** commands use or can use both local Hister state and the configured server. Examples
include persistent `hister index` modes, `hister import browser`, `hister list-urls --offline`, and
`hister delete-user`.

Use the scope label before running a command against a remote deployment. Local commands operate
on the data directory selected by the local configuration even when `--server-url` is supplied.

## Command-Line Usage

View all available commands:

```bash
./hister help
```

### Check for Updates

Check whether a newer Hister release is available:

```bash
hister check-update
```

The command displays the installed version and the latest published version when an update is
available. It also provides a link to the release. When Hister is current, the command reports
that it is up to date.

This check requires internet access. It does not download or install the update.

### Index a URL Manually

To manually index a specific URL:

```bash
./hister index https://example.com
```

For persistent recursive crawls, URL input jobs, custom job names, resume behavior, request
backends, and every `crawl` subcommand, see [Website Crawler](crawler).

### Updating Document Attributes

Use `hister update` to change attributes on every document selected by the query language:

```bash
hister update 'user_id:0' --user-id 2
hister update 'domain:example.com' --label research
hister update 'language:unknown' --language en
```

The supported attributes are `user_id`, `label`, `title`, and `language`. Pass an empty label or
title to clear it. Use `unknown` to clear a detected language. Hister reports the number of
matching, changed, unchanged, and conflicting documents.

The command previews the operation and asks for confirmation. Use `--dry` to preview without
applying changes, or `--yes` to apply immediately:

```bash
hister update 'label:inbox' --label archive --dry
hister update 'label:inbox' --label archive --yes
```

Changing `user_id` requires administrator access. User ID `0` makes a document global. An
ownership change never overwrites a document that already has the same URL and destination owner.
Those documents are reported as conflicts and skipped. Stored version records move with a changed
owner, while search history stays with the user who created it.

For a watched local file, its configured directory `user` must match the destination owner. Future
indexing can replace a manually changed title, language, or owner, so update the responsible
collector or directory configuration when the change should remain permanent.

### Exporting Documents

Use `hister export` to write indexed documents to a JSON file. This is useful for
backups or for moving documents between Hister instances. By default every indexed
document is exported:

```bash
hister export backup.json
```

You can limit the export to documents matching a search query by passing it after the
output file (see the [query language](query-language) reference):

```bash
hister export rust.json "rust language:en"
```

Each document is written as a single JSON line; lines that do not start with `{` are
structural markers (`[`, `]`, `,`) and can be safely skipped by parsers. Pass `-` as the
output file to write to standard output instead, which can be piped into other tools:

```bash
hister export - | gzip > backup.json.gz
```

Use `--start-date` / `--end-date` (`YYYY-MM-DD`) to only export documents whose `updated`
timestamp falls within the given date range. The resulting file can be re-imported with
`hister import file` (see below).

### Importing Documents

Use `hister import file` to add documents from files on disk. It accepts an arbitrary
number of files or directories, which are imported in order and reported as a
combined total:

```bash
hister import file export.json page.html ~/Downloads/saved-pages
```

Several input formats are supported:

- **JSON export files** files previously created by `hister export`. They are read
  line by line and each serialized document is restored without running content extraction
  again.
- **7z archives** (`.7z`) a 7z compressed archive containing a single JSON export file.
- **HTML files** (`.html` or `.htm`) a saved web page. The document URL is extracted
  from the HTML itself (the `<link rel="canonical">` tag, OpenGraph/Twitter `url` meta
  tags, etc.) and the page is submitted to the running server for processing.
- **Local file snapshots** PDF, DOCX, Markdown, Org mode, HTML without source URL metadata,
  JSON without the Hister export array shape, and valid UTF 8 text are extracted locally.
  Only prepared document fields are sent to the server.

When a directory is passed, Hister imports matching files recursively. With no input,
it imports files from configured watched directories and applies their filters.

```bash
# Import a single saved web page
hister import file ~/Downloads/article.html

# Import all supported files recursively from a directory
hister import file ~/Downloads/saved-pages

# Import files matched by every configured watched directory
hister import file
```

Useful flags:

- `--skip-existing` do not overwrite documents that are already in the index.
- `--start-date` / `--end-date` (`YYYY-MM-DD`) only import documents whose `added`
  timestamp falls within the given date range (applies to JSON exports).
- `--batch-size` controls how many documents are submitted in each bulk request.
  The default is `10` and the maximum is `100`.
- `--source` sets the stable source namespace for remote file snapshot URLs.
- `--allow-sensitive` skips sensitive content checks for imported snapshots.

> **Note:** `hister import file` talks to a running Hister server, so make sure the server
> is started before importing. See [Importing Documents](import) for file,
> browser history, and Linkwarden import instructions.

`hister import file` creates searchable snapshots when the server cannot read files directly. Snapshot extraction occurs locally and only the prepared document is sent through the normal add API. With no paths it imports every file matched by configured watched directories. Explicit directories are recursive.

```bash
hister import file
hister import file --source work-laptop ~/notes ~/Documents/report.pdf
```

The command extracts the same PDF, DOCX, Markdown, Org mode, and plain text formats used by watched directories. It does not send the original bytes or synchronize later file changes and removals. Run the command again to replace a snapshot with the same source name and absolute path.

## TUI (Terminal UI)

Hister provides a terminal-based user interface for searching your browsing history without leaving your terminal.

### Start the TUI

Run the search command without any arguments:

```bash
hister search
```

### TUI Features

- **Multi-tab interface**: Search, History, Rules, and Add tabs
- **Mouse support**: Scroll with mouse wheel, click to select, right-click for context menu
- **Theming**: Built-in color themes with interactive picker (press `ctrl+t`)
- **Settings overlay**: Edit keybindings interactively (press `ctrl+s`)
- **Context menu**: Right-click on results for quick actions (open, delete, prioritize)

### Tabs

- **Search** (Alt+1): Main search interface
- **History** (Alt+2): View your recent search history
- **Rules** (Alt+3): Manage skip, priority, versioning, and alias rules
- **Add** (Alt+4): Manually add URLs to the index

### TUI Keybindings

The TUI uses the following keybindings by default:

| Key          | Action          | Description                                    |
| ------------ | --------------- | ---------------------------------------------- |
| `ctrl+c`     | quit            | Exit the TUI                                   |
| `f1`         | toggle_help     | Show/hide keybindings help overlay             |
| `tab`, `esc` | toggle_focus    | Switch between search input and results list   |
| `up`, `k`    | scroll_up       | Navigate up in results                         |
| `down`, `j`  | scroll_down     | Navigate down in results                       |
| `enter`      | open_result     | Open the selected result in your browser       |
| `ctrl+d`     | delete_result   | Delete the selected result from the index      |
| `ctrl+t`     | toggle_theme    | Open the interactive theme picker              |
| `ctrl+s`     | toggle_settings | Open the keybinding editor overlay             |
| `ctrl+o`     | toggle_sort     | Toggle domain-based sorting for search results |
| `alt+1`      | tab_search      | Switch to the Search tab                       |
| `alt+2`      | tab_history     | Switch to the History tab                      |
| `alt+3`      | tab_rules       | Switch to the Rules tab                        |
| `alt+4`      | tab_add         | Switch to the Add tab                          |

### Mouse Controls

- **Left-click**: Select results or open tabs
- **Right-click**: Open context menu (open, delete, prioritize)
- **Scroll wheel**: Navigate through results
- **Scrollbar drag**: Quick scroll through long result lists

### Customizing TUI

TUI settings are stored in a separate `tui.yaml` file alongside your main config file. This file is automatically created with default values when you first run `hister search`.

On a typical Linux setup the path is `~/.config/hister/tui.yaml`. On other platforms, or when a custom config path is selected, `tui.yaml` is stored beside the main config file.

#### tui.yaml Structure

```yaml
# Theme settings
dark_theme: 'tokyonight'
light_theme: 'catppuccin-latte'
color_scheme: 'auto'
# themes_dir: "/path/to/custom/themes"  # optional

# TUI keybindings
hotkeys:
  ctrl+c: 'quit'
  ctrl+t: 'toggle_theme'
  ctrl+s: 'toggle_settings'
  ctrl+o: 'toggle_sort'
  alt+1: 'tab_search'
  alt+2: 'tab_history'
  alt+3: 'tab_rules'
  alt+4: 'tab_add'
  # ... and all other TUI keybindings
```

#### Available TUI Actions

- `quit` - Exit the TUI application
- `toggle_help` - Show/hide the help overlay
- `toggle_focus` - Switch between input and results views
- `scroll_up`/`scroll_down` - Move selection up/down
- `open_result` - Open selected URL in browser
- `delete_result` - Delete selected entry from index
- `toggle_theme` - Open theme picker
- `toggle_settings` - Open keybinding editor
- `toggle_sort` - Toggle sorting mode
- `tab_search`/`tab_history`/`tab_rules`/`tab_add` - Switch tabs

Note: After modifying `tui.yaml`, restart the `hister search` command to apply changes.
