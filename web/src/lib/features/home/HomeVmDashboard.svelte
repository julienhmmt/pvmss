<script lang="ts">
	import { get } from '$lib/shared/api/client';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import type { VmListItem } from '$lib/features/vms/list.svelte';
	import { onMount } from 'svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';

	type DashboardVm = VmListItem;

	const session = getSessionContext();

	let vms = $state<DashboardVm[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let refreshCount = $state(0);

	async function load(): Promise<void> {
		loading = true;
		error = null;
		try {
			const result = await get<{ items: DashboardVm[] }>('/api/v1/vms?scope=mine&pageSize=20');
			vms = result.items;
		} catch {
			error = m['home.dashboard.loadError']();
		} finally {
			loading = false;
		}
	}

	function statusClass(status: string): string {
		if (status === 'running') return 'bg-success-soft text-success-soft-foreground';
		if (status === 'stopped') return 'bg-muted text-muted-foreground';
		return 'bg-destructive-soft text-destructive-soft-foreground';
	}

	let total = $derived(vms.length);
	let running = $derived(vms.filter((v) => v.status === 'running').length);
	let stopped = $derived(vms.filter((v) => v.status === 'stopped').length);
	let paused = $derived(vms.filter((v) => v.status === 'paused').length);

	onMount(() => {
		if (session.principal) void load();
	});
</script>

{#if session.principal}
<section class="w-full max-w-5xl rounded-xl border border-border bg-card p-5 shadow-sm" aria-labelledby="dashboard-title">
	<div class="mb-4 flex items-center justify-between">
		<h2 id="dashboard-title" class="text-lg font-semibold tracking-tight">{m['home.dashboard.heading']()}</h2>
		<button
			type="button"
			class="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted"
			onclick={() => { refreshCount = refreshCount + 1; void load(); }}
			disabled={loading}
			data-testid="dashboard-refresh"
		>
			{m['home.dashboard.refresh']()}
		</button>
	</div>

	{#if loading && vms.length === 0}
		<p role="status" aria-live="polite" class="py-6 text-center text-sm text-muted-foreground">{m['common.loading']()}</p>
	{:else if error}
		<p role="alert" class="py-6 text-center text-sm text-destructive">{error}</p>
	{:else if vms.length === 0}
		<div class="py-8 text-center">
			<p class="text-sm text-muted-foreground">{m['home.dashboard.empty']()}</p>
			<a
				href={resolve('/vms/create')}
				class="mt-3 inline-block rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:opacity-90"
			>
				{m['home.dashboard.emptyAction']()}
			</a>
		</div>
	{:else}
		<!-- Stat cards -->
		<dl class="grid gap-3 sm:grid-cols-4" aria-label="VM summary">
			<div class="rounded-lg border border-border bg-background p-3">
				<dt class="text-xs text-muted-foreground">{m['common.total']()}</dt>
				<dd class="mt-1 text-xl font-semibold">{total}</dd>
			</div>
			<div class="rounded-lg border border-border bg-background p-3">
				<dt class="text-xs text-muted-foreground">{m['home.dashboard.statusRunning']()}</dt>
				<dd class="mt-1 text-xl font-semibold text-success-soft-foreground">{running}</dd>
			</div>
			<div class="rounded-lg border border-border bg-background p-3">
				<dt class="text-xs text-muted-foreground">{m['home.dashboard.statusStopped']()}</dt>
				<dd class="mt-1 text-xl font-semibold">{stopped}</dd>
			</div>
			<div class="rounded-lg border border-border bg-background p-3">
				<dt class="text-xs text-muted-foreground">{m['home.dashboard.statusPaused']()}</dt>
				<dd class="mt-1 text-xl font-semibold text-destructive-soft-foreground">{paused}</dd>
			</div>
		</dl>

		<!-- Compact VM list -->
		<div class="mt-4 space-y-1">
			<p class="text-xs font-medium text-muted-foreground uppercase tracking-wide">
				{m['home.dashboard.title']()} ({total})
			</p>
			<ul class="divide-y divide-border" role="list">
				{#each vms as vm (vm.cluster + ':' + vm.vmid)}
					<li class="flex flex-wrap items-center gap-x-4 gap-y-1 rounded-md px-3 py-2 text-sm hover:bg-muted/50">
						<a
							href={resolve(`/vms/${encodeURIComponent(vm.cluster)}/${vm.vmid}`)}
							class="font-medium hover:underline"
							data-testid="dashboard-vm-link"
						>
							{vm.name}
						</a>
						<span
							class="inline-flex items-center rounded-full px-2 py-0.5 text-xs {statusClass(vm.status)}"
							data-testid="dashboard-vm-status"
						>
							{vm.status}
						</span>
						<span class="font-mono text-muted-foreground">{vm.node}</span>
						<span class="font-mono text-muted-foreground">{vm.cpuCores} {m['common.cores']()}</span>
						<span class="font-mono text-muted-foreground">{vm.memoryTotal > 0 ? `${Math.round(vm.memoryTotal / 1048576 * 10) / 10} GiB` : '—'}</span>
					</li>
				{/each}
			</ul>
		</div>

		<div class="mt-4">
			<a
				href={resolve('/vms')}
				class="inline-block rounded-md border border-border bg-background px-3 py-1.5 text-sm font-medium hover:bg-muted"
				data-testid="dashboard-view-all"
			>
				{m['home.dashboard.viewAll']()}
			</a>
		</div>
	{/if}
</section>
{/if}
