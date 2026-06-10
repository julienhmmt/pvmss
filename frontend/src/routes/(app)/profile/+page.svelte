<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { auth } from '$lib/stores/auth.svelte';
	import { changePassword } from '$lib/api/auth';
	import { getVMsPaginated, type VMSummary } from '$lib/api/vms';
	import { Button } from '$lib/components/ui/button';
	import { FieldError } from '$lib/components/ui/field-error';
	import * as Select from '$lib/components/ui/select';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { Play, Stop, ArrowCounterClockwise, UserCircle, Lock, Desktop, SortAscending, SortDescending } from 'phosphor-svelte';
	import { api } from '$lib/api/client';
	import { vmList } from '$lib/utils/vm';

	// ── VM list state ─────────────────────────────────────────────────────────
	let vmsLoading = $state(true);
	let vmsError = $state<Error | null>(null);
	let vms = $state<VMSummary[]>([]);
	let actionLoading = $state<Record<number, boolean>>({});

	// Pagination state
	let currentPage = $state(1);
	let pageSize = $state(25);
	let totalVMs = $state(0);
	let totalPages = $state(1);
	let hasNext = $state(false);
	let hasPrev = $state(false);
	let searchQuery = $state('');
	let searchTimeout: ReturnType<typeof setTimeout>;
	let loadAbort: AbortController | null = null;
	let runningTotal = $state(0);
	let sortBy = $state<string>('vmid');
	let sortOrder = $state<string>('asc');

	// ── Password change ───────────────────────────────────────────────────────
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let passwordLoading = $state(false);
	let passwordError = $state<string | null>(null);

	// ── Derived ───────────────────────────────────────────────────────────────
	const poolName = $derived(auth.isAdmin ? null : `pvmss_${auth.username}`);
	const runningCount = $derived(runningTotal);

	function goToPage(page: number): void {
		currentPage = page;
		loadVMs();
	}

	function onSearchInput(e: Event): void {
		clearTimeout(searchTimeout);
		searchQuery = (e.target as HTMLInputElement).value;
		searchTimeout = setTimeout(() => {
			currentPage = 1;
			loadVMs();
		}, 300);
	}

	function onSortChange(value: string): void {
		sortBy = value;
		currentPage = 1;
		loadVMs();
	}

	function toggleSortOrder(): void {
		sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
		currentPage = 1;
		loadVMs();
	}

	function onPageSizeChange(v: string | undefined): void {
		if (!v) return;
		pageSize = Number(v);
		currentPage = 1;
		loadVMs();
	}

	// ── Data loading ──────────────────────────────────────────────────────────
	async function loadVMs(): Promise<void> {
		// Cancel any in-flight request to prevent race conditions.
		if (loadAbort) loadAbort.abort();
		const abort = new AbortController();
		loadAbort = abort;

		vmsLoading = true;
		vmsError = null;
		try {
			const res = await getVMsPaginated({
				page: currentPage,
				limit: pageSize,
				sortBy: sortBy,
				sortOrder: sortOrder,
				...(searchQuery && { search: searchQuery }),
			});
			if (abort.signal.aborted) return;
			vms = res.vms;
			totalVMs = res.pagination.total;
			totalPages = res.pagination.totalPages;
			hasNext = res.pagination.hasNext;
			hasPrev = res.pagination.hasPrev;
			runningTotal = res.pagination.runningCount;
		} catch (err: unknown) {
			if (abort.signal.aborted) return;
			vmsError = err instanceof Error ? err : new Error(String(err));
		} finally {
			if (!abort.signal.aborted) {
				vmsLoading = false;
			}
			if (loadAbort === abort) loadAbort = null;
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
		} catch (err: unknown) {
			console.error(`VM action ${action} failed:`, err instanceof Error ? err.message : String(err));
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
		} catch (err: unknown) {
			passwordError = err instanceof Error ? err.message : String(err);
			toast.error($t('user.profile.passwordChangeFailed'));
		} finally {
			passwordLoading = false;
		}
	}

	onMount(() => {
		loadVMs();
		return () => {
			clearTimeout(searchTimeout);
			clearTimeout(reloadTimeout);
			if (loadAbort) loadAbort.abort();
		};
	});
</script>

<svelte:head>
	<title>PVMSS — {$t('user.profile.title')}</title>
</svelte:head>

