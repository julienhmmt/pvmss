<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import ResourceCard from '$lib/components/data/ResourceCard.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import * as Table from '$lib/components/ui/table';
	import { getAppInfo } from '$lib/api/admin/appinfo';
	import { Info } from 'phosphor-svelte';
	import type { AppInfo } from '$lib/types/admin';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let info = $state<AppInfo | null>(null);

	let envVarEntries = $derived(
		info?.env_vars ? Object.entries(info.env_vars).sort(([a], [b]) => a.localeCompare(b)) : []
	);

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
	<div class="space-y-8">
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
			<ResourceCard title="Version" value={info.version} />
			<ResourceCard title="Environment" value={info.environment} />
			<ResourceCard title="Go Version" value={info.go_version} />
			<ResourceCard title="Platform" value={info.platform} />
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

		{#if info.cluster_info}
			<section class="space-y-4">
				<h2 class="text-lg font-semibold">Cluster Information</h2>
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
					<ResourceCard
						title="Cluster Mode"
						value={info.cluster_info.is_cluster ? 'Clustered' : 'Standalone'}
					/>
					<ResourceCard title="Cluster Name" value={info.cluster_info.cluster_name || 'N/A'} />
					<ResourceCard title="Node Count" value={String(info.cluster_info.node_count)} />
				</div>
			</section>
		{/if}

		{#if envVarEntries.length > 0}
			<section class="space-y-4">
				<h2 class="text-lg font-semibold">Environment Variables</h2>
				<div class="rounded-md border">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>Variable</Table.Head>
								<Table.Head>Value</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each envVarEntries as [key, value]}
								<Table.Row>
									<Table.Cell class="font-mono text-sm">{key}</Table.Cell>
									<Table.Cell class="font-mono text-sm">{value}</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
			</section>
		{/if}
	</div>
{/if}
