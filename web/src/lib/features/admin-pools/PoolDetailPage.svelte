<script lang="ts">
	import type { PoolDetail, PoolMemberVM } from './pool-detail.svelte';
	import { resolve } from '$app/paths';
	import { formatBytes } from '$lib/shared/format-bytes';
	import DeletePoolConfirm from './DeletePoolConfirm.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import CopyButton from '$lib/shared/ui/CopyButton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Meter from '$lib/shared/ui/Meter.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import StatCard from '$lib/shared/ui/StatCard.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		detail: PoolDetail | null;
		loading: boolean;
		error: string | null;
		notFound: boolean;
		deleting: boolean;
		deleteError: string | null;
		onRefresh: () => void;
		onDelete: () => Promise<void>;
		onDeleted: () => void;
	}

	let { detail, loading, error, notFound, deleting, deleteError, onRefresh, onDelete, onDeleted }: Props = $props();

	let showDelete = $state(false);

	const SECONDS_PER_MINUTE = 60;
	const MINUTES_PER_HOUR = 60;
	const HOURS_PER_DAY = 24;

	const totals = $derived.by(() => {
		const vms = detail?.vms ?? [];
		return {
			total: vms.length,
			running: vms.filter((vm) => vm.status === 'running').length,
			cores: vms.reduce((sum, vm) => sum + vm.cpuCores, 0),
			memory: vms.reduce((sum, vm) => sum + vm.memoryBytes, 0),
			disk: vms.reduce((sum, vm) => sum + vm.diskBytes, 0)
		};
	});

	function statusLabel(status: string): string {
		if (status === 'running') return m['machine.status.running']();
		if (status === 'stopped') return m['machine.status.stopped']();
		if (status === 'paused') return m['admin.dashboard.vmPaused']();
		return status;
	}

	function statusTone(status: string): 'ok' | 'off' | 'warn' {
		if (status === 'running') return 'ok';
		if (status === 'stopped') return 'off';
		return 'warn';
	}

	function uptimeLabel(seconds: number): string {
		if (seconds <= 0) return '—';
		const days: number = Math.floor(seconds / (SECONDS_PER_MINUTE * MINUTES_PER_HOUR * HOURS_PER_DAY));
		const hours: number = Math.floor((seconds / (SECONDS_PER_MINUTE * MINUTES_PER_HOUR)) % HOURS_PER_DAY);
		const minutes: number = Math.floor((seconds / SECONDS_PER_MINUTE) % MINUTES_PER_HOUR);
		return m['admin.nodeDetails.uptime']({ days, hours, minutes });
	}

	function activitySummary(entry: { detail: string }): string {
		try {
			const parsed = JSON.parse(entry.detail) as { summary?: string };
			return parsed.summary ?? '';
		} catch {
			return '';
		}
	}

	function vmHref(vm: PoolMemberVM): { cluster: string; vmid: string } {
		return { cluster: detail?.cluster ?? '', vmid: String(vm.vmid) };
	}

	async function confirmDelete(): Promise<void> {
		try {
			await onDelete();
			onDeleted();
		} catch {
			// error is set on the store; dialog stays open
		}
	}
</script>

<svelte:head>
	<title>{detail ? m['admin.poolDetail.pageTitle']({ name: detail.name }) : m['admin.poolDetail.title']()}</title>
</svelte:head>

<PageHeader
	title={detail?.name ?? m['admin.poolDetail.title']()}
	eyebrow={detail?.cluster ?? m['admin.pools.heading']()}
	description={m['admin.poolDetail.description']()}
	back={{ href: resolve('/admin/pools'), label: m['admin.pools.heading']() }}
	focusTarget
