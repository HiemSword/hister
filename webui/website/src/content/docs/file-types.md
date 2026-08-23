---
date: '2026-07-09T00:00:00+00:00'
draft: false
title: 'File Types'
description: 'Review supported local files, directory filters, document formats, and archive import behavior.'
---

Hister can index local files from configured directories and from explicit imports. Directory indexing is controlled by the `indexer.directories` configuration. After you configure a directory, restart the Hister server. It automatically scans the directory at startup and watches it for later changes. You do not need to run `hister import file` for this automatic tracking.

## Local File Indexing

| File type  | Extensions                     | Indexed content                               | Title source                                                   |
| ---------- | ------------------------------ | --------------------------------------------- | -------------------------------------------------------------- |
| PDF        | `.pdf`                         | Extracted plain text from all readable pages. | File path fallback                                             |
| DOCX       | `.docx`                        | Paragraph text.                               | DOCX metadata title when present, otherwise file path fallback |
| Markdown   | `.md`, `.markdown`             | Rendered Markdown text.                       | First H1 heading when present, otherwise file path fallback    |
| Org mode   | `.org`                         | Rendered Org text.                            | Org `TITLE` value when present, otherwise file path fallback   |
| Plain text | Any file with valid UTF 8 text | Full file contents.                           | File path fallback                                             |

Files that do not match a specialized handler are treated as plain text. Binary files are skipped.

The same handlers process snapshots created with `hister import file`. These imports are stored as remote file documents. Their extracted text and generated preview content are indexed, but the original file bytes are neither sent to the server nor retained.

## Directory Filters

The `filetypes` setting on a watched directory is an extension filter. Use names without the leading dot.

```yaml
indexer:
  directories:
    - path: '~/Documents'
      label: 'documents'
      filetypes: ['pdf', 'docx', 'md', 'txt']
```

If `filetypes` is omitted, Hister considers every file that passes the other directory rules. Specialized handlers run first, then valid UTF 8 text files are indexed as plain text.

Other directory rules still apply:

| Rule                         | Behavior                                                                                  |
| ---------------------------- | ----------------------------------------------------------------------------------------- |
| `label`                      | Applies the same searchable label to every file indexed from the directory.               |
| `include_hidden`             | Hidden files and directories are skipped unless this is enabled.                          |
| `excludes`                   | Matching paths are skipped.                                                               |
| `patterns`                   | When set, only matching files are considered.                                             |
| `indexer.max_file_size_mb`   | Files above the configured size limit are skipped.                                        |
| `sensitive_content_patterns` | Matching files are rejected unless the indexing path explicitly allows sensitive content. |

## Import Formats

The `hister import file` command accepts these file formats:

| File type           | Extensions                 | Behavior                                                     |
| ------------------- | -------------------------- | ------------------------------------------------------------ |
| Hister JSON export  | `.json`                    | Imports documents previously written by `hister export`.     |
| 7z archive          | `.7z`                      | Imports a compressed Hister JSON export.                     |
| Saved HTML page     | `.html`, `.htm`            | Extracts the original page URL when present.                 |
| Local file snapshot | Any supported local format | Extracts content locally and submits a remote file document. |

When importing a directory, Hister reads matching files recursively. With no input path, it uses every configured watched directory and applies its filters. This creates remote file snapshots and is intended only for directories that the command line client can access but the server cannot. The snapshots are not watched for changes.

Use `hister import file` to create snapshots of the PDF, DOCX, Markdown, Org mode, and plain text formats listed under local file indexing. Extraction occurs on the client, so this also works when the server cannot access the filesystem.
