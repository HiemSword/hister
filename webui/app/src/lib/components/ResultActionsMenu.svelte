<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
  import type { ResultState } from '$lib/result-state.svelte';
  import { SkipRuleActions } from '@hister/components';
  import { Input } from '@hister/components/ui/input';
  import { Button } from '@hister/components/ui/button';
  import * as DropdownMenu from '@hister/components/ui/dropdown-menu';
  import { MoreVertical, Pin, PinOff, Tag, Trash2, Unlink } from '@lucide/svelte';

  interface Props {
    url: string;
    title: string;
    domain: string;
    resultState: ResultState;
    query: string;
    pinned?: boolean;
    historyResult?: boolean;
    canWrite?: boolean;
    onForget?: () => void;
    onDelete?: () => void;
    removeResult: (url: string) => void;
    removeResultsByDomain: (domain: string) => void;
  }

  let {
    url,
    title,
    domain,
    resultState,
    query,
    pinned = false,
    historyResult = false,
    canWrite = true,
    onForget,
    onDelete,
    removeResult,
    removeResultsByDomain,
  }: Props = $props();

  let open = $state(false);
</script>

{#if canWrite}
  <DropdownMenu.Root
    bind:open
    onOpenChange={(isOpen) => {
      if (!isOpen) return;
      resultState.onOpen();
    }}
  >
    <DropdownMenu.Trigger>
      {#snippet child({ props })}
        <Button
          {...props}
          variant="ghost"
          size="icon-sm"
          class="text-text-brand-muted hover:text-text-brand shrink-0 cursor-pointer self-start"
        >
          <MoreVertical class="size-4" />
        </Button>
      {/snippet}
    </DropdownMenu.Trigger>
    <DropdownMenu.Content
      class="border-brutal-border bg-card-surface {pinned
        ? 'w-72'
        : 'w-100'} rounded-none border-[3px] p-3 shadow-[4px_4px_0_var(--brutal-shadow)]"
    >
      <div class="space-y-3">
        <div class="space-y-2">
          {#if pinned}
            <p
              class="font-outfit text-text-brand-muted mb-1 text-xs font-bold tracking-widest uppercase"
            >
              Priority
            </p>
            <Button
              variant="outline"
              size="sm"
              class="border-hister-rose text-hister-rose hover:bg-hister-rose/10 w-full border-[2px] text-xs"
              onclick={() => resultState.pin(url, title, query, true)}
            >
              <PinOff class="size-3.5" />
              Unpin
            </Button>
          {:else}
            <p class="font-outfit mb-1 text-xs font-bold tracking-widest uppercase">
              Prioritize this result in query:
            </p>
            <div class="flex items-center gap-2">
              <Input
                bind:value={resultState.actionsQuery}
                placeholder="Query.."
                size="sm"
                class="font-inter border-border-brand-muted focus-visible:border-hister-indigo flex-1 border-[2px] text-sm shadow-none focus-visible:ring-0"
              />
              <Button
                variant="outline"
                size="sm"
                class="border-hister-indigo text-hister-indigo border-[2px] text-xs"
                onclick={() => resultState.pin(url, title, query)}
              >
                <Pin class="size-3.5" />
                Pin
              </Button>
            </div>
            <hr />
          {/if}
          {#if historyResult}
            <p class="font-inter text-text-brand-muted text-xs">
              Stops prioritizing this result for “{query}”. The document remains indexed.
            </p>
            <Button
              variant="outline"
              size="sm"
              class="border-hister-rose text-hister-rose hover:bg-hister-rose/10 w-full border-[2px] text-xs"
              onclick={async () => {
                if (await resultState.forgetForQuery(url, query)) {
                  open = false;
                  onForget?.();
                }
              }}
            >
              <Unlink class="size-3.5" />
              Forget for this query
            </Button>
          {/if}
        </div>
        <SkipRuleActions
          onAddSkipRule={async (type, deleteMatches) => {
            await resultState.addSkipRule(
              url,
              domain,
              type,
              deleteMatches,
              removeResult,
              removeResultsByDomain,
            );
            if (deleteMatches) open = false;
          }}
        />
        <hr />
        <div class="space-y-2">
          <p class="font-outfit mb-1 text-xs font-bold tracking-widest uppercase">Label:</p>
          <div class="flex items-center gap-2">
            <Input
              bind:value={resultState.labelInput}
              placeholder="Add a label…"
              size="sm"
              class="font-inter border-border-brand-muted focus-visible:border-hister-amber flex-1 border-[2px] text-sm shadow-none focus-visible:ring-0"
            />
            <Button
              variant="outline"
              size="sm"
              class="border-[2px] text-xs"
              onclick={() => resultState.updateLabel(url)}
            >
              <Tag class="size-3.5" />
              Save
            </Button>
          </div>
          {#if resultState.labelMessage}
            <p
              class="font-inter text-xs {resultState.labelError
                ? 'text-hister-rose'
                : 'text-hister-teal'}"
            >
              {resultState.labelMessage}
            </p>
          {/if}
        </div>
        {#if !pinned}
          <hr />
          <Button
            variant="outline"
            size="sm"
            class="border-hister-rose text-hister-rose hover:bg-hister-rose/10 w-full border-[2px] text-xs"
            onclick={() => {
              open = false;
              onDelete?.();
            }}
          >
            <Trash2 class="size-3.5" />
            Delete result
          </Button>
        {/if}
        {#if resultState.actionsMessage}
          <p
            class="font-inter text-xs {resultState.actionsError
              ? 'text-hister-rose'
              : 'text-hister-teal'}"
          >
            {resultState.actionsMessage}
          </p>
        {/if}
      </div>
    </DropdownMenu.Content>
  </DropdownMenu.Root>
{/if}