<div class="mx-auto px-4 py-6 pv-content-width">
	<!-- Header -->
	<div class="mb-5">
		<h1 class="text-2xl font-bold">{$t('user.profile.title')}</h1>
	</div>

	<!-- Two-column layout: sidebar (info + password) + main (VMs) -->
	<div class="grid grid-cols-1 gap-6 lg:grid-cols-[320px_1fr]">

		<!-- Left sidebar: user info + password -->
		<div class="flex flex-col gap-6">

			<!-- User info card -->
			<div class="pv-card">
				<div class="pv-card-header">
					<UserCircle class="h-5 w-5" />
					<span>{$t('user.profile.username')}</span>
				</div>
				<div class="pv-card-body">
					<!-- Avatar / initials -->
					<div class="mb-4 flex items-center gap-3">
						<div class="pv-profile-avatar">
							{(auth.username ?? '?').slice(0, 1).toUpperCase()}
						</div>
						<div>
							<p class="font-semibold">{auth.username}</p>
							<p class="text-xs text-muted-foreground">
								{auth.isAdmin ? $t('nav.administrator') : $t('nav.user')}
							</p>
						</div>
					</div>
					<div class="flex flex-col gap-3">
						{#if poolName}
							<div class="flex justify-between text-sm">
								<span class="text-muted-foreground">{$t('user.profile.pool')}</span>
								<span class="font-mono">{poolName}</span>
							</div>
						{/if}
						{#if !vmsLoading}
							<div class="flex justify-between text-sm">
								<span class="text-muted-foreground">{$t('user.profile.vmCount')}</span>
								<span>{totalVMs}
									{#if runningCount > 0}
										<span class="text-success text-xs">({runningCount} running)</span>
									{/if}
								</span>
							</div>
						{/if}
					</div>
				</div>
			</div>

			<!-- Password change card -->
			<div class="pv-card">
				<div class="pv-card-header">
					<Lock class="h-5 w-5" />
					<span>{$t('user.profile.changePassword')}</span>
				</div>
				<div class="pv-card-body">
					{#if auth.isAdmin}
						<p class="text-sm text-muted-foreground">{$t('user.profile.adminPasswordNote')}</p>
					{:else}
						<form
							class="flex flex-col gap-3"
							onsubmit={(e: SubmitEvent) => {
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
							<FieldError message={passwordError} />
							<div>
								<Button type="submit" size="sm" loading={passwordLoading}>
									{passwordLoading ? $t('user.profile.saving') : $t('user.profile.savePassword')}
								</Button>
							</div>
						</form>
					{/if}
				</div>
			</div>
		</div>

		<!-- Right: VM list -->
		<div class="pv-card min-w-0">
			<div class="pv-card-header">
				<Desktop class="h-5 w-5" />
				<span>{$t('user.profile.myVms')}</span>
				{#if !vmsLoading}
					<span class="ml-auto text-xs text-muted-foreground font-normal">{totalVMs} VM{totalVMs !== 1 ? 's' : ''}</span>
				{/if}
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
				{:else if totalVMs === 0}
					<p class="p-6 text-center text-sm text-muted-foreground">
						{#if searchQuery}
							{$t('user.profile.noSearchResults')}
						{:else}
							{$t('user.profile.noVms')}
						{/if}
					</p>
				{:else}
					<!-- Search & sort bar -->
					<div class="flex items-center gap-2 p-3 border-b border-border">
						<input
							type="search"
							class="pv-input flex-1"
							placeholder={$t('common.search', { default: 'Search...' })}
							oninput={onSearchInput}
							value={searchQuery}
						/>
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
											<span class="pv-badge {vmList.statusClass(vm.status)}">
												{$t(`common.statusMap.${vm.status}`, { default: vm.status })}
											</span>
										</td>
										<td class="pv-td-muted text-sm">{vm.node}</td>
										<td class="pv-td-muted tabular-nums text-sm">{vmList.uptimeLabel(vm.uptime)}</td>
										<td onclick={(e: MouseEvent) => e.stopPropagation()}>
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

					<!-- Pagination controls -->
					{#if totalPages > 1}
						<div class="flex items-center justify-between px-4 py-3 border-t border-border">
							<p class="text-sm text-muted-foreground">
								{$t('vms.pagination.showing', {
									values: {
										start: (currentPage - 1) * pageSize + 1,
										end: Math.min(currentPage * pageSize, totalVMs),
										total: totalVMs
									}
								})}
							</p>
							<div class="flex items-center gap-4">
								<Select.Root
									type="single"
									value={String(pageSize)}
									onValueChange={onPageSizeChange}
								>
									<Select.Trigger class="w-[110px]">
										{$t('vms.pagination.perPage', { values: { count: pageSize } })}
									</Select.Trigger>
									<Select.Content>
										<Select.Item value="10">10 / page</Select.Item>
										<Select.Item value="25">25 / page</Select.Item>
										<Select.Item value="50">50 / page</Select.Item>
										<Select.Item value="100">100 / page</Select.Item>
									</Select.Content>
								</Select.Root>
								<div class="flex items-center gap-2">
									<button
										class="pv-action-btn"
										disabled={!hasPrev}
										onclick={() => goToPage(currentPage - 1)}
									>
										{$t('common.previous')}
									</button>
									<span class="text-sm text-muted-foreground">
										{$t('common.pageOf', { values: { page: currentPage, total: totalPages } })}
									</span>
									<button
										class="pv-action-btn"
										disabled={!hasNext}
										onclick={() => goToPage(currentPage + 1)}
									>
										{$t('common.next')}
									</button>
								</div>
							</div>
						</div>
					{/if}
				{/if}
			</div>
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

	/* Profile avatar */
	:global(.pv-profile-avatar) {
		width: 44px;
		height: 44px;
		border-radius: 10px;
		background: linear-gradient(135deg, hsl(var(--blaze-orange-500)), hsl(var(--blaze-orange-700)));
		color: white;
		font-size: 1.125rem;
		font-weight: 700;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}
</style>
