<script lang="ts">
	import type { AdminPool, CreatedPoolCredentials } from './pools.svelte';
	import CreatePoolDialog from './CreatePoolDialog.svelte';
	import DeletePoolConfirm from './DeletePoolConfirm.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		pools: AdminPool[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		deleting: string | null;
		deleteError: string | null;
		announce: string | null;
		credentials: CreatedPoolCredentials | null;
		onSearch: (value: string) => void;
		onCreate: (name: string, comment: string) => Promise<void>;
		onDelete: (name: string) => Promise<void>;
		onDismissCredentials: () => void;
	}

	let { pools, loading, error, saving, saveError, deleting, deleteError, announce, credentials, onSearch, onCreate, onDelete, onDismissCredentials }: Props = $props();
	let search = $state('');
	let showCreate = $state(false);
	let deleteName = $state<string | null>(null);

	function openCreate(): void {
		showCreate = true;
	}

	function closeCreate(): void {
		showCreate = false;
	}

	function openDelete(name: string): void {
		deleteName = name;
	}

	function closeDelete(): void {
		deleteName = null;
	}

	async function confirmDelete(): Promise<void> {
		if (deleteName) {
			try {
				await onDelete(deleteName);
				deleteName = null;
			} catch {
				// error is set on the store; dialog stays open
			}
		}
	}
</script>

<svelte:head>
	<title>{m['admin.pools.pageTitle']()}</title>
</svelte:head>

<PageHeader title={m['admin.pools.heading']()} description={m['admin.pools.description']()}>
	{#snippet actions()}
		<Button onclick={openCreate}>{m['admin.pools.newPool']()}</Button>
	{/snippet}
</PageHeader>

<div class="mb-5 max-w-sm">
	<label for="pool-search" class="mb-1 block text-sm font-medium">{m['admin.pools.search']()}</label>
	<input id="pool-search" class="w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" type="search" bind:value={search} oninput={() => onSearch(search)} />
</div>

<div class="sr-only" role="status" aria-live="polite">{announce ?? ''}</div>
{#if loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={6} />
{:else if error}
	<p role="alert" class="text-destructive">{error}</p>
{:else}
	{#if saveError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{saveError}</p>
	{/if}
	{#if deleteError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{deleteError}</p>
	{/if}
	<div class="overflow-x-auto rounded-lg border border-border">
		<table class="w-full text-sm">
			<thead class="bg-muted/50 text-left">
				<tr>
					<th class="px-4 py-2 font-medium">{m['common.name']()}</th>
					<th class="px-4 py-2 font-medium">{m['admin.pools.comment']()}</th>
					<th class="px-4 py-2 font-medium">{m['admin.pools.origin']()}</th>
					<th class="px-4 py-2 text-right font-medium">{m['common.total']()}</th>
					<th class="px-4 py-2 text-right font-medium">{m['common.running']()}</th>
					<th class="px-4 py-2 text-right font-medium">{m['common.stopped']()}</th>
					<th class="px-4 py-2 text-right font-medium">{m['common.actions']()}</th>
				</tr>
			</thead>
			<tbody>
				{#each pools as pool (pool.name)}
					<tr class="border-t border-border">
						<td class="px-4 py-2 font-mono">{pool.name}</td>
						<td class="px-4 py-2 text-muted-foreground">{pool.comment || '—'}</td>
						<td class="px-4 py-2">
							<span class="inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium {pool.managed ? 'border-primary/30 bg-primary/10 text-primary' : 'border-border bg-muted text-muted-foreground'}">
								{pool.managed ? m['admin.pools.managedByPvmss']() : m['admin.pools.managedByProxmox']()}
							</span>
						</td>
						<td class="px-4 py-2 text-right">{pool.total}</td>
						<td class="px-4 py-2 text-right">{pool.running}</td>
						<td class="px-4 py-2 text-right">{pool.stopped}</td>
						<td class="px-4 py-2 text-right">
							{#if pool.managed}
								<Button variant="destructive" size="sm" label={m['admin.pools.deletePoolLabel']({ name: pool.name })} onclick={() => openDelete(pool.name)}>{m['common.delete']()}</Button>
							{:else}
								<span class="text-xs text-muted-foreground">{m['admin.pools.deleteBlockedNotManaged']()}</span>
							{/if}
						</td>
					</tr>
				{:else}
					<tr><td colspan={7} class="p-0">
						<EmptyState title={search ? m['admin.pools.noSearchResults']() : m['admin.pools.noPools']()} />
					</td></tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

<CreatePoolDialog bind:open={showCreate} saving={saving} error={saveError} onClose={closeCreate} onCreate={async (name, comment) => { try { await onCreate(name, comment); closeCreate(); } catch { /* error shown via saveError */ } }} />
{#if deleteName}
	<DeletePoolConfirm open={true} poolName={deleteName} deleting={deleting === deleteName} error={deleteError} onClose={closeDelete} onConfirm={confirmDelete} />
{/if}

{#if credentials}
	<Dialog open={true} labelledBy="credentials-title" onClose={onDismissCredentials}>
		<h2 id="credentials-title" class="mb-4 text-lg font-semibold">{m['admin.pools.credentialsTitle']()}</h2>
		<div class="space-y-4">
			<p class="rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive" role="alert">
				{m['admin.pools.credentialsWarning']()}
			</p>
			<div class="space-y-2">
				<div>
					<span class="mb-1 block text-sm font-medium">{m['admin.pools.poolName']()}</span>
					<div class="flex items-center gap-2">
						<code class="flex-1 rounded-md border border-border bg-muted px-3 py-2 text-sm">{credentials.name}</code>
						<Button variant="secondary" size="sm" onclick={() => navigator.clipboard.writeText(credentials.name)}>{m['common.copy']()}</Button>
					</div>
				</div>
				<div>
					<span class="mb-1 block text-sm font-medium">{m['admin.pools.username']()}</span>
					<div class="flex items-center gap-2">
						<code class="flex-1 rounded-md border border-border bg-muted px-3 py-2 text-sm">{credentials.username}@pve</code>
						<Button variant="secondary" size="sm" onclick={() => navigator.clipboard.writeText(`${credentials.username}@pve`)}>{m['common.copy']()}</Button>
					</div>
				</div>
				<div>
					<span class="mb-1 block text-sm font-medium">{m['admin.pools.generatedPassword']()}</span>
					<div class="flex items-center gap-2">
						<code class="flex-1 rounded-md border border-border bg-muted px-3 py-2 text-sm font-mono">{credentials.password}</code>
						<Button variant="secondary" size="sm" onclick={() => navigator.clipboard.writeText(credentials.password)}>{m['common.copy']()}</Button>
					</div>
				</div>
			</div>
			<div class="flex justify-end pt-2">
				<Button onclick={onDismissCredentials}>{m['admin.pools.credentialsDismiss']()}</Button>
			</div>
		</div>
	</Dialog>
{/if}
