<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import { getDbOpsContext } from './dbOps.svelte';

	const store = getDbOpsContext();

	let confirmOpen = $state(false);

	const CONFIRM_DIALOG_ID = 'import-confirm-title';

	function onFileChange(event: Event): void {
		const target = event.target as HTMLInputElement;
		const fileList = target.files;
		if (fileList && fileList.length > 0) {
			const file: File = fileList.item(0) as File;
			void store.uploadImport(file);
		}
	}

	function openConfirm(): void {
		confirmOpen = true;
	}

	function closeConfirm(): void {
		confirmOpen = false;
	}

	function doConfirm(): void {
		confirmOpen = false;
		void store.confirmImport();
	}
</script>

<section class="space-y-3">
	<h2 class="text-xl font-semibold tracking-tight">Import Database</h2>
	<p class="text-sm text-muted-foreground">
		Upload a previously exported database file. You'll see a preview before anything is written.
	</p>

	{#if store.importError}
		<p role="alert" class="rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{store.importError}</p>
	{/if}

	{#if store.confirmError}
		<p role="alert" class="rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{store.confirmError}</p>
	{/if}

	{#if store.confirmResult}
		<p role="status" class="rounded-md bg-green-500/10 px-4 py-2 text-sm text-green-700">
			Import successful — {store.confirmResult.tables.map((t) => t.name).join(', ')} replaced.
		</p>
	{/if}

	<input
		type="file"
		accept=".db,application/octet-stream"
		onchange={onFileChange}
		class="block w-full text-sm text-muted-foreground file:mr-4 file:rounded-md file:border-0 file:bg-primary file:px-4 file:py-2 file:text-sm file:font-medium file:text-primary-foreground"
	/>

	{#if store.importing}
		<p role="status" aria-live="polite" class="text-muted-foreground">Uploading and validating…</p>
	{/if}

	{#if store.preview}
		<div class="rounded-md border border-border p-4 space-y-3">
			<h3 class="font-medium">Preview</h3>
			<p class="text-sm text-muted-foreground">
				The following tables will be <strong>replaced</strong> (existing rows deleted, then reloaded from the upload):
			</p>
			<table class="w-full text-sm">
				<thead class="bg-muted/50 text-left">
					<tr>
						<th class="px-3 py-1.5 font-medium">Table</th>
						<th class="px-3 py-1.5 font-medium">Rows</th>
					</tr>
				</thead>
				<tbody>
					{#each store.preview.tables as table (table.name)}
						<tr class="border-t border-border">
							<td class="px-3 py-1.5 font-mono">{table.name}</td>
							<td class="px-3 py-1.5">{table.rowCount}</td>
						</tr>
					{/each}
				</tbody>
			</table>

			{#if store.preview.ignoredTables.length > 0}
				<p class="text-sm text-muted-foreground">
					Ignored tables (present in the upload but not on the allowlist):
					<span class="font-mono">{store.preview.ignoredTables.join(', ')}</span>
				</p>
			{/if}

			<p class="text-sm text-muted-foreground">
				Expires at {new Date(store.preview.expiresAt).toLocaleString()}.
			</p>

			<div class="flex gap-2">
				<button
					type="button"
					class="rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground"
					onclick={openConfirm}
					disabled={store.confirming}
				>
					{store.confirming ? 'Confirming…' : 'Confirm Import'}
				</button>
				<button
					type="button"
					class="rounded-md border border-border px-4 py-2 text-sm"
					onclick={() => store.cancelPreview()}
				>
					Cancel
				</button>
			</div>
		</div>
	{/if}
</section>

<Dialog open={confirmOpen} labelledBy={CONFIRM_DIALOG_ID} onClose={closeConfirm}>
	<h2 id={CONFIRM_DIALOG_ID} class="text-lg font-semibold">Confirm Import</h2>
	<p class="mt-2 text-sm text-muted-foreground">
		This will <strong>delete and replace</strong> every listed table with the upload's contents. This cannot be undone. Continue?
	</p>
	<div class="mt-4 flex justify-end gap-2">
		<button type="button" class="rounded-md border border-border px-4 py-2 text-sm" onclick={closeConfirm}>Cancel</button>
		<button type="button" class="rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground" onclick={doConfirm}>Confirm</button>
	</div>
</Dialog>
