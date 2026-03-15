<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import DataTable from '$lib/components/data/DataTable.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { getVMBRs } from '$lib/api/admin/vmbr';
	import { WifiHigh } from 'phosphor-svelte';
	import type { VMBR } from '$lib/types/admin';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let vmbrs = $state<VMBR[]>([]);

	async function load() {
		loading = true;
		error = null;
		try {
			vmbrs = await getVMBRs();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	onMount(load);

	const columns = [
		{ key: 'iface', label: 'Interface', sortable: true },
		{ key: 'type', label: 'Type', sortable: true },
		{ key: 'bridge_ports', label: 'Bridge Ports' },
		{ key: 'node', label: 'Node', sortable: true },
		{ key: 'active', label: 'Active' }
	];
</script>

<PageHeader title="Network Bridges" icon={WifiHigh} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else}
	<DataTable data={vmbrs} {columns} {loading} emptyMessage="No network bridges found" />
{/if}
