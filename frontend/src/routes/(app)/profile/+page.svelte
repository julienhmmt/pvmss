<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { auth } from '$lib/stores/auth.svelte';
	import { changePassword } from '$lib/api/auth';
	import { getVMs, type VMSummary } from '$lib/api/vms';
	import { Button } from '$lib/components/ui/button';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { Play, Stop, ArrowCounterClockwise, UserCircle, Lock, Desktop } from 'phosphor-svelte';
	import { api } from '$lib/api/client';

	// ── VM list ───────────────────────────────────────────────────────────────
	let vmsLoading = $state(true);
	let vmsError = $state<Error | null>(null);
	let vms = $state<VMSummary[]>([]);
	let actionLoading = $state<Record<number, boolean>>({});

	// ── Password change ───────────────────────────────────────────────────────
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let passwordLoading = $state(false);
	let passwordError = $state<string | null>(null);

	// ── Derived ───────────────────────────────────────────────────────────────
	const poolName = $derived(auth.isAdmin ? null : `pvmss_${auth.username}`);
	const runningCount = $derived(vms.filter((v) => v.status === 'running').length);

	// ── Helpers ───────────────────────────────────────────────────────────────
	function statusClass(status: string): string {
		if (status === 'running') return 'pv-badge--online';
		if (status === 'stopped') return 'pv-badge--offline';
		return 'pv-badge--warn';
	}

	function uptimeLabel(seconds: number): string {
		if (!seconds) return '—';
		const d = Math.floor(seconds / 86400);
		const h = Math.floor((seconds % 86400) / 3600);
		if (d > 0) return `${d}d ${h}h`;
		const m = Math.floor((seconds % 3600) / 60);
		return h > 0 ? `${h}h ${m}m` : `${m}m`;
	}

	// ── Data loading ──────────────────────────────────────────────────────────
	async function loadVMs(): Promise<void> {
		vmsLoading = true;
		vmsError = null;
		try {
			vms = await getVMs();
		} catch (e) {
			vmsError = e as Error;
		} finally {
			vmsLoading = false;
		}
	}

	// ── Actions ───────────────────────────────────────────────────────────────
	let reloadTimeout: ReturnType<typeof setTimeout>;

	async function doAction(vm: VMSummary, action: string): Promise<void> {
		actionLoading = { ...actionLoading, [vm.vmid]: true };
		try {
			await api.post(`/api/v1/vms/${vm.vmid}/action`, { action, node: vm.node });
			toast.success(`${action} sent to ${vm.name || vm.vmid}`);
			clearTimeout(reloadTimeout);
			reloadTimeout = setTimeout(() => loadVMs(), 2000);
		} catch {
			toast.error(`Failed to ${action} ${vm.name || vm.vmid}`);
		} finally {
			actionLoading = { ...actionLoading, [vm.vmid]: false };
		}
	}

	async function handlePasswordChange(): Promise<void> {
		passwordError = null;
		if (newPassword !== confirmPassword) {
			passwordError = $t('user.profile.passwordsMismatch');
			return;
		}
		if (newPassword.length < 8) {
			passwordError = $t('user.profile.passwordMinLength');
			return;
		}

		passwordLoading = true;
		try {
			await changePassword(currentPassword, newPassword);
			toast.success($t('user.profile.passwordChanged'));
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
		} catch (e) {
			const err = e instanceof Error ? e.message : String(e);
			passwordError = err;
			toast.error($t('user.profile.passwordChangeFailed'));
		} finally {
			passwordLoading = false;
		}
	}

	onMount(() => loadVMs());
</script>

<svelte:head>
	<title>PVMSS — {$t('user.profile.title')}</title>
</svelte:head>

