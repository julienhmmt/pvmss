<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import { getStorages } from '$lib/api/admin/storage';
	import { formatBytes, formatPercent } from '$lib/utils/format';
	import { Database } from 'phosphor-svelte';
	import type { Storage } from '$lib/types/admin';
	import * as Table from '$lib/components/ui/table';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let storages = $state<Storage[]>([]);

	async function load() {
		loading = true;
		error = null;
		try {
			storages = await getStorages();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	onMount(load);
</script>

<PageHeader title="Storage" icon={Database} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if storages.length === 0}
	<EmptyState title="No storages found" icon={Database} />
{:else}
	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>Name</Table.Head>
					<Table.Head>Type</Table.Head>
					<Table.Head>Node</Table.Head>
					<Table.Head>Total</Table.Head>
					<Table.Head>Used</Table.Head>
					<Table.Head>Free</Table.Head>
					<Table.Head>Usage</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each storages as s}
					<Table.Row>
						<Table.Cell class="font-medium">{s.storage}</Table.Cell>
						<Table.Cell>{s.type}</Table.Cell>
						<Table.Cell>{s.node}</Table.Cell>
						<Table.Cell>{formatBytes(s.total)}</Table.Cell>
						<Table.Cell>{formatBytes(s.used)}</Table.Cell>
						<Table.Cell>{formatBytes(s.free)}</Table.Cell>
						<Table.Cell>
							<div class="flex items-center gap-2">
								<div class="h-2 w-20 rounded-full bg-muted">
									<div
										class="h-2 rounded-full bg-primary"
										style="width: {formatPercent(s.used, s.total)}%"
									></div>
								</div>
								<span class="text-xs">{formatPercent(s.used, s.total)}%</span>
							</div>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}
