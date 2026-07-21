<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import ResourceCard from '$lib/components/data/ResourceCard.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import PvHeader from '$lib/components/layout/PvHeader.svelte';
	import PvHeaderStat from '$lib/components/layout/PvHeaderStat.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { getAppInfo } from '$lib/api/admin/appinfo';
	import { CheckCircle, XCircle, HardDrives, Desktop } from 'phosphor-svelte';
	import type { AppInfo } from '$lib/types/admin';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let info = $state<AppInfo | null>(null);

	let envVarEntries = $derived(
		info?.envVars ? Object.entries(info.envVars).sort(([a], [b]) => a.localeCompare(b)) : []
	);

	async function load() {
		if (info !== null) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			info = await getAppInfo();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	onMount(load);
</script>

<svelte:head>
	<title>PVMSS — {$t('admin.appinfo.title')}</title>
</svelte:head>

<PvHeader
	eyebrow={$t('nav.administration')}
	title={$t('admin.appinfo.title')}
	subtitle={info ? `v${info.version} · ${info.environment}` : undefined}
>
	{#snippet stats()}
		{#if info}
			<PvHeaderStat label={$t('admin.appinfo.version')} value={info.version} />
			<PvHeaderStat label={$t('admin.appinfo.totalNodes')} value={info.totalNodes} />
			<PvHeaderStat label={$t('admin.appinfo.totalVMs')} value={info.totalVms} />
			<PvHeaderStat
				label="Proxmox"
				value={info.proxmoxConnected ? $t('admin.appinfo.connected') : $t('admin.appinfo.disconnected')}
				tone={info.proxmoxConnected ? 'default' : 'danger'}
			/>
		{/if}
	{/snippet}
</PvHeader>

<div class="pv-content-width">

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="card" rows={4} />
{:else if info}
	<div class="space-y-8">

		<!-- Runtime info -->
		<section>
			<p class="pv-section-title">
				<Desktop class="h-3.5 w-3.5" />
				{$t('admin.appinfo.title')}
			</p>
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
				<ResourceCard title={$t('admin.appinfo.version')} value={info.version} />
				<ResourceCard title={$t('admin.appinfo.environment')} value={info.environment} />
				<ResourceCard title={$t('admin.appinfo.goVersion')} value={info.goVersion} />
				<ResourceCard title={$t('admin.appinfo.platform')} value={info.platform} />
				<ResourceCard
					title={$t('admin.appinfo.offlineMode')}
					value={info.offlineMode ? $t('common.yes') : $t('common.no')}
				/>
			</div>
		</section>

		<!-- Proxmox connection -->
		<section>
			<p class="pv-section-title">
				<HardDrives class="h-3.5 w-3.5" />
				Proxmox
			</p>
			<div class="pv-table-wrap">
				<table class="pv-table">
					<tbody>
						<tr class="pv-row">
							<th>{$t('admin.appinfo.proxmox')}</th>
							<td>
								{#if info.proxmoxConnected}
									<span class="pv-badge--online flex items-center gap-1 w-fit">
										<CheckCircle class="h-3.5 w-3.5" />
										{$t('admin.appinfo.connected')}
									</span>
								{:else}
									<span class="pv-badge--offline flex items-center gap-1 w-fit">
										<XCircle class="h-3.5 w-3.5" />
										{$t('admin.appinfo.disconnected')}
									</span>
								{/if}
							</td>
						</tr>
						<tr class="pv-row">
							<th>{$t('common.url')}</th>
							<td><span class="pv-td-mono">{info.proxmoxUrl}</span></td>
						</tr>
						<tr class="pv-row">
							<th>{$t('admin.appinfo.totalNodes')}</th>
							<td class="font-medium tabular-nums">{info.totalNodes}</td>
						</tr>
						<tr class="pv-row">
							<th>{$t('admin.appinfo.totalVMs')}</th>
							<td class="font-medium tabular-nums">{info.totalVms}</td>
						</tr>
					</tbody>
				</table>
			</div>
		</section>

		<!-- Cluster info -->
		{#if info.clusterInfo}
			<section>
				<p class="pv-section-title">{$t('admin.appinfo.clusterInfo')}</p>
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
					<ResourceCard
						title={$t('admin.appinfo.clusterMode')}
						value={info.clusterInfo.isCluster
							? $t('admin.appinfo.clustered')
							: $t('admin.appinfo.standalone')}
					/>
					<ResourceCard
						title={$t('admin.appinfo.clusterName')}
						value={info.clusterInfo.clusterName || 'N/A'}
					/>
					<ResourceCard
						title={$t('admin.appinfo.nodeCount')}
						value={String(info.clusterInfo.nodeCount)}
					/>
				</div>
			</section>
		{/if}

		<!-- Environment variables -->
		{#if envVarEntries.length > 0}
			<section>
				<p class="pv-section-title">{$t('admin.appinfo.envVars')}</p>
				<div class="pv-table-wrap">
					<table class="pv-table">
						<thead>
							<tr>
								<th>{$t('admin.appinfo.variable')}</th>
								<th>{$t('common.value')}</th>
							</tr>
						</thead>
						<tbody>
							{#each envVarEntries as [key, value], i (i)}
								<tr class="pv-row">
									<td><span class="pv-td-mono">{key}</span></td>
									<td class="pv-td-muted">{value}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</section>
		{/if}
	</div>
{/if}

</div>
