<script lang="ts">
	import type { AdminPool } from './pools.svelte';
	import CreatePoolDialog from './CreatePoolDialog.svelte';
	import DeletePoolConfirm from './DeletePoolConfirm.svelte';

	interface Props {
		pools: AdminPool[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		deleting: string | null;
		deleteError: string | null;
		announce: string | null;
		onSearch: (value: string) => void;
		onCreate: (name: string, password: string, comment: string) => Promise<void>;
		onDelete: (name: string) => Promise<void>;
	}

	let { pools, loading, error, saving, saveError, deleting, deleteError, announce, onSearch, onCreate, onDelete }: Props = $props();
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
	<title>Admin Pools — PVMSS</title>
</svelte:head>

<section class="mx-auto w-full max-w-5xl px-4 py-8">
	<div class="mb-6 flex flex-wrap items-center justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Pools</h1>
			<p class="mt-1 text-sm text-muted-foreground">Manage user tenancy pools and their VM memberships.</p>
		</div>
		<button type="button" class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90" onclick={openCreate}>New pool</button>
	</div>

	<div class="mb-5 max-w-sm">
		<label for="pool-search" class="mb-1 block text-sm font-medium">Search pools</label>
		<input id="pool-search" class="w-full rounded-md border bg-background px-3 py-2 text-sm" type="search" bind:value={search} oninput={() => onSearch(search)} />
	</div>

	<div class="sr-only" role="status" aria-live="polite">{announce ?? ''}</div>
	{#if loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
	{:else if error}
		<p role="alert" class="text-destructive">{error}</p>
	{:else}
		{#if saveError}
			<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{saveError}</p>
		{/if}
		{#if deleteError}
			<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{deleteError}</p>
		{/if}
		<div class="overflow-x-auto rounded-lg border">
			<table class="w-full text-sm">
				<thead class="bg-muted/50 text-left">
					<tr>
						<th class="px-4 py-2 font-medium">Name</th>
						<th class="px-4 py-2 font-medium">Comment</th>
						<th class="px-4 py-2 text-right font-medium">Total</th>
						<th class="px-4 py-2 text-right font-medium">Running</th>
						<th class="px-4 py-2 text-right font-medium">Stopped</th>
						<th class="px-4 py-2 text-right font-medium">Actions</th>
					</tr>
				</thead>
				<tbody>
					{#each pools as pool (pool.name)}
						<tr class="border-t">
							<td class="px-4 py-2 font-mono">{pool.name}</td>
							<td class="px-4 py-2 text-muted-foreground">{pool.comment || '—'}</td>
							<td class="px-4 py-2 text-right">{pool.total}</td>
							<td class="px-4 py-2 text-right">{pool.running}</td>
							<td class="px-4 py-2 text-right">{pool.stopped}</td>
							<td class="px-4 py-2 text-right">
								<button type="button" class="text-xs text-destructive hover:text-destructive/80" onclick={() => openDelete(pool.name)}>Delete</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>

<CreatePoolDialog bind:open={showCreate} saving={saving} error={saveError} onClose={closeCreate} onCreate={async (name, password, comment) => { try { await onCreate(name, password, comment); closeCreate(); } catch { /* error shown via saveError */ } }} />
{#if deleteName}
	<DeletePoolConfirm open={true} poolName={deleteName} deleting={deleting === deleteName} error={deleteError} onClose={closeDelete} onConfirm={confirmDelete} />
{/if}
