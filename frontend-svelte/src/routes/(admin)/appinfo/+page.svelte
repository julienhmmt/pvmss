<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
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

<PageHeader title={$t('admin.appinfo.title')} icon={Info} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="card" rows={4} />
{:else if info}
	<div class="space-y-8">
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
			<ResourceCard title={$t('admin.appinfo.version')} value={info.version} />
			<ResourceCard title={$t('admin.appinfo.environment')} value={info.environment} />
			<ResourceCard title={$t('admin.appinfo.goVersion')} value={info.go_version} />
			<ResourceCard title={$t('admin.appinfo.platform')} value={info.platform} />
			<ResourceCard
				title={$t('admin.appinfo.proxmox')}
				value={info.proxmox_connected ? $t('admin.appinfo.connected') : $t('admin.appinfo.disconnected')}
				subtitle={info.proxmox_url}
			/>
			<ResourceCard
				title={$t('admin.appinfo.offlineMode')}
				value={info.offline_mode ? $t('common.yes') : $t('common.no')}
			/>
			<ResourceCard title={$t('admin.appinfo.totalNodes')} value={String(info.total_nodes)} />
			<ResourceCard title={$t('admin.appinfo.totalVms')} value={String(info.total_vms)} />
		</div>

		{#if info.cluster_info}
			<section class="space-y-4">
				<h2 class="text-lg font-semibold">{$t('admin.appinfo.clusterInfo')}</h2>
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
					<ResourceCard
						title={$t('admin.appinfo.clusterMode')}
						value={info.cluster_info.is_cluster ? $t('admin.appinfo.clusterDetected') : $t('admin.appinfo.standaloneMode')}
					/>
					<ResourceCard title={$t('admin.appinfo.clusterName')} value={info.cluster_info.cluster_name || 'N/A'} />
					<ResourceCard title={$t('admin.appinfo.nodeCount')} value={String(info.cluster_info.node_count)} />
				</div>
			</section>
		{/if}

		{#if envVarEntries.length > 0}
			<section class="space-y-4">
				<h2 class="text-lg font-semibold">{$t('admin.appinfo.envVars')}</h2>
				<div class="rounded-md border">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>{$t('admin.appinfo.variable')}</Table.Head>
								<Table.Head>{$t('common.value')}</Table.Head>
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
