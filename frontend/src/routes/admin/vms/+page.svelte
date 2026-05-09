<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import PvHeader from '$lib/components/layout/PvHeader.svelte';
	import PvHeaderStat from '$lib/components/layout/PvHeaderStat.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import StatusBadge from '$lib/components/data/StatusBadge.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Select from '$lib/components/ui/select';
	import { getAllVMsPaginated, vmAction, deleteVM } from '$lib/api/admin/vms';
	import { getNodes } from '$lib/api/admin/nodes';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { formatBytes, formatCpu, formatUptime, formatPercent } from '$lib/utils/format';
	import { Desktop, ArrowsClockwise, SortAscending, SortDescending } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { VM, VMAction } from '$lib/types/admin';

	const AUTO_REFRESH_INTERVAL = 30_000;

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let vms = $state<VM[]>([]);
	let selectedNode = $state<string>('');
	let page = $state(1);
	let perPage = $state(25);
	let totalVMs = $state(0);
	let totalPages = $state(1);
	let hasNext = $state(false);
	let hasPrev = $state(false);
	let searchQuery = $state('');
	let searchTimeout: ReturnType<typeof setTimeout>;
	let runningTotal = $state(0);
	let stoppedTotal = $state(0);
	let nodeNames = $state<string[]>([]);
	let loadAbort: AbortController | null = null;
	let sortBy = $state<string>('vmid');
	let sortOrder = $state<string>('asc');

	let deleteTarget = $state<VM | null>(null);
	let deleting = $state(false);

	const runningCount = $derived(runningTotal);
	const stoppedCount = $derived(stoppedTotal);

	const startIndex = $derived((page - 1) * perPage + 1);
	const endIndex = $derived(Math.min(page * perPage, totalVMs));

	function parseTags(tags: string): string[] {
		if (!tags) return [];
		return tags.split(';').filter((tag) => tag.trim().length > 0);
	}

	function usageBarClass(percent: number): string {
		if (percent >= 80) return 'pv-usage-bar-fill--danger';
		if (percent >= 60) return 'pv-usage-bar-fill--warn';
		return '';
	}

	async function load(isRefresh = false) {
		// Cancel any in-flight request to prevent race conditions.
		if (loadAbort) loadAbort.abort();
		const abort = new AbortController();
		loadAbort = abort;

		if (isRefresh) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			const res = await getAllVMsPaginated({
				page,
				limit: perPage,
				search: searchQuery || undefined,
				node: selectedNode || undefined,
				sortBy: sortBy,
				sortOrder: sortOrder,
			});
			if (abort.signal.aborted) return;
			vms = res.vms;
			totalVMs = res.pagination.total;
			totalPages = res.pagination.totalPages;
			hasNext = res.pagination.hasNext;
			hasPrev = res.pagination.hasPrev;
			runningTotal = res.pagination.runningCount;
			stoppedTotal = res.pagination.stoppedCount;
		} catch (e) {
			if (abort.signal.aborted) return;
			error = e as Error;
		} finally {
			if (!abort.signal.aborted) {
				loading = false;
				refreshing = false;
			}
			if (loadAbort === abort) loadAbort = null;
		}
	}

	function onSearchInput(e: Event): void {
		clearTimeout(searchTimeout);
		searchQuery = (e.target as HTMLInputElement).value;
		searchTimeout = setTimeout(() => {
			page = 1;
			load();
		}, 300);
	}

	function onNodeChange(value: string): void {
		selectedNode = value;
		page = 1;
		load();
	}

	function onSortChange(value: string): void {
		sortBy = value;
		page = 1;
		load();
	}

	function toggleSortOrder(): void {
		sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
		page = 1;
		load();
	}

	function onPerPageChange(v: string | undefined): void {
		if (!v) return;
		perPage = Number(v);
		page = 1;
		load();
	}

	async function doAction(vm: VM, action: VMAction) {
		try {
			await vmAction(vm.vmid, vm.node, action);
			toast.success($t('admin.vms.toast.actionSent', { values: { action, vmid: vm.vmid } }));
			await load(true);
		} catch (e) {
			toast.error(
				$t('admin.vms.toast.actionFailed', {
					values: { action, vmid: vm.vmid, error: (e as Error).message }
				})
			);
		}
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		const vm = deleteTarget;
		deleting = true;
		try {
			await deleteVM(vm.vmid);
			toast.success($t('admin.vms.toast.deleteSuccess', { values: { name: vm.name, vmid: vm.vmid } }));
			deleteTarget = null;
			await load(true);
		} catch (e) {
			toast.error($t('admin.vms.toast.deleteFailed', { values: { vmid: vm.vmid, error: (e as Error).message } }));
		} finally {
			deleting = false;
		}
	}

	onMount(() => {
		load();

		// Fetch node names for the dropdown filter.
		getNodes()
			.then((nodes) => { nodeNames = nodes.map((n) => n.name).sort(); })
			.catch(() => {});

		const autoRefreshTimer = setInterval(() => {
			if (!document.hidden && !refreshing && !loading) {
				load(true);
			}
		}, AUTO_REFRESH_INTERVAL);

		return () => {
			clearInterval(autoRefreshTimer);
			clearTimeout(searchTimeout);
			if (loadAbort) loadAbort.abort();
		};
	});
