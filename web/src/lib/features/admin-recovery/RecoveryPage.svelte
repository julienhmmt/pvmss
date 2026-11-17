<script lang="ts">
	/**
	 * RecoveryPage - the one-shot assistant that re-adopts leftovers a previous
	 * PVMSS install left on the cluster: orphan pvmss-* pools, stray pvmss
	 * VMs outside any managed pool, and catalog tags this instance lost with
	 * its database. Nothing changes on the cluster: adoption only re-marks
	 * pools managed and recreates catalog tags with the suggested color.
	 * Items the admin decides to leave alone can be hidden: they move to a
	 * collapsed section at the bottom where they can be unhidden or adopted.
	 */
	import type { RecoveryDismissalSelection, RecoveryPool, RecoveryReport, RecoveryTag, RecoveryVM } from './recovery.svelte';
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
	import TextField from '#lib/shared/ui/TextField.svelte';
	import EyeOffIcon from '#lib/shared/ui/icons/EyeOffIcon.svelte';
	import EyeIcon from '#lib/shared/ui/icons/EyeIcon.svelte';
	import MachineStatusPill from '#lib/features/vms/MachineStatusPill.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		report: RecoveryReport | null;
		/** The three visible lists, already filtered by `search`. */
		pools: RecoveryPool[];
		tags: RecoveryTag[];
		looseVMs: RecoveryVM[];
		search: string;
		allPoolsSelected: boolean;
		allTagsSelected: boolean;
		allVMsSelected: boolean;
		loading: boolean;
		rescanning: boolean;
		error: string | null;
		adopting: boolean;
		adoptError: string | null;
		hiding: boolean;
		hideError: string | null;
		announce: string | null;
		clusterOptions: ClusterOption[];
		cluster: string;
		selectedPools: ReadonlySet<string>;
		selectedTags: ReadonlySet<string>;
		selectedVMs: ReadonlySet<number>;
		onClusterChange: (value: string) => void;
		onRescan: () => void;
		onSearchChange: (value: string) => void;
		onToggleAllPools: () => void;
		onToggleAllTags: () => void;
		onToggleAllVMs: () => void;
		onTogglePool: (name: string) => void;
		onToggleTag: (name: string) => void;
		onToggleVM: (vmid: number) => void;
		onAdopt: () => Promise<void>;
		onHideSelected: () => Promise<void>;
		onHide: (items: RecoveryDismissalSelection) => Promise<void>;
		onUnhide: (items: RecoveryDismissalSelection) => Promise<void>;
		onAdoptHidden: (item: { pool?: string; tag?: string }) => Promise<void>;
	}

	let {
		report,
		pools,
		tags,
		looseVMs,
		search,
		allPoolsSelected,
		allTagsSelected,
		allVMsSelected,
		loading,
		rescanning,
		error,
		adopting,
		adoptError,
		hiding,
		hideError,
		announce,
		clusterOptions,
		cluster,
		selectedPools,
		selectedTags,
		selectedVMs,
		onClusterChange,
		onRescan,
		onSearchChange,
		onToggleAllPools,
		onToggleAllTags,
		onToggleAllVMs,
		onTogglePool,
		onToggleTag,
		onToggleVM,
		onAdopt,
		onHideSelected,
		onHide,
		onUnhide,
		onAdoptHidden
	}: Props = $props();

	const hasAdoptables = $derived(pools.length > 0 || tags.length > 0);
	const visibleCount = $derived(pools.length + tags.length + looseVMs.length);
	const hiddenCount = $derived(
		(report?.hiddenPools.length ?? 0) + (report?.hiddenTags.length ?? 0) + (report?.hiddenLooseVMs.length ?? 0)
	);
	const hasContent = $derived(visibleCount > 0 || hiddenCount > 0 || search.trim() !== '');
	const busy = $derived(adopting || hiding || rescanning);

	function poolHint(memberCount: number, comment: string): string {
		const count = m['admin.recovery.vmCount']({ count: memberCount });
		return comment ? `${count} - ${comment}` : count;
	}
</script>

<svelte:head>
	<title>{m['admin.recovery.title']()}</title>
</svelte:head>

<div class="sr-only" role="status" aria-live="polite">{announce ?? ''}</div>

