<script lang="ts">
	import { get } from '$lib/shared/api/client';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import { formatBytes } from '$lib/shared/format-bytes';
	import type { VmListItem, VmQuota, VmStatus } from '$lib/features/vms/list.svelte';
	import { onMount } from 'svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import ButtonLink from '$lib/shared/ui/ButtonLink.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Meter from '$lib/shared/ui/Meter.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import StatCard from '$lib/shared/ui/StatCard.svelte';
	import SidebarIcon from '$lib/features/chrome/SidebarIcon.svelte';
	import SpinnerIcon from '$lib/shared/ui/icons/SpinnerIcon.svelte';
	import ChevronDownIcon from '$lib/shared/ui/icons/ChevronDownIcon.svelte';

	type DashboardVm = VmListItem;

	const session = getSessionContext();
	const tray = getTaskTrayContext();

	let vms = $state<DashboardVm[]>([]);
	let quota = $state<VmQuota | null | undefined>(undefined);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let refreshCount = $state(0);

	async function load(): Promise<void> {
		loading = true;
		error = null;
		try {
			// The list endpoint needs an explicit cluster - a scope=mine request
			// with none returns a 500, since it can't pick one out of several
			// without being told. The session already knows which cluster this
			// user belongs to (there's exactly one, unlike the admin-facing VM
			// list's cross-cluster view), so pass it along.
			const cluster = session.principal?.cluster ?? '';
			const query = `scope=mine&pageSize=20${cluster ? `&cluster=${encodeURIComponent(cluster)}` : ''}`;
			const result = await get<{ items: DashboardVm[]; quota?: VmQuota }>(`/api/v1/vms?${query}`);
			vms = result.items;
			quota = result.quota ?? null;
		} catch {
			error = m['home.dashboard.loadError']();
		} finally {
			loading = false;
		}
	}

	// Pill tone per state (same mapping as the main VM list) and a matching
	// tint for the identity tile, so state reads at a glance without a second
	// colour vocabulary.
	const statusTone: Record<VmStatus, 'ok' | 'off' | 'warn'> = {
		running: 'ok',
		stopped: 'off',
		paused: 'warn'
	};

	const markTone: Record<VmStatus, string> = {
		running: 'bg-success-soft text-success-soft-foreground border-success-soft-border',
		stopped: 'bg-muted text-muted-foreground border-border',
		paused: 'bg-warning-soft text-warning-soft-foreground border-warning-soft-border'
	};

	const statusLabels: Record<VmStatus, () => string> = {
		running: () => m['common.statusRunning'](),
		stopped: () => m['common.statusStopped'](),
		paused: () => m['common.statusPaused']()
	};

	let total = $derived(vms.length);
	let running = $derived(vms.filter((v) => v.status === 'running').length);
	let stopped = $derived(vms.filter((v) => v.status === 'stopped').length);
	let paused = $derived(vms.filter((v) => v.status === 'paused').length);

	onMount(() => {
		if (session.principal && !session.principal.isAdmin) void load();
	});
</script>

