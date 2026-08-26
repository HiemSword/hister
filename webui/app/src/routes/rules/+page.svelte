<script lang="ts">
  import { onMount, tick } from 'svelte';
  import type { Action } from 'svelte/action';

  const focusInput: Action<HTMLElement> = (node) => {
    (node.querySelector('input') as HTMLInputElement | null)?.focus();
  };
  import { fetchConfig, apiFetch, getUserId } from '$lib/api';
  import { base } from '$app/paths';
  import BulkRulesDialog from '$lib/components/BulkRulesDialog.svelte';
  import DeleteMatchingDocumentsOption from '$lib/components/DeleteMatchingDocumentsOption.svelte';
  import { Button } from '@hister/components/ui/button';
  import { Input } from '@hister/components/ui/input';
  import { Badge } from '@hister/components/ui/badge';
  import * as Card from '@hister/components/ui/card';
  import * as Dialog from '@hister/components/ui/dialog';
  import * as Table from '@hister/components/ui/table';
  import {
    Shield,
    Link2,
    Plus,
    Trash2,
    Pencil,
    Check,
    X,
    Search,
    ChevronsUpDown,
    ChevronUp,
    ChevronDown,
    ListPlus,
  } from '@lucide/svelte';
  import { PageHeader } from '@hister/components';
  import { Label } from '@hister/components/ui/label';
  import * as Alert from '@hister/components/ui/alert';
  import AlertCircle from '@lucide/svelte/icons/circle-alert';
  import CheckCircle from '@lucide/svelte/icons/circle-check';

  interface RulesData {
    skip: string[];
    priority: string[];
    versioning: string[];
    aliases: Record<string, string>;
  }

  interface RuleRow {
    pattern: string;
    type: 'skip' | 'priority' | 'versioning';
    addedOrder: number;
  }

  interface PendingDocumentDeletion {
    patterns: string[];
    matched: number;
    savedMessage: string;
  }

  function parseRulePatterns(value: string, existingPatterns: string[]) {
    const seenPatterns = new Set(existingPatterns);
    const uniquePatterns: string[] = [];
    let duplicateCount = 0;

    for (const pattern of value.split(/\r?\n/)) {
      const trimmedPattern = pattern.trim();
      if (!trimmedPattern) continue;
      if (seenPatterns.has(trimmedPattern)) {
        duplicateCount++;
        continue;
      }
      seenPatterns.add(trimmedPattern);
      uniquePatterns.push(trimmedPattern);
    }

    return { uniquePatterns, duplicateCount };
  }

  function ruleCount(count: number) {
    return `${count} rule${count === 1 ? '' : 's'}`;
  }

  function rulesAddedMessage(addedCount: number, duplicateCount: number) {
    const duplicateMessage = `${duplicateCount} duplicate${duplicateCount === 1 ? '' : 's'}`;
    if (addedCount === 0) return `No new rules added; skipped ${duplicateMessage}.`;
    if (duplicateCount === 0) return `${ruleCount(addedCount)} added.`;
    return `${ruleCount(addedCount)} added; skipped ${duplicateMessage}.`;
  }

  let rules: RulesData = $state({ skip: [], priority: [], versioning: [], aliases: {} });
  let loading = $state(true);
  let saving = $state(false);
  let message = $state('');
  let isError = $state(false);
  let newAliasKeyword = $state('');
  let newAliasValue = $state('');
  let newRulePattern = $state('');
  let newRuleType: 'skip' | 'priority' | 'versioning' = $state('skip');
  let deleteMatchingDocuments = $state(false);
  let bulkAddOpen = $state(false);
  let bulkRulePatterns = $state('');
  let bulkDeleteMatchingDocuments = $state(false);
  let deleteConfirmOpen = $state(false);
  let deletingMatchingDocuments = $state(false);
  let pendingDocumentDeletion = $state<PendingDocumentDeletion | null>(null);

  // Editing state for aliases
  let editingAliasKey = $state<string | null>(null);
  let editAliasKeyword = $state('');
  let editAliasValue = $state('');

  // Editing state for rules
  let editingRuleIndex = $state<number | null>(null);
  let editRulePattern = $state('');
  let editRuleType: 'skip' | 'priority' | 'versioning' = $state('skip');
  let editDeleteMatchingDocuments = $state(false);

  // Filter state
  let aliasFilterOpen = $state(false);
  let aliasFilter = $state('');
  let ruleFilterOpen = $state(false);
  let ruleFilter = $state('');

  // Sort state
  class SortState<C extends string> {
    col = $state<C | null>(null);
    dir = $state<'asc' | 'desc'>('asc');
    toggle(col: C) {
      if (this.col === col) {
        if (this.dir === 'asc') this.dir = 'desc';
        else {
          this.col = null;
          this.dir = 'asc';
        }
      } else {
        this.col = col;
        this.dir = 'asc';
      }
    }
  }
  const aliasSort = new SortState<'keyword' | 'value'>();
  const ruleSort = new SortState<'pattern' | 'type'>();

  const existingRulePatterns = $derived([...rules.skip, ...rules.priority, ...rules.versioning]);
  const bulkRuleSummary = $derived.by(() =>
    parseRulePatterns(bulkRulePatterns, existingRulePatterns),
  );

  const ruleRows = $derived.by(() => {
    const rows: RuleRow[] = [];
    let addedOrder = 0;
    for (const p of rules.skip) rows.push({ pattern: p, type: 'skip', addedOrder: addedOrder++ });
    for (const p of rules.priority)
      rows.push({ pattern: p, type: 'priority', addedOrder: addedOrder++ });
    for (const p of rules.versioning)
      rows.push({ pattern: p, type: 'versioning', addedOrder: addedOrder++ });
    return rows;
  });

  const filteredAliases = $derived.by(() => {
    const q = aliasFilter.trim().toLowerCase();
    const entries = Object.entries(rules.aliases);
    if (!q) return entries;
    return entries.filter(([k, v]) => k.toLowerCase().includes(q) || v.toLowerCase().includes(q));
  });

  const filteredRuleRows = $derived.by(() => {
    const q = ruleFilter.trim().toLowerCase();
    const indexed = ruleRows.map((row, i) => ({ row, i }));
    if (!q) return indexed;
    return indexed.filter(({ row }) => row.pattern.toLowerCase().includes(q));
  });

  const sortedAliases = $derived.by(() => {
    const arr = [...filteredAliases];
    if (!aliasSort.col) return arr;
    const { col, dir } = aliasSort;
    return arr.sort((a, b) => {
      const va = col === 'keyword' ? a[0] : a[1];
      const vb = col === 'keyword' ? b[0] : b[1];
      return dir === 'asc' ? va.localeCompare(vb) : vb.localeCompare(va);
    });
  });

  const sortedRuleRows = $derived.by(() => {
    const arr = [...filteredRuleRows];
    if (!ruleSort.col) return arr.sort((a, b) => b.row.addedOrder - a.row.addedOrder);
    const { col, dir } = ruleSort;
    return arr.sort((a, b) => {
      const va = col === 'pattern' ? a.row.pattern : a.row.type;
      const vb = col === 'pattern' ? b.row.pattern : b.row.type;
      return dir === 'asc' ? va.localeCompare(vb) : vb.localeCompare(va);
    });
  });

  onMount(async () => {
    const cfg = await fetchConfig();
    if (cfg.public && !cfg.canWrite) {
      window.location.href = base + '/auth';
      return;
    }
    await loadRules();
    await tick();
    if (window.location.hash) {
      document.getElementById(window.location.hash.slice(1))?.scrollIntoView({ block: 'start' });
    }
  });

  async function loadRules() {
    loading = true;
    try {
      const res = await apiFetch('/rules', { headers: { Accept: 'application/json' } });
      if (!res.ok) throw new Error('Failed to load rules');
      const data = await res.json();
      rules = {
        skip: data.skip ?? [],
        priority: data.priority ?? [],
        versioning: data.versioning ?? [],
        aliases: data.aliases ?? {},
      };
    } catch (e) {
      message = String(e);
      isError = true;
    } finally {
      loading = false;
    }
  }

  async function saveRules(nextRules: RulesData = rules): Promise<boolean> {
    if (saving) return false;
    saving = true;
    message = '';
    try {
      const formData = new URLSearchParams();
      formData.set('skip', nextRules.skip.join('\n'));
      formData.set('priority', nextRules.priority.join('\n'));
      formData.set('versioning', nextRules.versioning.join('\n'));
      const res = await apiFetch('/rules', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: formData.toString(),
      });
      if (!res.ok) {
        const body = await res.text();
        throw new Error(body.trim() || 'Failed to save rules');
      }
      message = 'Rules saved successfully';
      isError = false;
      await loadRules();
      return true;
    } catch (e) {
      message = String(e);
      isError = true;
      return false;
    } finally {
      saving = false;
    }
  }

  function removeRule(pattern: string, type: 'skip' | 'priority' | 'versioning') {
    if (type === 'skip') {
      rules.skip = rules.skip.filter((p) => p !== pattern);
    } else if (type === 'priority') {
      rules.priority = rules.priority.filter((p) => p !== pattern);
    } else {
      rules.versioning = rules.versioning.filter((p) => p !== pattern);
    }
    saveRules();
  }

  function rulesWithPatterns(patterns: string[]): RulesData {
    return {
      ...rules,
      [newRuleType]: [...rules[newRuleType], ...patterns],
    };
  }

  async function addRule() {
    const pattern = newRulePattern.trim();
    if (!pattern) return;

    if (existingRulePatterns.includes(pattern)) {
      message = `Rule "${pattern}" already exists.`;
      isError = true;
      return;
    }

    const shouldDeleteMatches = newRuleType === 'skip' && deleteMatchingDocuments;
    if (!(await saveRules(rulesWithPatterns([pattern])))) return;
    newRulePattern = '';
    deleteMatchingDocuments = false;
    await deleteMatchesAfterSaving([pattern], shouldDeleteMatches, rulesAddedMessage(1, 0));
  }

  function openBulkAdd() {
    bulkRulePatterns = '';
    bulkDeleteMatchingDocuments = false;
    bulkAddOpen = true;
  }

  async function addBulkRules() {
    const { uniquePatterns, duplicateCount } = bulkRuleSummary;

    if (uniquePatterns.length === 0) {
      bulkRulePatterns = '';
      bulkDeleteMatchingDocuments = false;
      bulkAddOpen = false;
      message = rulesAddedMessage(0, duplicateCount);
      isError = false;
      return;
    }

    const shouldDeleteMatches = newRuleType === 'skip' && bulkDeleteMatchingDocuments;
    if (!(await saveRules(rulesWithPatterns(uniquePatterns)))) return;

    bulkRulePatterns = '';
    bulkDeleteMatchingDocuments = false;
    bulkAddOpen = false;
    await deleteMatchesAfterSaving(
      uniquePatterns,
      shouldDeleteMatches,
      rulesAddedMessage(uniquePatterns.length, duplicateCount),
    );
  }

  function quoteRegexpValue(pattern: string): string {
    let escaped = '';
    let backslashCount = 0;
    for (const char of pattern) {
      if (char === '\\') {
        escaped += char;
        backslashCount++;
        continue;
      }
      if (char === '"' && backslashCount % 2 === 0) escaped += '\\';
      escaped += char;
      backslashCount = 0;
    }
    return escaped;
  }

  function deleteQuery(patterns: string[]): string {
    const userID = getUserId();
    const userFilter = userID === undefined ? '' : ` user_id:${userID}`;
    const pattern = patterns.map((value) => `(${value})`).join('|');
    return `url_re:"${quoteRegexpValue(pattern)}"${userFilter}`;
  }

  async function deleteRegexpMatches(patterns: string[]): Promise<number> {
    const res = await apiFetch('/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query: deleteQuery(patterns) }),
    });
    if (!res.ok) {
      const body = await res.text();
      throw new Error(body.trim() || 'Failed to delete matching documents');
    }
    const data = await res.json();
    return Number(data.deleted ?? 0);
  }

  async function countRegexpMatches(patterns: string[]): Promise<number> {
    const res = await apiFetch('/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query: deleteQuery(patterns), dry_run: true }),
    });
    if (!res.ok) {
      const body = await res.text();
      throw new Error(body.trim() || 'Failed to count matching documents');
    }
    const data = await res.json();
    return Number(data.matched ?? 0);
  }

  async function deleteMatchesAfterSaving(
    patterns: string[],
    enabled: boolean,
    savedMessage: string,
  ) {
    if (!enabled) {
      message = savedMessage;
      return;
    }
    try {
      const matched = await countRegexpMatches(patterns);
      if (matched === 0) {
        message = `${savedMessage} No matching documents found.`;
        isError = false;
        return;
      }
      pendingDocumentDeletion = { patterns, matched, savedMessage };
      deleteConfirmOpen = true;
      isError = false;
    } catch (e) {
      message = `${savedMessage} Matching documents could not be checked: ${String(e)}`;
      isError = true;
    }
  }

  async function confirmMatchingDocumentDeletion() {
    const pending = pendingDocumentDeletion;
    if (!pending) return;
    deletingMatchingDocuments = true;
    try {
      const deleted = await deleteRegexpMatches(pending.patterns);
      message = `${pending.savedMessage} ${deleted} matching document${deleted === 1 ? '' : 's'} deleted.`;
      isError = false;
    } catch (e) {
      message = `${pending.savedMessage} Matching documents could not be deleted: ${String(e)}`;
      isError = true;
    } finally {
      deletingMatchingDocuments = false;
      deleteConfirmOpen = false;
      pendingDocumentDeletion = null;
    }
  }

  function cancelMatchingDocumentDeletion() {
    const savedMessage = pendingDocumentDeletion?.savedMessage;
    deleteConfirmOpen = false;
    pendingDocumentDeletion = null;
    if (savedMessage) {
      message = `${savedMessage} Matching documents were not deleted.`;
      isError = false;
    }
  }

  async function deleteAlias(keyword: string) {
    const formData = new URLSearchParams({ alias: keyword });
    const res = await apiFetch('/delete_alias', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: formData.toString(),
    });
    if (res.ok) await loadRules();
  }

  async function addAlias(e: Event) {
    e.preventDefault();
    if (!newAliasKeyword || !newAliasValue) return;
    const keyword = newAliasKeyword.trim();
    if (Object.prototype.hasOwnProperty.call(rules.aliases, keyword)) {
      message = `Alias "${keyword}" already exists.`;
      isError = true;
      return;
    }
    const formData = new URLSearchParams({
      'alias-keyword': keyword,
      'alias-value': newAliasValue,
    });
    const res = await apiFetch('/add_alias', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: formData.toString(),
    });
    if (res.ok) {
      newAliasKeyword = '';
      newAliasValue = '';
      await loadRules();
    }
  }

  function startEditAlias(keyword: string, value: string) {
    editingAliasKey = keyword;
    editAliasKeyword = keyword;
    editAliasValue = value;
  }

  function cancelEditAlias() {
    editingAliasKey = null;
  }

  async function saveEditAlias() {
    const trimmedKeyword = editAliasKeyword.trim();
    const trimmedValue = editAliasValue.trim();
    if (!trimmedKeyword || !trimmedValue) return;
    const oldKey = editingAliasKey!;

    // If the keyword is being renamed, check the new keyword doesn't already exist
    if (
      trimmedKeyword !== oldKey &&
      Object.prototype.hasOwnProperty.call(rules.aliases, trimmedKeyword)
    ) {
      message = `Alias "${trimmedKeyword}" already exists.`;
      isError = true;
      return;
    }

    // Add/overwrite with new keyword+value
    const addForm = new URLSearchParams({
      'alias-keyword': trimmedKeyword,
      'alias-value': trimmedValue,
    });
    const addRes = await apiFetch('/add_alias', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: addForm.toString(),
    });
    if (!addRes.ok) return;

    // If the keyword was renamed, delete the old key
    if (trimmedKeyword !== oldKey) {
      await apiFetch('/delete_alias', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams({ alias: oldKey }).toString(),
      });
    }

    editingAliasKey = null;
    await loadRules();
  }

  function startEditRule(index: number) {
    const row = ruleRows[index];
    editingRuleIndex = index;
    editRulePattern = row.pattern;
    editRuleType = row.type;
    editDeleteMatchingDocuments = false;
  }

  function cancelEditRule() {
    editingRuleIndex = null;
    editDeleteMatchingDocuments = false;
  }

  async function saveEditRule() {
    const trimmed = editRulePattern.trim();
    if (!trimmed) return;
    const row = ruleRows[editingRuleIndex!];
    // Reject if the new pattern already exists elsewhere (different item)
    const isDuplicate =
      (rules.skip.includes(trimmed) ||
        rules.priority.includes(trimmed) ||
        rules.versioning.includes(trimmed)) &&
      trimmed !== row.pattern;
    if (isDuplicate) {
      message = `Rule "${trimmed}" already exists.`;
      isError = true;
      editingRuleIndex = null;
      editDeleteMatchingDocuments = false;
      return;
    }
    const shouldDeleteMatches = editRuleType === 'skip' && editDeleteMatchingDocuments;
    // Update in the appropriate array
    if (row.type === 'skip') {
      rules.skip = rules.skip.map((p) => (p === row.pattern ? trimmed : p));
    } else if (row.type === 'priority') {
      rules.priority = rules.priority.map((p) => (p === row.pattern ? trimmed : p));
    } else {
      rules.versioning = rules.versioning.map((p) => (p === row.pattern ? trimmed : p));
    }
    // If type changed, move between arrays
    if (editRuleType !== row.type) {
      // Remove from old array
      if (row.type === 'skip') {
        rules.skip = rules.skip.filter((p) => p !== trimmed);
      } else if (row.type === 'priority') {
        rules.priority = rules.priority.filter((p) => p !== trimmed);
      } else {
        rules.versioning = rules.versioning.filter((p) => p !== trimmed);
      }
      // Add to new array
      if (editRuleType === 'skip') {
        rules.skip = [...rules.skip, trimmed];
      } else if (editRuleType === 'priority') {
        rules.priority = [...rules.priority, trimmed];
      } else {
        rules.versioning = [...rules.versioning, trimmed];
      }
    }
    editingRuleIndex = null;
    editDeleteMatchingDocuments = false;
    if (!(await saveRules())) return;
    await deleteMatchesAfterSaving([trimmed], shouldDeleteMatches, 'Rule updated.');
  }
