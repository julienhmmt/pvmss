<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import PvHeader from '$lib/components/layout/PvHeader.svelte';
	import PvHeaderStat from '$lib/components/layout/PvHeaderStat.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import Paginator from '$lib/components/data/Paginator.svelte';
	import { paginate } from '$lib/utils/paginate';
	import ConfirmDialog from '$lib/components/forms/ConfirmDialog.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import { getPools, createPool, deletePool } from '$lib/api/admin/userpool';
	import { TrashIcon, UsersThreeIcon } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { Pool } from '$lib/types/admin';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let pools = $state<Pool[]>([]);
	let createOpen = $state(false);
	let deleteTarget = $state<string | null>(null);
	let form = $state({ poolName: '', password: '' });

	let page = $state(1);
	let perPage = $state(25);
	const pagedPools = $derived(paginate(pools, page, perPage));

	async function load() {
		if (pools.length > 0) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			pools = await getPools();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function handleCreate() {
		if (!form.poolName || !form.password) return;
		try {
			await createPool({
				poolName: form.poolName,
				password: form.password
			});
			toast.success($t('admin.userpool.toast.created', { values: { poolName: form.poolName } }));
			form = { poolName: '', password: '' };
			createOpen = false;
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		try {
			await deletePool(deleteTarget);
			toast.success($t('admin.userpool.toast.deleted', { values: { poolName: deleteTarget } }));
			deleteTarget = null;
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	onMount(load);
</script>

<svelte:head>
	<title>PVMSS — {$t('admin.userpool.title')}</title>
</svelte:head>

<PvHeader
	eyebrow={$t('nav.administration')}
	title={$t('admin.userpool.title')}
	subtitle={!loading ? `${pools.length} ${$t('admin.userpool.title').toLowerCase()}` : undefined}
>
	{#snippet stats()}
		{#if !loading && pools.length > 0}
			<PvHeaderStat label={$t('admin.userpool.title')} value={pools.length} />
		{/if}
	{/snippet}
	{#snippet actions()}
		{#if !loading}
			<Button class="pv-header-btn" variant="outline" onclick={() => (createOpen = true)}>
				{$t('admin.userpool.createPool')}
			</Button>
		{/if}
	{/snippet}
</PvHeader>

<div class="pv-content-width">

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if pools.length === 0}
	<EmptyState
		title={$t('admin.userpool.noPools')}
		icon={UsersThreeIcon}
		description={$t('admin.userpool.noPoolsDesc')}
	/>
{:else}
	<div class="pv-table-wrap">
		<table class="pv-table">
			<thead>
				<tr>
					<th>{$t('admin.userpool.poolId')}</th>
					<th>{$t('admin.userpool.comment')}</th>
					<th class="pv-th-num">{$t('admin.userpool.vmCount')}</th>
					<th class="pv-td-actions">{$t('common.actions')}</th>
				</tr>
			</thead>
			<tbody>
				{#each pagedPools as pool (pool.poolId)}
					<tr class="pv-row">
						<td>
							<div class="pv-resource-cell">
								<div class="pv-resource-icon" style="width:28px;height:28px;font-size:0.65rem">
									{pool.poolId.slice(0, 2).toUpperCase()}
								</div>
								<span class="pv-td-mono">{pool.poolId}</span>
							</div>
						</td>
						<td class="pv-td-muted">{pool.comment || '—'}</td>
						<td class="pv-td-num">
							<span class="pv-action-badge pv-action-badge--vm">{pool.vmCount}</span>
						</td>
						<td class="pv-td-actions">
							<Button
								variant="ghost"
								size="sm"
								class="text-destructive hover:text-destructive hover:bg-destructive/10"
								aria-label={$t('common.delete')}
								onclick={() => (deleteTarget = pool.poolId)}
							>
								<TrashIcon class="h-4 w-4" />
							</Button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	<Paginator total={pools.length} bind:page bind:perPage />
{/if}

<!-- Create pool dialog -->
<Dialog.Root bind:open={createOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{$t('admin.userpool.createPoolTitle')}</Dialog.Title>
			<Dialog.Description>{$t('admin.userpool.createPoolDesc')}</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-4 py-2">
			<div class="space-y-2">
				<Label>{$t('admin.userpool.poolName')}</Label>
				<Input bind:value={form.poolName} placeholder="my-pool" />
				{#if form.poolName}
					<p class="text-xs text-muted-foreground">{$t('admin.userpool.usernameHint', { values: { username: form.poolName + '@pve' } })}</p>
				{/if}
			</div>
			<div class="space-y-2">
				<Label>{$t('admin.userpool.password')}</Label>
				<Input type="password" bind:value={form.password} />
			</div>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (createOpen = false)}>{$t('common.cancel')}</Button>
			<Button
				onclick={handleCreate}
				disabled={!form.poolName || !form.password}
			>
				{$t('common.create')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<ConfirmDialog
	open={deleteTarget !== null}
	title={$t('admin.userpool.deleteTitle')}
	description={$t('admin.userpool.deleteDesc', { values: { poolName: deleteTarget } })}
	confirmLabel={$t('common.delete')}
	variant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (deleteTarget = null)}
/>

</div>
