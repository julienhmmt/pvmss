<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import ConfirmDialog from '$lib/components/forms/ConfirmDialog.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import { getPools, createPool, deletePool } from '$lib/api/admin/userpool';
	import { UsersThree } from 'phosphor-svelte';
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

<PageHeader title={$t('admin.userpool.title')} icon={UsersThree}>
	{#snippet actions()}
		<Button onclick={() => (createOpen = true)}>{$t('admin.userpool.createPool')}</Button>
	{/snippet}
</PageHeader>

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if pools.length === 0}
	<EmptyState title={$t('admin.userpool.noPools')} icon={UsersThree} description={$t('admin.userpool.noPoolsDesc')} />
{:else}
	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>{$t('admin.userpool.poolId')}</Table.Head>
					<Table.Head>{$t('admin.userpool.comment')}</Table.Head>
					<Table.Head>{$t('admin.userpool.members')}</Table.Head>
					<Table.Head>{$t('common.actions')}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each pools as pool}
					<Table.Row>
						<Table.Cell class="font-medium">{pool.poolid}</Table.Cell>
						<Table.Cell>{pool.comment}</Table.Cell>
						<Table.Cell>{pool.members.length}</Table.Cell>
						<Table.Cell>
							<Button variant="destructive" size="sm" onclick={() => (deleteTarget = pool.poolid)}>
								{$t('common.delete')}
							</Button>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}

<Dialog.Root bind:open={createOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>{$t('admin.userpool.createTitle')}</Dialog.Title>
			<Dialog.Description>{$t('admin.userpool.createDesc')}</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-4">
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
			<Button onclick={handleCreate} disabled={!form.pool_name || !form.username || !form.password}>
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