<PageHeader cinematic title={m['admin.recovery.heading']()} eyebrow={m['chrome.adminNav.system']()} description={m['admin.recovery.description']()}>
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
		{:else}
			<TextField
				type="search"
				placeholder={m['admin.recovery.filterPlaceholder']()}
				aria-label={m['admin.recovery.filterPlaceholder']()}
				value={search}
				oninput={(e: Event & { currentTarget: HTMLInputElement }) => onSearchChange(e.currentTarget.value)}
				class="w-full sm:w-72"
			/>
			{#if visibleCount === 0 && search.trim() !== ''}
				<Card>
					<EmptyState title={m['admin.recovery.noMatch']({ query: search.trim() })} />
				</Card>
			{/if}
		{/if}

		{#if pools.length > 0}
			<Card title={m['admin.recovery.poolsHeading']()} description={m['admin.recovery.poolsHelp']()} pad="none">
				{#snippet actions()}
					<Checkbox label={m['admin.recovery.selectAll']()} checked={allPoolsSelected} onToggle={onToggleAllPools} />
				{/snippet}
				<ul class="divide-y divide-border">
					{#each pools as pool (pool.name)}
						<li class="px-5 py-4">
							<div class="flex items-start justify-between gap-2">
								<Checkbox
									label={pool.name}
									checked={selectedPools.has(pool.name)}
									onToggle={() => onTogglePool(pool.name)}
									hint={poolHint(pool.members.length, pool.comment)}
								/>
								<button
									type="button"
									class="mt-0.5 shrink-0 rounded text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:text-foreground"
									aria-label={m['admin.recovery.hideItem']({ name: pool.name })}
									disabled={busy}
									onclick={() => onHide({ pools: [pool.name] })}
								>
									<EyeOffIcon class="h-4 w-4" />
								</button>
							</div>
							{#if pool.members.length > 0}
								<details class="mt-3">
									<summary class="cursor-pointer text-xs text-muted-foreground">
										{m['admin.recovery.members']()} · {m['admin.recovery.vmCount']({ count: pool.members.length })}
									</summary>
									<table class="pv-table pv-responsive-table mt-2">
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
								</details>
							{/if}
						</li>
					{/each}
				</ul>
			</Card>
		{/if}

		{#if looseVMs.length > 0}
			<Card title={m['admin.recovery.looseHeading']()} description={m['admin.recovery.looseHelp']()} pad="none">
				{#snippet actions()}
					<Checkbox label={m['admin.recovery.selectAll']()} checked={allVMsSelected} onToggle={onToggleAllVMs} />
				{/snippet}
				<table class="pv-table pv-responsive-table">
					<caption class="sr-only">{m['admin.recovery.looseHeading']()}</caption>
					<thead>
						<tr>
							<th class="w-8"><span class="sr-only">{m['admin.recovery.select']()}</span></th>
							<th class="font-medium">{m['common.name']()}</th>
							<th class="font-medium">{m['common.node']()}</th>
							<th class="font-medium">{m['common.status']()}</th>
							<th class="font-medium">{m['admin.tags.title']()}</th>
							<th class="w-8"><span class="sr-only">{m['common.actions']()}</span></th>
						</tr>
					</thead>
					<tbody>
						{#each looseVMs as vm (vm.vmid)}
							<tr>
								<td>
									<input
										type="checkbox"
										class="h-4 w-4 rounded border-border accent-primary"
										checked={selectedVMs.has(vm.vmid)}
										aria-label={m['admin.recovery.selectItem']({ name: vm.name })}
										onchange={() => onToggleVM(vm.vmid)}
									/>
								</td>
								<td data-label={m['common.name']()}>
									<span class="font-mono font-medium">{vm.name}</span>
									<span class="text-xs text-muted-foreground">#{vm.vmid}</span>
								</td>
								<td class="text-muted-foreground" data-label={m['common.node']()}>{vm.node}</td>
								<td data-label={m['common.status']()}><MachineStatusPill status={vm.status} /></td>
								<td class="text-muted-foreground" data-label={m['admin.tags.title']()}>{vm.tags.join(', ') || ' - '}</td>
								<td>
									<button
										type="button"
										class="rounded text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:text-foreground"
										aria-label={m['admin.recovery.hideItem']({ name: vm.name })}
										disabled={busy}
										onclick={() => onHide({ vms: [vm.vmid] })}
									>
										<EyeOffIcon class="h-4 w-4" />
									</button>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</Card>
		{/if}

		{#if tags.length > 0}
			<Card title={m['admin.recovery.tagsHeading']()} description={m['admin.recovery.tagsHelp']()} pad="none">
				{#snippet actions()}
					<Checkbox label={m['admin.recovery.selectAll']()} checked={allTagsSelected} onToggle={onToggleAllTags} />
				{/snippet}
				<ul class="divide-y divide-border">
					{#each tags as tag (tag.name)}
						<li class="flex items-center gap-3 px-5 py-3">
							<span class="h-3.5 w-3.5 shrink-0 rounded-full border border-border" style="background-color: {tag.color}"></span>
							<Checkbox
								label={tag.name}
								checked={selectedTags.has(tag.name)}
								onToggle={() => onToggleTag(tag.name)}
								hint={m['admin.recovery.vmCount']({ count: tag.vmCount })}
							/>
							<button
								type="button"
								class="ml-auto shrink-0 rounded text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:text-foreground"
								aria-label={m['admin.recovery.hideItem']({ name: tag.name })}
								disabled={busy}
								onclick={() => onHide({ tags: [tag.name] })}
							>
								<EyeOffIcon class="h-4 w-4" />
							</button>
						</li>
					{/each}
				</ul>
			</Card>
		{/if}

		{#if hideError}
			<Alert>{hideError}</Alert>
		{/if}

		{#if visibleCount > 0}
			<div class="flex flex-wrap items-center justify-end gap-3">
				<span class="text-xs text-muted-foreground">{m['admin.recovery.lastScan']({ time: formatDateTime(report.scannedAt) })}</span>
				{#if adoptError}
					<Alert class="w-full sm:w-auto">{adoptError}</Alert>
				{/if}
				<Button variant="secondary" onclick={onHideSelected} disabled={busy}>
					{hiding ? m['admin.recovery.hiding']() : m['admin.recovery.hideSelected']()}
				</Button>
				{#if hasAdoptables}
					<Button onclick={onAdopt} disabled={busy}>
						{adopting ? m['admin.recovery.adopting']() : m['admin.recovery.adopt']()}
					</Button>
				{/if}
			</div>
		{/if}

		{#if hiddenCount > 0}
			<details class="rounded-xl border border-border bg-card text-card-foreground shadow-card">
				<summary class="cursor-pointer px-5 py-4">
					<span class="text-sm font-semibold tracking-tight text-foreground"
						>{m['admin.recovery.hiddenHeading']({ count: hiddenCount })}</span
					>
					<span class="ml-2 text-xs text-muted-foreground">{m['admin.recovery.hiddenHelp']()}</span>
				</summary>
				<ul class="divide-y divide-border border-t border-border">
					{#each report.hiddenPools as pool (pool.name)}
						<li class="flex items-center justify-between gap-3 px-5 py-3">
							<div class="min-w-0">
								<span class="font-mono font-medium">{pool.name}</span>
								<span class="ml-2 text-xs text-muted-foreground">{poolHint(pool.memberCount, pool.comment)}</span>
							</div>
							<div class="flex shrink-0 items-center gap-2">
								<Button variant="secondary" size="sm" onclick={() => onAdoptHidden({ pool: pool.name })} disabled={busy}>
									{m['admin.recovery.adoptItem']()}
								</Button>
								<button
									type="button"
									class="rounded text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:text-foreground"
									aria-label={m['admin.recovery.unhideItem']({ name: pool.name })}
									disabled={busy}
									onclick={() => onUnhide({ pools: [pool.name] })}
								>
									<EyeIcon class="h-4 w-4" />
								</button>
							</div>
						</li>
					{/each}
					{#each report.hiddenTags as tag (tag.name)}
						<li class="flex items-center justify-between gap-3 px-5 py-3">
							<div class="flex min-w-0 items-center gap-2">
								<span class="h-3.5 w-3.5 shrink-0 rounded-full border border-border" style="background-color: {tag.color}"></span>
								<span class="font-mono font-medium">{tag.name}</span>
								<span class="ml-2 text-xs text-muted-foreground">{m['admin.recovery.vmCount']({ count: tag.vmCount })}</span>
							</div>
							<div class="flex shrink-0 items-center gap-2">
								<Button variant="secondary" size="sm" onclick={() => onAdoptHidden({ tag: tag.name })} disabled={busy}>
									{m['admin.recovery.adoptItem']()}
								</Button>
								<button
									type="button"
									class="rounded text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:text-foreground"
									aria-label={m['admin.recovery.unhideItem']({ name: tag.name })}
									disabled={busy}
									onclick={() => onUnhide({ tags: [tag.name] })}
								>
									<EyeIcon class="h-4 w-4" />
								</button>
							</div>
						</li>
					{/each}
					{#each report.hiddenLooseVMs as vm (vm.vmid)}
						<li class="flex items-center justify-between gap-3 px-5 py-3">
							<div class="min-w-0">
								<span class="font-mono font-medium">{vm.name}</span>
								<span class="ml-2 text-xs text-muted-foreground">#{vm.vmid}</span>
							</div>
							<button
								type="button"
								class="shrink-0 rounded text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:text-foreground"
								aria-label={m['admin.recovery.unhideItem']({ name: vm.name })}
								disabled={busy}
								onclick={() => onUnhide({ vms: [vm.vmid] })}
							>
								<EyeIcon class="h-4 w-4" />
							</button>
						</li>
					{/each}
				</ul>
			</details>
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
