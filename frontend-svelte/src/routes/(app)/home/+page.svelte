<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { getVMs, type VMSummary } from '$lib/api/vms';
	import { api } from '$lib/api/client';
	import { ArrowsClockwise, PlusSquare, Desktop, Play, Stop, ArrowCounterClockwise } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let vms = $state<VMSummary[]>([]);
	let actionLoading = $state<Record<number, boolean>>({});

	async function load(isRefresh = false) {
		if (isRefresh) refreshing = true;
		else loading = true;
		error = null;
		try {
			vms = await getVMs();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function doAction(vm: VMSummary, action: string) {
		actionLoading = { ...actionLoading, [vm.vmid]: true };
		try {
			await api.post(`/api/v1/vms/${vm.vmid}/action`, { action, node: vm.node });
			toast.success(`${action} sent to ${vm.name || vm.vmid}`);
			setTimeout(() => load(true), 2000);
		} catch {
			toast.error(`Failed to ${action} ${vm.name || vm.vmid}`);
		} finally {
			actionLoading = { ...actionLoading, [vm.vmid]: false };
		}
	}

	function statusClass(status: string) {
		if (status === 'running') return 'pv-badge--online';
		if (status === 'stopped') return 'pv-badge--offline';
		return 'pv-badge--warn';
	}

	function uptimeLabel(seconds: number): string {
		if (!seconds) return '—';
		const d = Math.floor(seconds / 86400);
		const h = Math.floor((seconds % 86400) / 3600);
		if (d > 0) return `${d}j ${h}h`;
		const m = Math.floor((seconds % 3600) / 60);
		return h > 0 ? `${h}h ${m}m` : `${m}m`;
	}

	onMount(() => load());
</script>

<div class="mx-auto max-w-5xl px-4 py-6">
	<!-- Header -->
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-2xl font-bold">{$t('user.home.title')}</h1>
		<div class="flex items-center gap-2">
			<Button href="/vm/create" size="sm">
				<PlusSquare class="mr-1.5 h-4 w-4" />
				{$t('nav.createVm')}
			</Button>
			<Button variant="outline" size="sm" onclick={() => load(true)} disabled={refreshing}>
				<ArrowsClockwise class="h-4 w-4 {refreshing ? 'animate-spin' : ''}" />
			</Button>
		</div>
	</div>

	{#if error}
		<ErrorBanner {error} onRetry={() => load()} />
	{:else if loading}
		<LoadingSkeleton variant="card" rows={4} />
	{:else if vms.length === 0}
		<div class="pv-table-wrap py-16 text-center text-muted-foreground">
			<Desktop class="mx-auto mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('vms.noVms')}</p>
		</div>
	{:else}
		<div class="pv-table-wrap">
			<table class="pv-table">
				<thead>
					<tr>
						<th>{$t('vms.vmid')}</th>
						<th>{$t('common.name')}</th>
						<th>{$t('common.status')}</th>
						<th>{$t('vms.cpu')}</th>
						<th>{$t('vms.ram')}</th>
						<th>{$t('vms.uptime')}</th>
						<th></th>
					</tr>
				</thead>
				<tbody>
					{#each vms as vm (vm.vmid)}
						{@const busy = actionLoading[vm.vmid] ?? false}
						<tr class="pv-row">
							<td class="pv-td-mono text-sm">{vm.vmid}</td>
							<td>
								<div class="pv-resource-cell">
									<div class="pv-resource-icon pv-resource-icon--vm text-[0.6rem]">VM</div>
									<span class="pv-resource-name">{vm.name || '—'}</span>
								</div>
							</td>
							<td>
								<span class="pv-badge {statusClass(vm.status)}">
									{$t(`common.statusMap.${vm.status}`, { default: vm.status })}
								</span>
							</td>
							<td class="tabular-nums text-sm">
								{#if vm.status === 'running'}
									<div class="pv-usage-bar w-24">
										<div class="pv-usage-bar-track" style="flex:1">
											<div class="pv-usage-bar-fill" style="width:{Math.round(vm.cpu * 100)}%"></div>
										</div>
										<span class="pv-usage-label">{Math.round(vm.cpu * 100)}%</span>
									</div>
								{:else}
									<span class="text-muted-foreground">—</span>
								{/if}
							</td>
							<td class="tabular-nums text-sm">
								{#if vm.max_mem_mb > 0}
									<div class="pv-usage-bar w-24">
										<div class="pv-usage-bar-track" style="flex:1">
											<div class="pv-usage-bar-fill" style="width:{Math.round((vm.mem_mb / vm.max_mem_mb) * 100)}%"></div>
										</div>
										<span class="pv-usage-label">{Math.round(vm.max_mem_mb / 1024)} GB</span>
									</div>
								{:else}
									<span class="text-muted-foreground">—</span>
								{/if}
							</td>
							<td class="pv-td-muted tabular-nums text-sm">{uptimeLabel(vm.uptime)}</td>
							<td>
								<div class="flex items-center gap-1">
									{#if vm.status === 'stopped'}
										<button
											class="pv-action-btn pv-action-btn--start"
											onclick={() => doAction(vm, 'start')}
											disabled={busy}
											title={$t('vms.actions.start')}
										>
											<Play class="h-3.5 w-3.5" weight="fill" />
										</button>
									{:else if vm.status === 'running'}
										<button
											class="pv-action-btn pv-action-btn--stop"
											onclick={() => doAction(vm, 'shutdown')}
											disabled={busy}
											title={$t('vms.actions.shutdown')}
										>
											<Stop class="h-3.5 w-3.5" weight="fill" />
										</button>
										<button
											class="pv-action-btn"
											onclick={() => doAction(vm, 'reboot')}
											disabled={busy}
											title={$t('vms.actions.reboot')}
										>
											<ArrowCounterClockwise class="h-3.5 w-3.5" />
										</button>
									{/if}
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<style>
	:global(.pv-action-btn) {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border-radius: 6px;
		border: 1px solid var(--border);
		background: transparent;
		color: var(--muted-foreground);
		cursor: pointer;
		transition: background 0.12s, color 0.12s, border-color 0.12s;
	}
	:global(.pv-action-btn:disabled) {
		opacity: 0.4;
		cursor: not-allowed;
	}
	:global(.pv-action-btn:hover:not(:disabled)) {
		background: var(--accent);
		color: var(--accent-foreground);
	}
	:global(.pv-action-btn--start:hover:not(:disabled)) {
		background: hsl(142 71% 45% / 0.15);
		border-color: hsl(142 71% 45% / 0.4);
		color: hsl(142 71% 35%);
	}
	:global(.pv-action-btn--stop:hover:not(:disabled)) {
		background: hsl(0 84% 60% / 0.15);
		border-color: hsl(0 84% 60% / 0.4);
		color: hsl(0 84% 50%);
	}
</style>
