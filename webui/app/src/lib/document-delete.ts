// SPDX-License-Identifier: AGPL-3.0-or-later

import { apiFetch } from '$lib/api';

interface DocumentDeletionResponse {
  matched?: number;
  deleted?: number;
}

async function requestDocumentDeletion(
  query: string,
  dryRun: boolean,
): Promise<DocumentDeletionResponse> {
  const res = await apiFetch('/delete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query, ...(dryRun && { dry_run: true }) }),
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(body.trim() || 'Document deletion failed');
  }
  return res.json();
}

export async function previewDocumentDeletion(query: string): Promise<number> {
  const result = await requestDocumentDeletion(query, true);
  return Number(result.matched ?? 0);
}

export async function deleteDocuments(query: string): Promise<number> {
  const result = await requestDocumentDeletion(query, false);
  return Number(result.deleted ?? 0);
}