<div class="mx-auto max-w-4xl px-4 py-6">
	<!-- Header -->
	<div class="mb-6">
		<h1 class="text-2xl font-bold">{$t('user.profile.title')}</h1>
	</div>

	<!-- Profile info card -->
	<div class="pv-card mb-6">
		<div class="pv-card-header">
			<UserCircle class="h-5 w-5" />
			<span>{$t('user.profile.username')}</span>
		</div>
		<div class="pv-card-body">
			<div class="pv-info-grid">
				<div class="pv-info-row">
					<span class="pv-info-label">{$t('user.profile.username')}</span>
					<span class="pv-info-value font-mono">{auth.username}</span>
				</div>
				<div class="pv-info-row">
					<span class="pv-info-label">{$t('user.profile.role')}</span>
					<span class="pv-info-value">
						{auth.isAdmin ? $t('nav.administrator') : $t('nav.user')}
					</span>
				</div>
				{#if poolName}
					<div class="pv-info-row">
						<span class="pv-info-label">{$t('user.profile.pool')}</span>
						<span class="pv-info-value font-mono">{poolName}</span>
					</div>
				{/if}
				{#if !vmsLoading}
					<div class="pv-info-row">
						<span class="pv-info-label">{$t('user.profile.vmCount')}</span>
						<span class="pv-info-value">{vms.length} ({runningCount} running)</span>
					</div>
				{/if}
			</div>
		</div>
	</div>

	<!-- Password change card -->
	<div class="pv-card mb-6">
		<div class="pv-card-header">
			<Lock class="h-5 w-5" />
			<span>{$t('user.profile.changePassword')}</span>
		</div>
		<div class="pv-card-body">
			{#if auth.isAdmin}
				<p class="text-sm text-muted-foreground">{$t('user.profile.adminPasswordNote')}</p>
			{:else}
				<form
					class="flex flex-col gap-3 max-w-sm"
					onsubmit={(e) => {
						e.preventDefault();
						handlePasswordChange();
					}}
				>
					<div class="pv-field">
						<label class="pv-label" for="current-password">
							{$t('user.profile.currentPassword')}
						</label>
						<input
							id="current-password"
							type="password"
							class="pv-input"
							bind:value={currentPassword}
							autocomplete="current-password"
							required
						/>
					</div>
					<div class="pv-field">
						<label class="pv-label" for="new-password">
							{$t('user.profile.newPassword')}
						</label>
						<input
							id="new-password"
							type="password"
							class="pv-input"
							bind:value={newPassword}
							autocomplete="new-password"
							minlength="8"
							required
						/>
					</div>
					<div class="pv-field">
						<label class="pv-label" for="confirm-password">
							{$t('user.profile.confirmPassword')}
						</label>
						<input
							id="confirm-password"
							type="password"
							class="pv-input"
							bind:value={confirmPassword}
							autocomplete="new-password"
							minlength="8"
							required
						/>
					</div>
					{#if passwordError}
						<p class="text-sm text-destructive">{passwordError}</p>
					{/if}
					<div>
						<Button type="submit" size="sm" disabled={passwordLoading}>
							{passwordLoading ? $t('user.profile.saving') : $t('user.profile.savePassword')}
						</Button>
					</div>
				</form>
			{/if}
		</div>
	</div>

	<!-- VM list card -->
	<div class="pv-card">
		<div class="pv-card-header">
			<Desktop class="h-5 w-5" />
			<span>{$t('user.profile.myVms')}</span>
		</div>
		<div class="pv-card-body p-0">
			{#if vmsError}
				<div class="p-4">
					<ErrorBanner error={vmsError} onRetry={loadVMs} />
				</div>
			{:else if vmsLoading}
				<div class="p-4">
					<LoadingSkeleton variant="table" rows={3} />
				</div>
			{:else if vms.length === 0}
				<p class="p-6 text-center text-sm text-muted-foreground">{$t('user.profile.noVms')}</p>
			{:else}
				<div class="pv-table-wrap">
					<table class="pv-table">
						<thead>
							<tr>
								<th>{$t('vms.vmid')}</th>
								<th>{$t('common.name')}</th>
								<th>{$t('common.status')}</th>
								<th>{$t('common.node')}</th>
								<th>{$t('vms.uptime')}</th>
								<th></th>
							</tr>
						</thead>
						<tbody>
							{#each vms as vm (vm.vmid)}
								{@const busy = actionLoading[vm.vmid] ?? false}
								<tr
									class="pv-row pv-row--clickable"
									onclick={() => goto(`/vm/${vm.vmid}`)}
								>
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
									<td class="pv-td-muted text-sm">{vm.node}</td>
									<td class="pv-td-muted tabular-nums text-sm">{uptimeLabel(vm.uptime)}</td>
									<td onclick={(e) => e.stopPropagation()}>
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
	</div>
</div>

<style>
	:global(.pv-card) {
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--card);
		overflow: hidden;
	}

	:global(.pv-card-header) {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 12px 16px;
		border-bottom: 1px solid var(--border);
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--foreground);
	}

	:global(.pv-card-body) {
		padding: 16px;
	}

	:global(.pv-info-grid) {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	:global(.pv-info-row) {
		display: flex;
		align-items: baseline;
		gap: 12px;
	}

	:global(.pv-info-label) {
		font-size: 0.8125rem;
		color: var(--muted-foreground);
		font-weight: 500;
		min-width: 10rem;
		flex-shrink: 0;
	}

	:global(.pv-info-value) {
		font-size: 0.875rem;
		color: var(--foreground);
	}

	:global(.pv-field) {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	:global(.pv-label) {
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--foreground);
	}

	:global(.pv-input) {
		height: 36px;
		padding: 0 10px;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: var(--background);
		color: var(--foreground);
		font-size: 0.875rem;
		outline: none;
		transition: border-color 0.12s;
	}

	:global(.pv-input:focus) {
		border-color: hsl(var(--blaze-orange-500));
	}

	/* Row clickable */
	:global(.pv-row--clickable) {
		cursor: pointer;
	}
	:global(.pv-row--clickable:hover td) {
		background: var(--accent);
	}
</style>