</script>

<svelte:head>
	<title>PVMSS — {$t('admin.vms.title')}</title>
</svelte:head>

<PvHeader title={$t('admin.vms.title')}>
	{#snippet stats()}
		{#if !loading}
			<PvHeaderStat label={$t('admin.vms.title')} value={totalVMs} />
			{#if runningCount > 0}
				<PvHeaderStat label={$t('common.statusMap.running')} value={runningCount} />
			{/if}
			{#if stoppedCount > 0}
				<PvHeaderStat label={$t('common.statusMap.stopped')} value={stoppedCount} tone="danger" />
			{/if}
		{/if}
	{/snippet}
	{#snippet actions()}
		{#if !loading}
			<Button
				class="pv-header-btn"
				variant="outline"
				size="sm"
				onclick={() => load(true)}
				disabled={loading || refreshing}
			>
				<ArrowsClockwise class="mr-1 h-4 w-4 {refreshing ? 'animate-spin' : ''}" />
				{$t('common.refresh')}
			</Button>
		{/if}
	{/snippet}
</PvHeader>

<div class="pv-content-width">

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={() => load()} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={8} />
{:else if totalVMs === 0 && !loading}
	{#if searchQuery || selectedNode}
		<EmptyState title={$t('admin.vms.noSearchResults')} icon={Desktop} />
	{:else}
		<EmptyState title={$t('admin.vms.noVms')} icon={Desktop} />
	{/if}
{:else}
	<!-- Toolbar -->
	<div class="pv-toolbar">
		<div class="pv-toolbar-info">
			{totalVMs}
			{$t('admin.vms.title').toLowerCase()}
			{#if selectedNode}· {selectedNode}{/if}
		</div>
		<div class="flex items-center gap-2">
			<input
				type="search"
				class="h-8 px-3 text-sm border border-border rounded-md bg-background text-foreground outline-none focus:border-primary"
				placeholder={$t('common.search', { default: 'Search...' })}
				oninput={onSearchInput}
				value={searchQuery}
			/>
			<Select.Root type="single" value={selectedNode} onValueChange={onNodeChange}>
				<Select.Trigger class="h-8 text-sm" style="width: 140px;">
					{#if selectedNode}
						{selectedNode}
					{:else}
						{$t('admin.vms.allNodes')}
					{/if}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="">{$t('admin.vms.allNodes')}</Select.Item>
					{#each nodeNames as nodeName}
						<Select.Item value={nodeName}>{nodeName}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
			<Select.Root type="single" value={sortBy} onValueChange={onSortChange}>
				<Select.Trigger class="h-8 text-sm" style="width: 130px;">
					{$t('vms.sort.label')}: {$t(`vms.sort.${sortBy}`)}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="vmid">{$t('vms.sort.vmid')}</Select.Item>
					<Select.Item value="name">{$t('vms.sort.name')}</Select.Item>
					<Select.Item value="status">{$t('vms.sort.status')}</Select.Item>
					<Select.Item value="cpu">{$t('vms.sort.cpu')}</Select.Item>
					<Select.Item value="memory">{$t('vms.sort.memory')}</Select.Item>
				</Select.Content>
			</Select.Root>
			<button
				class="h-8 w-8 flex items-center justify-center border border-border rounded-md bg-background text-foreground hover:bg-accent transition-colors"
				onclick={toggleSortOrder}
				title={sortOrder === 'asc' ? $t('vms.sort.asc') : $t('vms.sort.desc')}
			>
				{#if sortOrder === 'asc'}
					<SortAscending class="h-4 w-4" />
				{:else}
					<SortDescending class="h-4 w-4" />
				{/if}
			</button>
		</div>
	</div>

	<div class="pv-table-wrap">
		<table class="pv-table">
			<thead>
				<tr>
					<th>{$t('admin.vms.vmid')}</th>
					<th>{$t('common.name')}</th>
					<th>{$t('common.status')}</th>
					<th>{$t('admin.vms.tags')}</th>
					<th>{$t('admin.vms.cpu')}</th>
					<th>{$t('admin.vms.ram')}</th>
					<th>{$t('admin.vms.uptime')}</th>
					<th>{$t('common.actions')}</th>
				</tr>
			</thead>
			<tbody>
				{#each vms as vm}
					{@const cpuPercent = Math.round(vm.cpu * 100)}
					{@const ramPercent = formatPercent(vm.mem, vm.maxMem)}
					{@const isRunning = vm.status === 'running'}
					<tr class="pv-row">
						<!-- VMID -->
						<td>
							<span class="pv-td-mono">{vm.vmid}</span>
						</td>

						<!-- Name + node -->
						<td>
							<div class="pv-resource-cell">
								<div class="pv-resource-icon pv-resource-icon--storage">
									{vm.name.slice(0, 2).toUpperCase()}
								</div>
								<div>
									<div class="pv-resource-name">{vm.name}</div>
									{#if !selectedNode}
										<div class="pv-td-muted text-xs">{vm.node}</div>
									{/if}
								</div>
							</div>
						</td>

						<!-- Status -->
						<td>
							<StatusBadge status={vm.status} />
						</td>

						<!-- Tags -->
						<td>
							<div class="flex flex-wrap gap-1">
								{#each parseTags(vm.tags) as tag}
									<span class="pv-action-badge pv-action-badge--vm text-[0.65rem]">{tag}</span>
								{/each}
							</div>
						</td>

						<!-- CPU -->
						<td>
							{#if isRunning}
								<div class="pv-usage-bar" style="min-width: 90px;">
									<div class="pv-usage-bar-track" style="flex: 1;">
										<div
											class="pv-usage-bar-fill {usageBarClass(cpuPercent)}"
											style="width: {cpuPercent}%"
										></div>
									</div>
									<span class="pv-usage-label">{formatCpu(vm.cpu)}</span>
								</div>
							{:else}
								<span class="pv-td-muted">—</span>
							{/if}
						</td>

						<!-- RAM -->
						<td>
							{#if isRunning && vm.maxMem > 0}
								<div class="pv-usage-bar" style="min-width: 100px;">
									<div class="pv-usage-bar-track" style="flex: 1;">
										<div
											class="pv-usage-bar-fill {usageBarClass(ramPercent)}"
											style="width: {ramPercent}%"
										></div>
									</div>
									<span class="pv-usage-label">{ramPercent}%</span>
								</div>
								<div class="pv-td-muted text-xs mt-0.5">
									{formatBytes(vm.mem)} / {formatBytes(vm.maxMem)}
								</div>
							{:else if vm.maxMem > 0}
								<span class="pv-td-muted text-xs">{formatBytes(vm.maxMem)}</span>
							{:else}
								<span class="pv-td-muted">—</span>
							{/if}
						</td>

						<!-- Uptime -->
						<td>
							{#if isRunning && vm.uptime > 0}
								<span class="text-sm tabular-nums">{formatUptime(vm.uptime)}</span>
							{:else}
								<span class="pv-td-muted">—</span>
							{/if}
						</td>

						<!-- Actions -->
						<td>
							<DropdownMenu.Root>
								<DropdownMenu.Trigger>
									{#snippet child({ props })}
										<Button variant="outline" size="sm" {...props}>
											{$t('common.actions')}
										</Button>
									{/snippet}
								</DropdownMenu.Trigger>
								<DropdownMenu.Content>
									{#if !isRunning}
										<DropdownMenu.Item onclick={() => doAction(vm, 'start')}>
											{$t('admin.vms.actions.start')}
										</DropdownMenu.Item>
									{:else}
										<DropdownMenu.Item onclick={() => doAction(vm, 'shutdown')}>
											{$t('admin.vms.actions.shutdown')}
										</DropdownMenu.Item>
										<DropdownMenu.Item onclick={() => doAction(vm, 'reboot')}>
											{$t('admin.vms.actions.reboot')}
										</DropdownMenu.Item>
										<DropdownMenu.Item onclick={() => doAction(vm, 'reset')}>
											{$t('admin.vms.actions.reset')}
										</DropdownMenu.Item>
										<DropdownMenu.Separator />
										<DropdownMenu.Item
											class="text-destructive"
											onclick={() => doAction(vm, 'stop')}
										>
											{$t('admin.vms.actions.forceStop')}
										</DropdownMenu.Item>
									{/if}
									<DropdownMenu.Separator />
									<DropdownMenu.Item
										class="text-destructive"
										onclick={() => { deleteTarget = vm; }}
									>
										{$t('admin.vms.actions.delete')}
									</DropdownMenu.Item>
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	<!-- Pagination -->
	<div class="flex items-center justify-between pt-4">
		<p class="text-sm text-muted-foreground">
			{$t('vms.pagination.showing', {
				values: { start: startIndex, end: endIndex, total: totalVMs }
			})}
		</p>

		<div class="flex items-center gap-4">
			<Select.Root
				type="single"
				value={String(perPage)}
				onValueChange={onPerPageChange}
			>
				<Select.Trigger class="w-[110px]">
					{$t('vms.pagination.perPage', { values: { count: perPage } })}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="10">10</Select.Item>
					<Select.Item value="25">25</Select.Item>
					<Select.Item value="50">50</Select.Item>
					<Select.Item value="100">100</Select.Item>
				</Select.Content>
			</Select.Root>

			<div class="flex items-center gap-2">
				<Button
					variant="outline"
					size="sm"
					disabled={!hasPrev}
					onclick={() => { page = page - 1; load(); }}
				>
					{$t('common.previous')}
				</Button>
				<span class="text-sm text-muted-foreground">
					{$t('common.pageOf', { values: { page, total: totalPages } })}
				</span>
				<Button
					variant="outline"
					size="sm"
					disabled={!hasNext}
					onclick={() => { page = page + 1; load(); }}
				>
					{$t('common.next')}
				</Button>
			</div>
		</div>
	</div>
{/if}

<!-- Delete confirmation dialog -->
<AlertDialog.Root open={!!deleteTarget} onOpenChange={(o) => { if (!o) deleteTarget = null; }}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{$t('vm.deleteConfirmTitle')}</AlertDialog.Title>
			<AlertDialog.Description>
				{#if deleteTarget}
					{$t('vm.deleteConfirmMessage', { values: { name: deleteTarget.name, vmid: deleteTarget.vmid } })}
					{#if deleteTarget.status === 'running'}
						<p class="mt-2 font-medium text-destructive">{$t('vm.deleteConfirmRunning')}</p>
					{/if}
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={deleting}>{$t('common.cancel')}</AlertDialog.Cancel>
			<AlertDialog.Action
				class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
				disabled={deleting}
				onclick={confirmDelete}
			>
				{deleting ? $t('common.loading') : $t('common.delete')}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

</div>
