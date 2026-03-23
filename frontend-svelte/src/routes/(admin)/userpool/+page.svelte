<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import ConfirmDialog from '$lib/components/forms/ConfirmDialog.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import { getPools, createPool, deletePool } from '$lib/api/admin/userpool';
	import { UsersThree, Trash } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { Pool } from '$lib/types/admin';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let pools = $state<Pool[]>([]);
	let createOpen = $state(false);
	let deleteTarget = $state<string | null>(null);
	let form = $state({ pool_name: '', username: '', password: '' });

	async function load() {
		loading = true;
		error = null;
		try {
			pools = await getPools();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	async function handleCreate() {
		if (!form.pool_name || !form.username || !form.password) return;
		try {
			await createPool(form);
			toast.success($t('admin.userpool.toast.created', { values: { poolName: form.pool_name } }));
			form = { pool_name: '', username: '', password: '' };
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

<!-- Gradient page header -->
<div class="pv-header -mx-6 -mt-6 mb-6">
	<div class="pv-header-flex">
		<div>
			<p class="pv-eyebrow">{$t('nav.administration')}</p>
			<h1 class="pv-title">{$t('admin.userpool.title')}</h1>
			{#if !loading}
				<p class="pv-subtitle">
					{pools.length}
					{$t('admin.userpool.title').toLowerCase()}
				</p>
			{/if}
		</div>

		{#if !loading}
			<div class="flex items-center gap-3">
				{#if pools.length > 0}
					<div class="pv-header-stats">
						<div class="pv-header-stat">
							<div class="pv-header-stat-label">{$t('admin.userpool.title')}</div>
							<div class="pv-header-stat-value">{pools.length}</div>
						</div>
					</div>
				{/if}
				<Button class="pv-header-btn" variant="outline" onclick={() => (createOpen = true)}>
					{$t('admin.userpool.createPool')}
				</Button>
			</div>
		{/if}
	</div>
</div>

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if pools.length === 0}
	<EmptyState
		title={$t('admin.userpool.noPools')}
		icon={UsersThree}
		description={$t('admin.userpool.noPoolsDesc')}
	/>
{:else}
	<div class="pv-table-wrap">
		<table class="pv-table">
			<thead>
				<tr>
					<th>{$t('admin.userpool.poolId')}</th>
					<th>{$t('admin.userpool.comment')}</th>
					<th class="pv-th-num">{$t('admin.userpool.members')}</th>
					<th class="pv-td-actions">{$t('common.actions')}</th>
				</tr>
			</thead>
			<tbody>
				{#each pools as pool}
					<tr class="pv-row">
						<td>
							<div class="pv-resource-cell">
								<div class="pv-resource-icon" style="width:28px;height:28px;font-size:0.65rem">
									{pool.poolid.slice(0, 2).toUpperCase()}
								</div>
								<span class="pv-td-mono">{pool.poolid}</span>
							</div>
						</td>
						<td class="pv-td-muted">{pool.comment || '—'}</td>
						<td class="pv-td-num">
							<span class="pv-action-badge pv-action-badge--vm">{pool.members.length}</span>
						</td>
						<td class="pv-td-actions">
							<Button
								variant="ghost"
								size="sm"
								class="text-destructive hover:text-destructive hover:bg-destructive/10"
								onclick={() => (deleteTarget = pool.poolid)}
							>
								<Trash class="h-4 w-4" />
							</Button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

<!-- Create pool dialog -->
<Dialog.Root bind:open={createOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{$t('admin.userpool.createTitle')}</Dialog.Title>
			<Dialog.Description>{$t('admin.userpool.createDesc')}</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-4 py-2">
			<div class="space-y-2">
				<Label>{$t('admin.userpool.poolName')}</Label>
				<Input bind:value={form.pool_name} placeholder="my-pool" />
			</div>
			<div class="space-y-2">
				<Label>{$t('admin.userpool.username')}</Label>
				<Input bind:value={form.username} placeholder="user@pve" />
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
				disabled={!form.pool_name || !form.username || !form.password}
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