</script>

<svelte:head>
  <title>Hister - Rules</title>
</svelte:head>

<div class="flex flex-1 flex-col gap-8 overflow-y-auto px-6 py-8 md:gap-10 md:px-12 md:py-12">
  <!-- Section Header -->
  <div class="flex flex-col gap-4">
    <PageHeader color="hister-coral" size="lg" class="uppercase">Rules &amp; Aliases</PageHeader>
  </div>

  {#if message}
    <Alert.Root variant={isError ? 'error' : 'success'} class="shadow-brutal border-[3px]">
      {#if isError}
        <AlertCircle class="size-5 shrink-0" />
      {:else}
        <CheckCircle class="size-5 shrink-0" />
      {/if}
      <Alert.Description class="font-inter text-sm">{message}</Alert.Description>
    </Alert.Root>
  {/if}

  {#snippet filterToggleButton(colorClass: string, toggle: () => void)}
    <Button
      size="sm"
      onclick={toggle}
      class="font-space border-brutal-border brutal-press h-8 gap-1.5 border-[3px] px-3 text-xs font-bold tracking-[1px] uppercase {colorClass}"
    >
      <Search class="size-3" />Filter
    </Button>
  {/snippet}

  {#snippet filterInputRow(
    open: boolean,
    filterValue: string,
    setFilter: (v: string) => void,
    placeholder: string,
    focusClass: string,
  )}
    {#if open}
      <Table.Row class="bg-muted-surface border-brutal-border border-b-[3px]">
        <Table.Head colspan={3} class="h-auto px-2 py-2 md:px-5">
          <div use:focusInput>
            <Input
              type="text"
              variant="brutal"
              value={filterValue}
              oninput={(e) => setFilter((e.target as HTMLInputElement).value)}
              {placeholder}
              class="bg-card-surface h-8 w-full px-3 text-sm font-normal {focusClass}"
            />
          </div>
        </Table.Head>
      </Table.Row>
    {/if}
  {/snippet}

  {#snippet sortableHead(
    label: string,
    col: string,
    sortCol: string | null,
    sortDir: 'asc' | 'desc',
    onToggle: () => void,
    headClass: string,
  )}
    <Table.Head class={headClass}>
      <button
        class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 font-[inherit] tracking-[inherit] text-[inherit] uppercase opacity-80 hover:opacity-100"
        onclick={onToggle}
      >
        {label}
        {#if sortCol === col}
          {#if sortDir === 'asc'}
            <ChevronUp class="size-3 shrink-0" />
          {:else}
            <ChevronDown class="size-3 shrink-0" />
          {/if}
        {:else}
          <ChevronsUpDown class="size-3 shrink-0 opacity-50" />
        {/if}
      </button>
    </Table.Head>
  {/snippet}

  {#snippet editCancelButtons(onSave: () => void, onCancel: () => void)}
    <div class="flex items-center gap-1">
      <Button
        variant="ghost"
        size="icon-sm"
        class="text-hister-teal shrink-0 transition-colors"
        onclick={onSave}
      >
        <Check class="size-4" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        class="text-text-brand-muted shrink-0 transition-colors"
        onclick={onCancel}
      >
        <X class="size-4" />
      </Button>
    </div>
  {/snippet}

  {#snippet editDeleteButtons(onEdit: () => void, onDelete: () => void, editHoverClass: string)}
    <div class="flex items-center gap-1">
      <Button
        variant="ghost"
        size="icon-sm"
        class="text-text-brand-muted shrink-0 transition-colors {editHoverClass}"
        onclick={onEdit}
      >
        <Pencil class="size-4" />
      </Button>
      <Button
        variant="ghost"
        size="icon-sm"
        class="text-text-brand-muted hover:text-hister-rose shrink-0 transition-colors"
        onclick={onDelete}
      >
        <Trash2 class="size-4" />
      </Button>
    </div>
  {/snippet}

  {#if loading}
    <div class="flex items-center justify-center py-16">
      <p class="font-inter text-text-brand-muted text-lg">Loading rules...</p>
    </div>
  {:else}
    <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <!-- Search Aliases Card -->
      <Card.Root id="search-aliases" class="scroll-mt-6">
        <Card.Header color="hister-indigo">
          <div class="flex h-12 w-12 shrink-0 items-center justify-center bg-white/20">
            <Link2 class="size-6 text-white" />
          </div>
          <div class="flex flex-col gap-1">
            <Card.Title
              class="font-space text-xl font-extrabold tracking-[1px] text-white uppercase"
              >Search aliases</Card.Title
            >
            <Card.Description class="font-inter text-sm text-white/80"
              >{Object.keys(rules.aliases).length} aliases configured</Card.Description
            >
          </div>
        </Card.Header>

        <div
          class="bg-muted-surface border-brutal-border flex items-center border-b-[3px] px-4 py-4 md:px-5 md:py-5"
        >
          <form
            onsubmit={addAlias}
            class="flex w-full flex-col items-stretch gap-3 md:flex-row md:items-end"
          >
            <div class="flex flex-col gap-1">
              <Label class="font-outfit text-text-brand text-sm font-bold">Keyword</Label>
              <Input
                type="text"
                variant="brutal"
                bind:value={newAliasKeyword}
                placeholder="keyword..."
                class="bg-card-surface focus-visible:border-hister-indigo h-10 w-full px-3 md:w-35"
              />
            </div>
            <div class="flex flex-1 flex-col gap-1">
              <Label class="font-outfit text-text-brand text-sm font-bold">Expands to</Label>
              <Input
                type="text"
                variant="brutal"
                bind:value={newAliasValue}
                placeholder="expands to..."
                class="bg-card-surface focus-visible:border-hister-indigo h-10 w-full px-3"
              />
            </div>
            <Button
              type="submit"
              class="bg-hister-indigo font-space border-brutal-border brutal-press h-10 gap-2 border-[3px] px-5 text-sm font-bold tracking-[1px] text-white uppercase"
            >
              <Plus class="size-4 shrink-0" />
              Add
            </Button>
          </form>
        </div>

        <Card.Content class="flex-1 p-0">
          <!-- Aliases table -->
          <Table.Root>
            <Table.Header>
              <Table.Row
                class="bg-muted-surface border-brutal-border hover:bg-muted-surface border-b-[3px]"
              >
                {@render sortableHead(
                  'Keyword',
                  'keyword',
                  aliasSort.col,
                  aliasSort.dir,
                  () => aliasSort.toggle('keyword'),
                  'font-space text-text-brand-muted h-auto w-20 px-2 py-3 text-xs font-bold tracking-[1px] uppercase md:w-35 md:px-5',
                )}
                {@render sortableHead(
                  'Expands to',
                  'value',
                  aliasSort.col,
                  aliasSort.dir,
                  () => aliasSort.toggle('value'),
                  'font-space text-text-brand-muted h-auto px-2 py-3 text-xs font-bold tracking-[1px] uppercase md:px-5',
                )}
                <Table.Head class="h-auto w-16 px-2 py-3 md:w-20 md:px-5">
                  {@render filterToggleButton('bg-hister-indigo text-white', () => {
                    aliasFilterOpen = !aliasFilterOpen;
                    if (!aliasFilterOpen) aliasFilter = '';
                  })}
                </Table.Head>
              </Table.Row>
              {@render filterInputRow(
                aliasFilterOpen,
                aliasFilter,
                (v) => (aliasFilter = v),
                'Filter aliases...',
                'focus-visible:border-hister-indigo',
              )}
            </Table.Header>
            <Table.Body>
              {#each sortedAliases as [keyword, value]}
                <Table.Row class="border-brutal-border border-b-[3px]">
                  {#if editingAliasKey === keyword}
                    <Table.Cell class="px-2 py-2 md:px-3" colspan={2}>
                      <div class="flex items-center gap-2">
                        <Input
                          type="text"
                          variant="brutal"
                          bind:value={editAliasKeyword}
                          class="bg-card-surface focus-visible:border-hister-indigo h-8 w-20 px-2 text-sm md:w-28"
                        />
                        <Input
                          type="text"
                          variant="brutal"
                          bind:value={editAliasValue}
                          class="bg-card-surface focus-visible:border-hister-indigo h-8 flex-1 px-2 text-sm"
                          onkeydown={(e) => {
                            if (e.key === 'Enter') saveEditAlias();
                            if (e.key === 'Escape') cancelEditAlias();
                          }}
                        />
                      </div>
                    </Table.Cell>
                    <Table.Cell class="w-16 px-1 py-2 md:w-20 md:px-3">
                      {@render editCancelButtons(saveEditAlias, cancelEditAlias)}
                    </Table.Cell>
                  {:else}
                    <Table.Cell
                      class="font-fira text-text-brand w-20 px-2 py-3 text-sm font-semibold md:w-35 md:px-5"
                      >{keyword}</Table.Cell
                    >
                    <Table.Cell
                      class="font-fira text-text-brand-secondary max-w-0 truncate px-2 py-3 text-sm md:px-5"
                      >{value}</Table.Cell
                    >
                    <Table.Cell class="w-16 px-1 py-3 md:w-20 md:px-3">
                      {@render editDeleteButtons(
                        () => startEditAlias(keyword, value),
                        () => deleteAlias(keyword),
                        'hover:text-hister-indigo',
                      )}
                    </Table.Cell>
                  {/if}
                </Table.Row>
              {/each}
            </Table.Body>
          </Table.Root>

          {#if sortedAliases.length === 0}
            <div class="flex flex-col items-center justify-center gap-3 py-10">
              <div
                class="flex h-12 w-12 items-center justify-center"
                style="background-color: color-mix(in srgb, var(--hister-indigo) 10%, transparent); color: var(--hister-indigo);"
              >
                <Link2 class="size-5" />
              </div>
              <p class="font-inter text-text-brand-muted text-sm">
                {aliasFilter ? 'No aliases match the filter.' : 'No aliases defined yet.'}
              </p>
            </div>
          {/if}
        </Card.Content>
      </Card.Root>

      <!-- Indexing Rules Card -->
      <Card.Root id="indexing-rules" class="scroll-mt-6">
        <Card.Header color="hister-coral">
          <div class="flex h-12 w-12 shrink-0 items-center justify-center bg-white/20">
            <Shield class="size-6 text-white" />
          </div>
          <div class="flex flex-col gap-1">
            <Card.Title
              class="font-space text-xl font-extrabold tracking-[1px] text-white uppercase"
              >Indexing rules</Card.Title
            >
            <Card.Description class="font-inter text-sm text-white/80"
              >{ruleRows.length} rules configured · patterns use
              <a
                href="https://pkg.go.dev/regexp/syntax"
                target="_blank"
                class="text-white underline opacity-80 hover:opacity-100">Go regexp</a
              > syntax</Card.Description
            >
          </div>
        </Card.Header>

        <div
          class="bg-muted-surface border-brutal-border flex items-center border-b-[3px] px-4 py-4 md:px-5 md:py-5"
        >
          <div class="flex w-full flex-col gap-3">
            <div class="flex flex-col items-stretch gap-3 md:flex-row md:items-end">
              <div class="flex flex-1 flex-col gap-1">
                <Label for="rule-pattern" class="font-outfit text-text-brand text-sm font-bold"
                  >Pattern</Label
                >
                <Input
                  id="rule-pattern"
                  type="text"
                  variant="brutal"
                  bind:value={newRulePattern}
                  placeholder="Enter Go regexp pattern"
                  class="bg-card-surface focus-visible:border-hister-coral h-10 w-full px-3"
                  onkeydown={(e) => {
                    if (e.key === 'Enter') addRule();
                  }}
                />
              </div>
              <div class="flex flex-col gap-1">
                <Label for="rule-type" class="font-outfit text-text-brand text-sm font-bold"
                  >Type</Label
                >
                <select
                  id="rule-type"
                  value={newRuleType}
                  onchange={(event) => {
                    newRuleType = (event.currentTarget as HTMLSelectElement)
                      .value as typeof newRuleType;
                    if (newRuleType !== 'skip') deleteMatchingDocuments = false;
                  }}
                  class="bg-card-surface border-brutal-border font-space text-text-brand h-10 w-full shrink-0 cursor-pointer appearance-none border-[3px] px-3 text-center text-xs font-bold tracking-[0.5px] outline-none md:w-27.5"
                >
                  <option value="skip">SKIP</option>
                  <option value="priority">PRIORITY</option>
                  <option value="versioning">VERSION</option>
                </select>
              </div>
              <div class="flex gap-2">
                <Button
                  type="button"
                  variant="outline"
                  onclick={openBulkAdd}
                  disabled={saving}
                  class="bg-card-surface text-text-brand-secondary font-space border-brutal-border brutal-press h-10 flex-1 gap-2 rounded-none border-[3px] px-4 text-xs font-bold tracking-[0.5px] uppercase md:flex-none"
                >
                  <ListPlus class="size-4 shrink-0" />
                  Bulk add
                </Button>
                <Button
                  type="button"
                  onclick={addRule}
                  disabled={saving || !newRulePattern.trim()}
                  class="bg-hister-coral font-space border-brutal-border brutal-press h-10 flex-1 gap-2 border-[3px] px-5 text-sm font-bold tracking-[1px] text-white uppercase md:flex-none"
                >
                  <Plus class="size-4 shrink-0" />
                  Add
                </Button>
              </div>
            </div>
            {#if newRuleType === 'skip'}
              <DeleteMatchingDocumentsOption bind:checked={deleteMatchingDocuments} />
            {/if}
          </div>
        </div>

        <Card.Content class="flex-1 p-0">
          <!-- Rules table -->
          <Table.Root>
            <Table.Header>
              <Table.Row
                class="bg-muted-surface border-brutal-border hover:bg-muted-surface border-b-[3px]"
              >
                {@render sortableHead(
                  'Pattern',
                  'pattern',
                  ruleSort.col,
                  ruleSort.dir,
                  () => ruleSort.toggle('pattern'),
                  'font-space text-text-brand-muted h-auto px-2 py-3 text-xs font-bold tracking-[1px] uppercase md:px-5',
                )}
                {@render sortableHead(
                  'Type',
                  'type',
                  ruleSort.col,
                  ruleSort.dir,
                  () => ruleSort.toggle('type'),
                  'font-space text-text-brand-muted h-auto w-20 px-2 py-3 text-xs font-bold tracking-[1px] uppercase md:w-28 md:px-5',
                )}
                <Table.Head class="h-auto w-16 px-2 py-3 md:w-20 md:px-5">
                  {@render filterToggleButton('bg-hister-coral text-white', () => {
                    ruleFilterOpen = !ruleFilterOpen;
                    if (!ruleFilterOpen) ruleFilter = '';
                  })}
                </Table.Head>
              </Table.Row>
              {@render filterInputRow(
                ruleFilterOpen,
                ruleFilter,
                (v) => (ruleFilter = v),
                'Filter rules...',
                'focus-visible:border-hister-coral',
              )}
            </Table.Header>
            <Table.Body>
              {#each sortedRuleRows as { row, i }}
                <Table.Row class="border-brutal-border border-b-[3px]">
                  {#if editingRuleIndex === i}
                    <Table.Cell class="px-2 py-2 md:px-3" colspan={2}>
                      <div class="flex flex-col gap-2">
                        <div class="flex items-center gap-2">
                          <Input
                            type="text"
                            variant="brutal"
                            bind:value={editRulePattern}
                            class="bg-card-surface focus-visible:border-hister-coral h-8 flex-1 px-2 text-sm"
                            onkeydown={(e) => {
                              if (e.key === 'Enter') saveEditRule();
                              if (e.key === 'Escape') cancelEditRule();
                            }}
                          />
                          <select
                            value={editRuleType}
                            onchange={(event) => {
                              editRuleType = (event.currentTarget as HTMLSelectElement)
                                .value as typeof editRuleType;
                              if (editRuleType !== 'skip') editDeleteMatchingDocuments = false;
                            }}
                            class="bg-card-surface border-brutal-border font-space text-text-brand h-8 w-20 shrink-0 cursor-pointer appearance-none border-[3px] px-2 text-center text-xs font-bold tracking-[0.5px] outline-none md:w-25 md:px-3"
                          >
                            <option value="skip">SKIP</option>
                            <option value="priority">PRIORITY</option>
                            <option value="versioning">VERSION</option>
                          </select>
                        </div>
                        {#if editRuleType === 'skip'}
                          <DeleteMatchingDocumentsOption
                            bind:checked={editDeleteMatchingDocuments}
                          />
                        {/if}
                      </div>
                    </Table.Cell>
                    <Table.Cell class="w-16 px-1 py-2 md:w-20 md:px-3">
                      {@render editCancelButtons(saveEditRule, cancelEditRule)}
                    </Table.Cell>
                  {:else}
                    <Table.Cell
                      class="font-fira text-text-brand max-w-0 truncate px-2 py-3 text-sm md:px-5"
                      >{row.pattern}</Table.Cell
                    >
                    <Table.Cell class="w-20 px-2 py-3 md:w-28 md:px-5">
                      <Badge
                        variant="default"
                        class="font-space border-0 px-2 py-1 text-xs font-bold tracking-[0.5px] uppercase md:px-3 {row.type ===
                        'skip'
                          ? 'bg-hister-rose text-white'
                          : row.type === 'priority'
                            ? 'bg-hister-teal text-white'
                            : 'bg-violet-500 text-white'}"
                      >
                        {row.type}
                      </Badge>
                    </Table.Cell>
                    <Table.Cell class="w-16 px-1 py-3 md:w-20 md:px-3">
                      {@render editDeleteButtons(
                        () => startEditRule(i),
                        () => removeRule(row.pattern, row.type),
                        'hover:text-hister-coral',
                      )}
                    </Table.Cell>
                  {/if}
                </Table.Row>
              {/each}
            </Table.Body>
          </Table.Root>

          {#if sortedRuleRows.length === 0}
            <div class="flex flex-col items-center justify-center gap-3 py-10">
              <div
                class="flex h-12 w-12 items-center justify-center"
                style="background-color: color-mix(in srgb, var(--hister-coral) 10%, transparent); color: var(--hister-coral);"
              >
                <Shield class="size-5" />
              </div>
              <p class="font-inter text-text-brand-muted text-sm">
                {ruleFilter ? 'No rules match the filter.' : 'No rules defined yet.'}
              </p>
            </div>
          {/if}
        </Card.Content>
      </Card.Root>
    </div>
  {/if}
</div>

<BulkRulesDialog
  bind:open={bulkAddOpen}
  bind:patterns={bulkRulePatterns}
  bind:ruleType={newRuleType}
  bind:deleteMatches={bulkDeleteMatchingDocuments}
  {saving}
  newCount={bulkRuleSummary.uniquePatterns.length}
  duplicateCount={bulkRuleSummary.duplicateCount}
  onAdd={addBulkRules}
/>

<Dialog.Root bind:open={deleteConfirmOpen}>
  <Dialog.Content
    escapeKeydownBehavior="ignore"
    onInteractOutside={(event) => event.preventDefault()}
    showCloseButton={false}
    class="border-border-brand bg-card-surface max-w-md gap-0 overflow-hidden rounded-none border-[3px] p-0 shadow-[6px_6px_0px_var(--brutal-shadow)]"
  >
    <Dialog.Header class="bg-hister-rose flex-row items-center gap-2 px-5 py-4">
      <Dialog.Title class="flex items-center gap-2">
        <Trash2 class="size-5 text-white" />
        <span class="font-outfit text-lg font-extrabold text-white">Delete matching documents?</span
        >
      </Dialog.Title>
    </Dialog.Header>
    <div class="space-y-3 px-5 py-5">
      <p class="font-inter text-text-brand-secondary text-sm">
        {pendingDocumentDeletion?.matched ?? 0} existing document{pendingDocumentDeletion?.matched ===
        1
          ? ''
          : 's'} match the saved skip
        {pendingDocumentDeletion?.patterns.length === 1 ? 'rule' : 'rules'}.
      </p>
      <p class="font-inter text-text-brand-muted text-xs">
        This permanently deletes the matching documents from the index. The saved skip
        {pendingDocumentDeletion?.patterns.length === 1 ? 'rule' : 'rules'} will remain if you cancel.
      </p>
    </div>
    <Dialog.Footer class="border-border-brand-muted bg-muted-surface border-t-[3px] px-5 py-3">
      <Button
        type="button"
        variant="outline"
        disabled={deletingMatchingDocuments}
        onclick={cancelMatchingDocumentDeletion}
        class="border-border-brand-muted text-text-brand-secondary rounded-none"
      >
        Cancel
      </Button>
      <Button
        type="button"
        disabled={deletingMatchingDocuments}
        onclick={confirmMatchingDocumentDeletion}
        class="bg-hister-rose font-space border-brutal-border rounded-none border-[3px] text-xs font-bold text-white uppercase"
      >
        {deletingMatchingDocuments
          ? 'Deleting…'
          : `Delete ${pendingDocumentDeletion?.matched ?? 0} document${pendingDocumentDeletion?.matched === 1 ? '' : 's'}`}
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
