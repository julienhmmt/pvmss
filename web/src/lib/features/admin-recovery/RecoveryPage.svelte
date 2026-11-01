<script lang="ts">
	/**
	 * RecoveryPage - the one-shot assistant that re-adopts leftovers a previous
	 * PVMSS install left on the cluster: orphan pvmss-* pools, stray pvmss
	 * VMs outside any managed pool, and catalog tags this instance lost with
	 * its database. Nothing changes on the cluster: adoption only re-marks
	 * pools managed and recreates catalog tags with the suggested color.
	 */
	import type { RecoveryReport, RecoveryVM } from './recovery.svelte';
	import type { ClusterOption } from '#lib/shared/clusters.js';
	import { formatDateTime } from '#lib/shared/format-date.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import Checkbox from '#lib/shared/ui/Checkbox.svelte';
	import ClusterSelector from '#lib/shared/ui/ClusterSelector.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import TableSkeleton from '#lib/shared/ui/TableSkeleton.svelte';
	import MachineStatusPill from '#lib/features/vms/MachineStatusPill.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		report: RecoveryReport | null;
		loading: boolean;
		rescanning: boolean;
		error: string | null;
		adopting: boolean;
		adoptError: string | null;
		announce: string | null;
		clusterOptions: ClusterOption[];
		cluster: string;
		selectedPools: ReadonlySet<string>;
		selectedTags: ReadonlySet<string>;
		onClusterChange: (value: string) => void;
		onRescan: () => void;
		onTogglePool: (name: string) => void;
		onToggleTag: (name: string) => void;
		onAdopt: () => Promise<void>;
	}

	let {
		report,
		loading,
		rescanning,
		error,
		adopting,
		adoptError,
		announce,
		clusterOptions,
		cluster,
		selectedPools,
		selectedTags,
		onClusterChange,
		onRescan,
		onTogglePool,
		onToggleTag,
		onAdopt
	}: Props = $props();

	const hasAdoptables = $derived((report?.pools.length ?? 0) > 0 || (report?.tags.length ?? 0) > 0);
	const hasContent = $derived(hasAdoptables || (report?.looseVMs.length ?? 0) > 0);

	function poolHint(memberCount: number, comment: string): string {
		const count = m['admin.recovery.vmCount']({ count: memberCount });
		return comment ? `${count} - ${comment}` : count;
	}
</script>

<svelte:head>
	<title>{m['admin.recovery.title']()}</title>
</svelte:head>

<div class="sr-only" role="status" aria-live="polite">{announce ?? ''}</div>

<PageHeader title={m['admin.recovery.heading']()} eyebrow={m['chrome.adminNav.system']()} description={m['admin.recovery.description']()}>
	{#snippet actions()}
		<ClusterSelector options={clusterOptions} value={cluster} onChange={onClusterChange} id="recovery-cluster" />
		<Button variant="secondary" onclick={onRescan} disabled={loading || rescanning}>
			{rescanning ? m['admin.recovery.scanning']() : m['admin.recovery.rescan']()}
		</Button>
	{/snippet}
</PageHeader>

{#if loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={5} />
{:else if error}
	<Alert>{error}</Alert>
{:else if report}
	<div class="space-y-5">
		{#if !hasContent}
			<Card>
				<EmptyState title={m['admin.recovery.empty']()} description={m['admin.recovery.emptyHelp']()} />
			</Card>
		{/if}

		{#if report.pools.length > 0}
			<Card title={m['admin.recovery.poolsHeading']()} description={m['admin.recovery.poolsHelp']()} pad="none">
				<ul class="divide-y divide-border">
					{#each report.pools as pool (pool.name)}
						<li class="px-5 py-4">
							<Checkbox
								label={pool.name}
								checked={selectedPools.has(pool.name)}
								onToggle={() => onTogglePool(pool.name)}
								hint={poolHint(pool.members.length, pool.comment)}
							/>
							{#if pool.members.length > 0}
								<table class="pv-table pv-responsive-table mt-3">
									<caption class="sr-only">{m['admin.recovery.members']()}</caption>
									<thead>
										<tr>
											<th class="font-medium">{m['common.name']()}</th>
											<th class="font-medium">{m['common.node']()}</th>
											<th class="font-medium">{m['common.status']()}</th>
											<th class="font-medium">{m['admin.tags.title']()}</th>
										</tr>
									</thead>
									<tbody>
										{#each pool.members as vm (vm.vmid)}
											{@render vmRow(vm)}
										{/each}
									</tbody>
								</table>
							{/if}
						</li>
					{/each}
				</ul>
			</Card>
		{/if}

		{#if report.looseVMs.length > 0}
			<Card title={m['admin.recovery.looseHeading']()} description={m['admin.recovery.looseHelp']()} pad="none">
				<table class="pv-table pv-responsive-table">
					<caption class="sr-only">{m['admin.recovery.looseHeading']()}</caption>
					<thead>
						<tr>
							<th class="font-medium">{m['common.name']()}</th>
							<th class="font-medium">{m['common.node']()}</th>
							<th class="font-medium">{m['common.status']()}</th>
							<th class="font-medium">{m['admin.tags.title']()}</th>
						</tr>
					</thead>
					<tbody>
						{#each report.looseVMs as vm (vm.vmid)}
							{@render vmRow(vm)}
						{/each}
					</tbody>
				</table>
			</Card>
		{/if}

		{#if report.tags.length > 0}
			<Card title={m['admin.recovery.tagsHeading']()} description={m['admin.recovery.tagsHelp']()} pad="none">
				<ul class="divide-y divide-border">
					{#each report.tags as tag (tag.name)}
						<li class="flex items-center gap-3 px-5 py-3">
							<span class="h-3.5 w-3.5 shrink-0 rounded-full border border-border" style="background-color: {tag.color}"></span>
							<Checkbox
								label={tag.name}
								checked={selectedTags.has(tag.name)}
								onToggle={() => onToggleTag(tag.name)}
								hint={m['admin.recovery.vmCount']({ count: tag.vmCount })}
							/>
						</li>
					{/each}
				</ul>
			</Card>
		{/if}

		{#if hasAdoptables}
			<div class="flex flex-wrap items-center justify-end gap-3">
				<span class="text-xs text-muted-foreground">{m['admin.recovery.lastScan']({ time: formatDateTime(report.scannedAt) })}</span>
				{#if adoptError}
					<Alert class="w-full sm:w-auto">{adoptError}</Alert>
				{/if}
				<Button onclick={onAdopt} disabled={adopting || rescanning}>
					{adopting ? m['admin.recovery.adopting']() : m['admin.recovery.adopt']()}
				</Button>
			</div>
		{/if}
	</div>
{/if}

{#snippet vmRow(vm: RecoveryVM)}
	<tr>
		<td data-label={m['common.name']()}>
			<span class="font-mono font-medium">{vm.name}</span>
			<span class="text-xs text-muted-foreground">#{vm.vmid}</span>
		</td>
		<td class="text-muted-foreground" data-label={m['common.node']()}>{vm.node}</td>
		<td data-label={m['common.status']()}><MachineStatusPill status={vm.status} /></td>
		<td class="text-muted-foreground" data-label={m['admin.tags.title']()}>{vm.tags.join(', ') || ' - '}</td>
	</tr>
{/snippet}
