<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import ResourceCard from '$lib/components/data/ResourceCard.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { getNodes } from '$lib/api/admin/nodes';
	import { getStorages } from '$lib/api/admin/storage';
	import { getAllVMs } from '$lib/api/admin/vms';
	import { formatBytes } from '$lib/utils/format';
	import { House } from 'phosphor-svelte';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let nodeCount = $state(0);
	let vmCount = $state(0);
	let storageTotal = $state(0);
	let storageUsed = $state(0);

	async function load() {
		loading = true;
		error = null;
		try {
			const [nodes, vms, storages] = await Promise.all([
				getNodes(),
				getAllVMs(),
				getStorages()
			]);
			nodeCount = nodes.length;
			vmCount = vms.length;
			storageTotal = storages.reduce((s, x) => s + x.total, 0);
			storageUsed = storages.reduce((s, x) => s + x.used, 0);
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	onMount(load);
</script>

<PageHeader title="Dashboard" icon={House} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="card" rows={4} />
{:else}
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<ResourceCard title="Nodes" value={String(nodeCount)} subtitle="Active" />
		<ResourceCard title="Virtual Machines" value={String(vmCount)} subtitle="Total" />
		<ResourceCard
			title="Storage Used"
			value={formatBytes(storageUsed)}
			subtitle={`of ${formatBytes(storageTotal)}`}
		/>
		<ResourceCard
			title="Storage Free"
			value={formatBytes(storageTotal - storageUsed)}
			subtitle={storageTotal > 0
				? `${Math.round(((storageTotal - storageUsed) / storageTotal) * 100)}% available`
				: '0% available'}
		/>
	</div>
{/if}