{#if session.principal && !session.principal.isAdmin}
<section class="w-full max-w-5xl rounded-xl border border-border bg-card p-5 shadow-sm" aria-labelledby="dashboard-title">
	<div class="mb-4 flex items-center justify-between">
		<h2 id="dashboard-title" class="text-lg font-semibold tracking-tight">{m['home.dashboard.heading']()}</h2>
		<Button
			variant="secondary"
			size="sm"
			loading={loading}
			onclick={() => { refreshCount = refreshCount + 1; void load(); }}
			data-testid="dashboard-refresh"
		>
			{m['home.dashboard.refresh']()}
		</Button>
	</div>

	<!-- Their quota and any operation still running - the two things beyond
	     the VM list itself that "how am I doing right now" needs. Rendered
	     once the first load settles (quota !== undefined); a failed load
	     leaves both at their last-known state rather than flashing empty. -->
	{#if quota !== undefined}
		<div class="mb-4 max-w-xs">
			<Meter {quota} heading={m['home.dashboard.quotaHeading']()} />
		</div>
	{/if}
	{#if tray.tasks.length > 0}
		<p class="mb-4 flex items-center gap-2 text-sm text-muted-foreground" role="status">
			<SpinnerIcon class="h-3.5 w-3.5" />
			{m['task.ariaLabel']({ count: tray.tasks.length })}
		</p>
	{/if}

	{#if loading && vms.length === 0}
		<p role="status" aria-live="polite" class="py-6 text-center text-sm text-muted-foreground">{m['common.loading']()}</p>
	{:else if error}
		<div class="flex justify-center py-6">
			<Alert class="max-w-sm">{error}</Alert>
		</div>
	{:else if vms.length === 0}
		<div class="py-8 text-center">
			<p class="text-sm text-muted-foreground">{m['home.dashboard.empty']()}</p>
			{#if !session.isAdmin}
				<ButtonLink href={resolve('/vms/create')} class="mt-3">
					{m['home.dashboard.emptyAction']()}
				</ButtonLink>
			{/if}
		</div>
	{:else}
		<!-- Stat cards - the shared StatCard tile, same as the admin dashboard. -->
		<div class="grid grid-cols-2 gap-3 sm:grid-cols-4" role="group" aria-label="VM summary">
			<StatCard label={m['common.total']()} value={total} data-testid="dashboard-stat-total" />
			<StatCard
				label={m['home.dashboard.statusRunning']()}
				value={running}
				data-testid="dashboard-stat-running"
			/>
			<StatCard
				label={m['home.dashboard.statusStopped']()}
				value={stopped}
				data-testid="dashboard-stat-stopped"
			/>
			<StatCard
				label={m['home.dashboard.statusPaused']()}
				value={paused}
				data-testid="dashboard-stat-paused"
			/>
		</div>

		<!-- VM list - one row per machine, identity-first: a status-tinted
		     machine mark, the name as the row's headline, then the facts
		     (node, vCPU, RAM) as a quiet subtitle. The whole row is the link,
		     so the target is the size of the tile, not just the name. -->
		<div class="mt-5">
			<p class="mb-2 text-xs font-medium text-muted-foreground uppercase tracking-wide">
				{m['home.dashboard.title']()} ({total})
			</p>
			<ul class="grid gap-1.5" role="list">
				{#each vms as vm (vm.cluster + ':' + vm.vmid)}
					<li>
						<a
							href={resolve(`/vms/${encodeURIComponent(vm.cluster)}/${vm.vmid}`)}
							class="group flex items-center gap-3 rounded-xl border border-transparent px-2.5 py-2.5 transition-colors hover:border-border hover:bg-muted/50 pv-focus"
							data-testid="dashboard-vm-link"
						>
							<span
								class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border {markTone[vm.status]}"
								aria-hidden="true"
							>
								<SidebarIcon name="vm" class="h-5 w-5" />
							</span>

							<span class="min-w-0 flex-1">
								<span
									class="block truncate text-[0.9375rem] font-semibold text-foreground transition-colors group-hover:text-primary"
								>
									{vm.name}
								</span>
								<!-- Each fact is its own nowrap group with a trailing
								     separator, so a wrapped line starts with the fact
								     rather than a leading dot. -->
								<span class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-muted-foreground">
									<span class="whitespace-nowrap font-mono tabular-nums">#{vm.vmid}</span>
									<span class="whitespace-nowrap font-mono">{vm.node} ·</span>
									<span class="whitespace-nowrap">
										{vm.cpuCores} {m['common.coreCount']({ count: vm.cpuCores })} ·
									</span>
									<span class="whitespace-nowrap font-mono tabular-nums">
										{vm.memoryTotal > 0 ? formatBytes(vm.memoryTotal) : ' - '}
									</span>
								</span>
							</span>

							<span data-testid="dashboard-vm-status">
								<Pill tone={statusTone[vm.status]} label={statusLabels[vm.status]()} />
							</span>

							<ChevronDownIcon
								class="h-4 w-4 shrink-0 -rotate-90 text-muted-foreground-subtle transition-colors group-hover:text-primary"
							/>
						</a>
					</li>
				{/each}
			</ul>
		</div>

		<div class="mt-4">
			<ButtonLink
				href={resolve('/vms')}
				variant="secondary"
				size="sm"
				data-testid="dashboard-view-all"
			>
				{m['home.dashboard.viewAll']()}
			</ButtonLink>
		</div>
	{/if}
</section>
{/if}
