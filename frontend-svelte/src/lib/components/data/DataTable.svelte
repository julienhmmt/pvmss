<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';
	import * as Table from '$lib/components/ui/table';
	import EmptyState from './EmptyState.svelte';
	import LoadingSkeleton from './LoadingSkeleton.svelte';
	import { Package } from 'phosphor-svelte';

	interface Column<T> {
		key: string;
		label: string;
		sortable?: boolean;
		render?: Snippet<[T]>;
	}

	interface Props {
		data: T[];
		columns: Column<T>[];
		loading?: boolean;
		emptyMessage?: string;
		onRowClick?: (row: T) => void;
	}

	let { data, columns, loading = false, emptyMessage = 'No data', onRowClick }: Props = $props();

	let sortKey = $state('');
	let sortAsc = $state(true);

	const sorted = $derived.by(() => {
		if (!sortKey) return data;
		return [...data].sort((a, b) => {
			const aVal = (a as Record<string, unknown>)[sortKey];
			const bVal = (b as Record<string, unknown>)[sortKey];
			if (aVal == null || bVal == null) return 0;
			const cmp = aVal < bVal ? -1 : aVal > bVal ? 1 : 0;
			return sortAsc ? cmp : -cmp;
		});
	});

	function toggleSort(key: string) {
		if (sortKey === key) {
			sortAsc = !sortAsc;
		} else {
			sortKey = key;
			sortAsc = true;
		}
	}
</script>

{#if loading}
	<LoadingSkeleton rows={5} variant="table" />
{:else if data.length === 0}
	<EmptyState title={emptyMessage} icon={Package} />
{:else}
	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					{#each columns as col}
						<Table.Head>
							{#if col.sortable}
								<button
									class="flex items-center gap-1 hover:text-foreground"
									onclick={() => toggleSort(col.key)}
								>
									{col.label}
									{#if sortKey === col.key}
										<span class="text-xs">{sortAsc ? '↑' : '↓'}</span>
									{/if}
								</button>
							{:else}
								{col.label}
							{/if}
						</Table.Head>
					{/each}
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each sorted as row}
					<Table.Row
						class={onRowClick ? 'cursor-pointer hover:bg-muted/50' : ''}
						onclick={() => onRowClick?.(row)}
					>
						{#each columns as col}
							<Table.Cell>
								{#if col.render}
									{@render col.render(row)}
								{:else}
									{(row as Record<string, unknown>)[col.key] ?? ''}
								{/if}
							</Table.Cell>
						{/each}
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}
