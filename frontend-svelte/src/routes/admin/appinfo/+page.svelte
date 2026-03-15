<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import ResourceCard from '$lib/components/data/ResourceCard.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { getAppInfo } from '$lib/api/admin/appinfo';
	import { Info } from 'phosphor-svelte';
	import type { AppInfo } from '$lib/types/admin';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let info = $state<AppInfo | null>(null);

	async function load() {
		loading = true;
		error = null;
		try {
			info = await getAppInfo();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	onMount(load);
</script>

<PageHeader title="App Info" icon={Info} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="card" rows={4} />
{:else if info}
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
		<ResourceCard title="Version" value={info.version} />
		<ResourceCard title="Environment" value={info.environment} />
		<ResourceCard
			title="Proxmox"
			value={info.proxmox_connected ? 'Connected' : 'Disconnected'}
			subtitle={info.proxmox_url}
		/>
		<ResourceCard
			title="Offline Mode"
			value={info.offline_mode ? 'Yes' : 'No'}
		/>
		<ResourceCard title="Total Nodes" value={String(info.total_nodes)} />
		<ResourceCard title="Total VMs" value={String(info.total_vms)} />
	</div>
{/if}