>
	{#snippet titleMeta()}
		{#if detail}
			<Pill
				size="md"
				dot={false}
				tone={detail.managed ? 'accent' : 'off'}
				label={detail.managed ? m['admin.pools.managedByPvmss']() : m['admin.pools.managedByProxmox']()}
			/>
		{/if}
	{/snippet}
	{#snippet actions()}
		{#if detail}
			<Button variant="secondary" size="sm" loading={loading} onclick={onRefresh} data-testid="pool-detail-refresh">
				{loading ? m['common.refreshing']() : m['common.refresh']()}
			</Button>
		{/if}
	{/snippet}
</PageHeader>

{#if detail === null}
	{#if loading}
		<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
		<div class="space-y-4" data-testid="pool-detail-skeleton">
			<Skeleton class="h-28 w-full" />
			<Skeleton class="h-52 w-full" />
			<Skeleton class="h-52 w-full" />
		</div>
	{:else}
		<Alert>{notFound ? m['admin.poolDetail.notFound']() : (error ?? m['admin.poolDetail.loadError']())}</Alert>
	{/if}
{:else}
	<div class="space-y-6" data-testid="admin-pool-detail">
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
			<StatCard label={m['common.vms']()} value={totals.total} hint={m['admin.poolDetail.vmsBreakdown']({ running: totals.running, stopped: totals.total - totals.running })} />
			<StatCard label={m['admin.nodeDetails.vcpu']()} value={totals.cores} />
			<StatCard label={m['common.memory']()} value={formatBytes(totals.memory)} />
			<StatCard label={m['admin.nodeDetails.disk']()} value={formatBytes(totals.disk)} />
		</div>

		<Card pad="none" title={m['admin.poolDetail.accountTitle']()} titleId="pool-account-section">
			<div class="grid gap-5 p-5 lg:grid-cols-2">
				<dl class="grid grid-cols-1 content-start gap-y-4 text-sm sm:grid-cols-2 sm:gap-x-5">
					<div>
						<dt class="text-xs text-muted-foreground">{m['admin.pools.username']()}</dt>
						<dd class="mt-1 flex items-center gap-1.5 font-mono">{detail.username}<CopyButton value={detail.username} /></dd>
					</div>
					<div>
						<dt class="text-xs text-muted-foreground">{m['admin.pools.poolName']()}</dt>
						<dd class="mt-1 font-mono">{detail.name}</dd>
					</div>
					<div>
						<dt class="text-xs text-muted-foreground">{m['common.cluster']()}</dt>
						<dd class="mt-1">{detail.cluster}</dd>
					</div>
					<div>
						<dt class="text-xs text-muted-foreground">{m['admin.poolDetail.provisioned']()}</dt>
						<dd class="mt-1 font-mono">
							{#if detail.createdAt}
								<time datetime={detail.createdAt}>{new Date(detail.createdAt).toLocaleString()}</time>
							{:else}
								{m['admin.nodeDetails.notReported']()}
							{/if}
						</dd>
					</div>
					<div class="sm:col-span-2">
						<dt class="text-xs text-muted-foreground">{m['admin.pools.comment']()}</dt>
						<dd class="mt-1">{detail.comment || m['admin.nodeDetails.notReported']()}</dd>
					</div>
				</dl>
				<div class="content-start">
					<p class="mb-2 text-xs text-muted-foreground">{m['admin.poolDetail.allowance']()}</p>
					<Meter quota={detail.quota} />
				</div>
			</div>
		</Card>

		<Card pad="none" title={m['admin.poolDetail.vmsTitle']()} titleId="pool-vms-section">
			{#if detail.vms.length === 0}
				<EmptyState title={m['admin.poolDetail.vmsEmpty']()} />
			{:else}
				<div class="overflow-x-auto">
					<table class="pv-table w-full min-w-[720px]">
						<caption class="sr-only">{m['admin.poolDetail.vmsTitle']()}</caption>
						<thead>
							<tr>
								<th class="font-medium">{m['admin.nodeDetails.vmid']()}</th>
								<th class="font-medium">{m['admin.nodeDetails.name']()}</th>
								<th class="font-medium">{m['common.node']()}</th>
								<th class="font-medium">{m['admin.nodeDetails.status']()}</th>
								<th class="font-medium">{m['admin.nodeDetails.uptimeLabel']()}</th>
								<th class="text-right font-medium">{m['admin.nodeDetails.vcpu']()}</th>
								<th class="text-right font-medium">{m['admin.nodeDetails.memory']()}</th>
								<th class="text-right font-medium">{m['admin.nodeDetails.disk']()}</th>
								<th class="font-medium">{m['admin.poolDetail.ipAddresses']()}</th>
							</tr>
						</thead>
						<tbody>
							{#each detail.vms as vm (vm.vmid)}
								<tr class="transition-colors hover:bg-muted/40">
									<td class="font-mono tabular-nums">{vm.vmid}</td>
									<td>
										<a
											href={resolve('/vms/[cluster]/[vmid]', vmHref(vm))}
											class="font-medium text-primary hover:underline"
										>{vm.name || m['admin.nodeDetails.notReported']()}</a>
									</td>
									<td>{vm.node}</td>
									<td><Pill tone={statusTone(vm.status)} label={statusLabel(vm.status)} /></td>
									<td class="font-mono tabular-nums">{uptimeLabel(vm.uptimeSeconds)}</td>
									<td class="text-right font-mono tabular-nums">{vm.cpuCores}</td>
									<td class="text-right font-mono tabular-nums">{formatBytes(vm.memoryBytes)}</td>
									<td class="text-right font-mono tabular-nums">{formatBytes(vm.diskBytes)}</td>
									<td class="font-mono text-xs">{vm.ipAddresses.length > 0 ? vm.ipAddresses.join(', ') : '—'}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</Card>

		<Card pad="none" title={m['admin.poolDetail.activityTitle']()} titleId="pool-activity-section">
			{#if detail.activity.length === 0}
				<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.poolDetail.activityEmpty']()}</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="pv-table w-full min-w-[560px]">
						<caption class="sr-only">{m['admin.poolDetail.activityTitle']()}</caption>
						<thead>
							<tr>
								<th class="font-medium">{m['admin.audit.time']()}</th>
								<th class="font-medium">{m['admin.audit.actor']()}</th>
								<th class="font-medium">{m['admin.audit.action']()}</th>
								<th class="font-medium">{m['admin.audit.columnVm']()}</th>
								<th class="font-medium">{m['admin.audit.detail']()}</th>
							</tr>
						</thead>
						<tbody>
							{#each detail.activity as entry (entry.id)}
								<tr>
									<td class="whitespace-nowrap font-mono text-xs tabular-nums">{new Date(entry.timestamp).toLocaleString()}</td>
									<td class="font-mono text-xs">{entry.actor}</td>
									<td><code class="text-xs">{entry.action}</code></td>
									<td class="font-mono tabular-nums">{entry.vmid ?? '—'}</td>
									<td class="max-w-72 truncate text-xs text-muted-foreground" title={activitySummary(entry)}>{activitySummary(entry) || '—'}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</Card>

		{#if detail.managed}
			<Card pad="none" title={m['admin.poolDetail.dangerTitle']()} titleId="pool-danger-section">
				<div class="flex flex-wrap items-center justify-between gap-3 p-5">
					<p class="max-w-prose text-sm text-muted-foreground">{m['admin.poolDetail.dangerBody']({ name: detail.name })}</p>
					<Button variant="destructive" onclick={() => (showDelete = true)} data-testid="pool-detail-delete">
						{m['admin.pools.deletePool']()}
					</Button>
				</div>
			</Card>
		{/if}
	</div>
{/if}

{#if showDelete && detail}
	<DeletePoolConfirm
		open={true}
		poolName={detail.name}
		{deleting}
		error={deleteError}
		onClose={() => (showDelete = false)}
		onConfirm={confirmDelete}
	/>
{/if}
