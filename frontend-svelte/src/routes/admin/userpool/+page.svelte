<script lang="ts">
	import { onMount } from 'svelte';
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
			toast.success(`Pool "${form.pool_name}" created`);
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
			toast.success(`Pool "${deleteTarget}" deleted`);
			deleteTarget = null;
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	onMount(load);
</script>

<PageHeader title="User Pools" icon={UsersThree}>
	{#snippet actions()}
		<Button onclick={() => (createOpen = true)}>Create Pool</Button>
	{/snippet}
</PageHeader>

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if pools.length === 0}
	<EmptyState title="No user pools" icon={UsersThree} description="Create a pool to manage user VMs" />
{:else}
	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>Pool ID</Table.Head>
					<Table.Head>Comment</Table.Head>
					<Table.Head>Members</Table.Head>
					<Table.Head>Actions</Table.Head>
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
								Delete
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
			<Dialog.Title>Create User Pool</Dialog.Title>
			<Dialog.Description>Set up a new Proxmox user pool with dedicated credentials.</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-4">
			<div class="space-y-2">
				<Label>Pool Name</Label>
				<Input bind:value={form.pool_name} placeholder="my-pool" />
			</div>
			<div class="space-y-2">
				<Label>Username</Label>
				<Input bind:value={form.username} placeholder="user@pve" />
			</div>
			<div class="space-y-2">
				<Label>Password</Label>
				<Input type="password" bind:value={form.password} />
			</div>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (createOpen = false)}>Cancel</Button>
			<Button onclick={handleCreate} disabled={!form.pool_name || !form.username || !form.password}>
				Create
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<ConfirmDialog
	open={deleteTarget !== null}
	title="Delete Pool"
	description={`Delete pool "${deleteTarget}"? This will remove the pool and associated user. VMs in the pool will NOT be deleted.`}
	confirmLabel="Delete"
	variant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (deleteTarget = null)}
/>
