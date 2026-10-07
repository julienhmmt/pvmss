<script lang="ts">
	/**
	 * CatalogListStates - the state ladder every admin catalog page renders:
	 * loading skeleton → load error → optional toggle error → empty catalog
	 * (action: go to clusters) → no filter match (action: reset filters) →
	 * the table itself. Six pages hand-rolled the same ladder; the per-kind
	 * differences are only labels and counts, passed as props.
	 */
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { Snippet } from 'svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		loading: boolean;
		error: string | null;
		/** Store-level toggle failure - shown above the table, non-fatal. */
		toggleError?: string | null;
		/** Skeleton column count. */
		columns: number;
		/** Unfiltered row count - 0 means the catalog itself is empty. */
		totalCount: number;
		/** Row count after filters - 0 (with rows present) means no match. */
		filteredCount: number;
		/** Screen-reader announcement when the list renders ("N items loaded"). */
		loadedLabel: string;
		emptyTitle: string;
		emptyDescription: string;
		emptyActionLabel: string;
		noMatchTitle: string;
		noMatchDescription: string;
		resetLabel: string;
		onResetFilters: () => void;
		/** The TableCard (toolbar + table) rendered when rows exist. */
		children: Snippet;
	}

	let {
		loading,
		error,
		toggleError = null,
		columns,
		totalCount,
		filteredCount,
		loadedLabel,
		emptyTitle,
		emptyDescription,
		emptyActionLabel,
		noMatchTitle,
		noMatchDescription,
		resetLabel,
		onResetFilters,
		children
	}: Props = $props();
</script>

{#if loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton {columns} />
{:else if error}
	<Alert>{error}</Alert>
{:else}
	<div role="status" aria-live="polite" class="sr-only">{loadedLabel}</div>

	{#if toggleError}
		<Alert class="mb-4">{toggleError}</Alert>
	{/if}

	{#if totalCount === 0}
		<EmptyState title={emptyTitle} description={emptyDescription}>
			{#snippet actions()}
				<Button variant="secondary" size="sm" onclick={() => goto(resolve('/admin/clusters'))}>
					{emptyActionLabel}
				</Button>
			{/snippet}
		</EmptyState>
	{:else if filteredCount === 0}
		<EmptyState title={noMatchTitle} description={noMatchDescription}>
			{#snippet actions()}
				<Button variant="secondary" size="sm" onclick={onResetFilters}>
					{resetLabel}
				</Button>
			{/snippet}
		</EmptyState>
	{:else}
		{@render children()}
	{/if}
{/if}
