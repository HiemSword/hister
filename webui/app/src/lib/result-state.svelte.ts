// SPDX-License-Identifier: AGPL-3.0-or-later

import { apiFetch, getUserId } from './api';
import { buildUrlSkipPattern, buildDomainSkipPattern } from '@hister/components';

export class ResultState {
  labelInput = $state('');
  labelMessage = $state<string | null>(null);
  labelError = $state(false);
  displayLabel = $state<string | undefined>(undefined);

  actionsQuery = $state('');
  actionsMessage = $state<string | null>(null);
  actionsError = $state(false);

  constructor(initialLabel?: string) {
    this.displayLabel = initialLabel || undefined;
    this.labelInput = initialLabel ?? '';
  }

  onOpen() {
    this.actionsMessage = null;
    this.actionsError = false;
    this.labelMessage = null;
    this.labelError = false;
  }

  async updateLabel(url: string) {
    this.labelMessage = null;
    this.labelError = false;
    const res = await apiFetch('/label', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url, label: this.labelInput }),
    });
    if (res.ok) {
      this.displayLabel = this.labelInput || undefined;
      this.labelMessage = this.labelInput ? 'Label saved.' : 'Label cleared.';
    } else {
      this.labelMessage = 'Failed to save label.';
      this.labelError = true;
    }
  }

  async pin(url: string, title: string, currentQuery: string, remove = false) {
    const q = this.actionsQuery || currentQuery;
    if (!q) return;
    const cleanTitle = title.replace(/<[^>]*>/g, '');
    try {
      await apiFetch('/history', {
        method: 'POST',
        headers: { 'Content-type': 'application/json; charset=UTF-8' },
        body: JSON.stringify({ url, title: cleanTitle, query: q, pin: !remove }),
      });
      this.actionsMessage = `Priority result ${remove ? 'removed' : 'added'}.`;
      this.actionsError = false;
    } catch {
      this.actionsMessage = 'Failed to update priority.';
      this.actionsError = true;
    }
  }

  async forgetForQuery(url: string, query: string): Promise<boolean> {
    if (!query) return false;
    try {
      const res = await apiFetch('/history', {
        method: 'POST',
        headers: { 'Content-type': 'application/json; charset=UTF-8' },
        body: JSON.stringify({ url, query, delete: true }),
      });
      if (!res.ok) throw new Error('Failed to forget result for query');
      this.actionsMessage = 'Result forgotten for this query.';
      this.actionsError = false;
      return true;
    } catch {
      this.actionsMessage = 'Failed to forget result for this query.';
      this.actionsError = true;
      return false;
    }
  }

  async addSkipRule(
    url: string,
    domain: string,
    type: 'url' | 'domain',
    deleteMatches: boolean,
    removeResult: (url: string) => void,
    removeResultsByDomain: (domain: string) => void,
  ) {
    this.actionsMessage = null;
    this.actionsError = false;
    try {
      const pattern = type === 'url' ? buildUrlSkipPattern(url) : buildDomainSkipPattern(url);
      const rulesResp = await apiFetch('/rules');
      const rules = await rulesResp.json();
      const skipList: string[] = rules.skip || [];
      if (!skipList.includes(pattern)) {
        skipList.push(pattern);
      }
      const formData = new URLSearchParams();
      formData.set('skip', skipList.join('\n'));
      formData.set('priority', (rules.priority || []).join('\n'));
      await apiFetch('/rules', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: formData.toString(),
      });
      if (deleteMatches) {
        const uid = getUserId();
        const userFilter = uid !== undefined ? ` user_id:${uid}` : '';
        const deleteQuery =
          type === 'url'
            ? `url:"${url.replaceAll('"', '\\"')}"${userFilter}`
            : `domain:${domain}${userFilter}`;
        await apiFetch('/delete', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ query: deleteQuery }),
        });
        if (type === 'url') {
          removeResult(url);
        } else {
          removeResultsByDomain(domain);
        }
      }
      this.actionsMessage = `Skip rule added${deleteMatches ? ' and documents deleted' : ''}.`;
    } catch {
      this.actionsMessage = 'Failed to add skip rule.';
      this.actionsError = true;
    }
  }
}
