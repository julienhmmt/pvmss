<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import StatusBadge from '$lib/components/data/StatusBadge.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Select from '$lib/components/ui/select';
	import { getAllVMs, vmAction, deleteVM } from '$lib/api/admin/vms';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { formatBytes, formatCpu, formatUptime, formatPercent } from '$lib/utils/format';
	import { Desktop, ArrowsClockwise } from 'phosphor-svelte';
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

	let deleteTarget = $state<VM | null>(null);
	let deleting = $state(false);

	const nodes = $derived([...new Set(vms.map((v) => v.node))].sort());

	const filteredVMs = $derived(selectedNode ? vms.filter((v) => v.node === selectedNode) : vms);

	const runningCount = $derived(filteredVMs.filter((v) => v.status === 'running').length);
	const stoppedCount = $derived(filteredVMs.filter((v) => v.status === 'stopped').length);

	const totalPages = $derived(Math.max(1, Math.ceil(filteredVMs.length / perPage)));
	const startIndex = $derived((page - 1) * perPage);
	const endIndex = $derived(Math.min(startIndex + perPage, filteredVMs.length));
	const paginatedVMs = $derived(filteredVMs.slice(startIndex, endIndex));

	function parseTags(tags: string): string[] {
		if (!tags) return [];
		return tags.split(';').filter((tag) => tag.trim().length > 0);
	}

	function usageBarClass(percent: number): string {
		if (percent >= 80) return 'pv-usage-bar-fill--danger';
		if (percent >= 60) return 'pv-usage-bar-fill--warn';
		return '';
	}

	async function load() {
		if (vms.length > 0) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			vms = await getAllVMs();
			page = 1;
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function doAction(vm: VM, action: VMAction) {
		try {
			await vmAction(vm.vmid, vm.node, action);
			toast.success($t('admin.vms.toast.actionSent', { values: { action, vmid: vm.vmid } }));
			await load();
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
			await load();
		} catch (e) {
			toast.error($t('admin.vms.toast.deleteFailed', { values: { vmid: vm.vmid, error: (e as Error).message } }));
		} finally {
			deleting = false;
		}
	}

	onMount(() => {
		load();

		const autoRefreshTimer = setInterval(() => {
			if (!document.hidden && !refreshing && !loading) {
				load();
			}
		}, AUTO_REFRESH_INTERVAL);

		return () => clearInterval(autoRefreshTimer);
	});
</script>

<!-- Gradient page header -->
<div class="pv-header -mx-6 -mt-6 mb-6">
	<div class="pv-header-flex">
		<div>
			<h1 class="pv-title">{$t('admin.vms.title')}</h1>
		</div>

		{#if !loading}
			<div class="flex items-center gap-3 flex-wrap">
				<div class="pv-header-stats">
					<div class="pv-header-stat">
						<div class="pv-header-stat-label">{$t('admin.vms.title')}</div>
						<div class="pv-header-stat-value">{filteredVMs.length}</div>
					</div>
					{#if runningCount > 0}
						<div class="pv-header-stat">
							<div class="pv-header-stat-label">{$t('common.statusMap.running')}</div>
							<div class="pv-header-stat-value">{runningCount}</div>
						</div>
					{/if}
					{#if stoppedCount > 0}
						<div class="pv-header-stat pv-header-stat--danger">
							<div class="pv-header-stat-label">{$t('common.statusMap.stopped')}</div>
							<div class="pv-header-stat-value">{stoppedCount}</div>
						</div>
					{/if}
				</div>
				<Button
					class="pv-header-btn"
					variant="outline"
					size="sm"
					onclick={load}
					disabled={loading || refreshing}
				>
					<ArrowsClockwise class="mr-1 h-4 w-4 {refreshing ? 'animate-spin' : ''}" />
					{$t('common.refresh')}
				</Button>
			</div>
		{/if}
	</div>
</div>

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={8} />
{:else if vms.length === 0}
	<EmptyState title={$t('admin.vms.noVms')} icon={Desktop} />
{:else}
	<!-- Toolbar -->
	<div class="pv-toolbar">
		<div class="pv-toolbar-info">
			{filteredVMs.length}
			{$t('admin.vms.title').toLowerCase()}
			{#if selectedNode}· {selectedNode}{/if}
		</div>
		{#if nodes.length > 1}
			<Select.Root type="single" value={selectedNode} onValueChange={(v) => { selectedNode = v ?? ''; page = 1; }}>
				<Select.Trigger class="w-[180px] h-8 text-sm">
					{selectedNode || $t('admin.vms.allNodes')}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="">{$t('admin.vms.allNodes')}</Select.Item>
					{#each nodes as node}
						<Select.Item value={node}>{node}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		{/if}
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
				{#each paginatedVMs as vm}
					{@const cpuPercent = Math.round(vm.cpu * 100)}
					{@const ramPercent = formatPercent(vm.mem, vm.maxmem)}
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
									{#if nodes.length > 1 || !selectedNode}
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
							{#if isRunning && vm.maxmem > 0}
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
									{formatBytes(vm.mem)} / {formatBytes(vm.maxmem)}
								</div>
							{:else if vm.maxmem > 0}
								<span class="pv-td-muted text-xs">{formatBytes(vm.maxmem)}</span>
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
			{$t('admin.vms.pagination.showing', {
				values: { start: startIndex + 1, end: endIndex, total: filteredVMs.length }
			})}
		</p>

		<div class="flex items-center gap-4">
			<Select.Root
				type="single"
				value={String(perPage)}
				onValueChange={(v) => {
					perPage = Number(v);
					page = 1;
				}}
			>
				<Select.Trigger class="w-[110px]">
					{$t('admin.vms.pagination.perPage', { values: { count: perPage } })}
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
					disabled={page <= 1}
					onclick={() => { page = Math.max(1, page - 1); }}
				>
					{$t('common.previous')}
				</Button>
				<span class="text-sm text-muted-foreground">
					{$t('common.pageOf', { values: { page, total: totalPages } })}
				</span>
				<Button
					variant="outline"
					size="sm"
					disabled={page >= totalPages}
					onclick={() => { page = Math.min(totalPages, page + 1); }}
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
