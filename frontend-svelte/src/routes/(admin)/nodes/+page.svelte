<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import ResourceCard from '$lib/components/data/ResourceCard.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import StatusBadge from '$lib/components/data/StatusBadge.svelte';
	import { getNodes } from '$lib/api/admin/nodes';
	import { formatBytes, formatCpu, formatUptime } from '$lib/utils/format';
	import { HardDrives } from 'phosphor-svelte';
	import type { Node } from '$lib/types/admin';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let nodes = $state<Node[]>([]);

	async function load() {
		loading = true;
		error = null;
		try {
			nodes = await getNodes();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	onMount(load);
</script>

<PageHeader title="Nodes" icon={HardDrives} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="card" rows={4} />
{:else if nodes.length === 0}
	<EmptyState title="No nodes found" icon={HardDrives} />
{:else}
	<div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
		{#each nodes as node}
			<div class="rounded-lg border p-4 space-y-3">
				<div class="flex items-center justify-between">
					<h3 class="font-semibold">{node.name}</h3>
					<StatusBadge status={node.status} />
				</div>
				<div class="grid grid-cols-3 gap-2 text-sm">
					<div>
						<p class="text-muted-foreground">CPU</p>
						<p class="font-medium">{formatCpu(node.cpu)}</p>
					</div>
					<div>
						<p class="text-muted-foreground">RAM</p>
						<p class="font-medium">{formatBytes(node.memory)} / {formatBytes(node.max_memory)}</p>
					</div>
					<div>
						<p class="text-muted-foreground">Uptime</p>
						<p class="font-medium">{formatUptime(node.uptime)}</p>
					</div>
				</div>
			</div>
		{/each}
	</div>
{/if}
