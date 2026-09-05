<script lang="ts">
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { getDbOpsContext } from './dbOps.svelte';
	import { m } from '$lib/paraglide/messages.js';

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
	<h2 class="text-xl font-semibold tracking-tight">{m['admin.db.importTitle']()}</h2>
	<p class="text-sm text-muted-foreground">
		{m['admin.db.importDescription']()}
	</p>

	{#if store.importError}
		<Alert>{store.importError}</Alert>
	{/if}

	{#if store.confirmError}
		<Alert>{store.confirmError}</Alert>
	{/if}

	{#if store.confirmResult}
		<p role="status" class="rounded-md bg-success-soft px-4 py-2 text-sm text-success-soft-foreground">
			{m['admin.db.importSuccess']({ tables: store.confirmResult.tables.map((t) => t.name).join(', ') })}
		</p>
	{/if}

	<input
		type="file"
		accept=".db,application/octet-stream"
		onchange={onFileChange}
		class="block w-full text-sm text-muted-foreground file:mr-4 file:rounded-md file:border-0 file:bg-primary file:px-4 file:py-2 file:text-sm file:font-medium file:text-primary-foreground"
	/>

	{#if store.importing}
		<p role="status" aria-live="polite" class="text-muted-foreground">{m['common.uploading']()}</p>
	{/if}

	{#if store.preview}
		<div class="rounded-md border border-border p-4 space-y-3">
			<h3 class="font-medium">{m['admin.db.preview']()}</h3>
			<p class="text-sm text-muted-foreground">
				{m['admin.db.previewDescription']()}
			</p>
			<table class="pv-table">
				<thead>
					<tr>
						<th class="py-1.5 font-medium">{m['admin.db.table']()}</th>
						<th class="py-1.5 font-medium">{m['admin.db.rows']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each store.preview.tables as table (table.name)}
						<tr class="border-t border-border">
							<td class="py-1.5 font-mono">{table.name}</td>
							<td class="py-1.5">{table.rowCount}</td>
						</tr>
					{/each}
				</tbody>
			</table>

			{#if store.preview.ignoredTables.length > 0}
				<p class="text-sm text-muted-foreground">
					{m['admin.db.ignoredTables']()}
					<span class="font-mono">{store.preview.ignoredTables.join(', ')}</span>
				</p>
			{/if}

			<p class="text-sm text-muted-foreground">
				{m['admin.db.expiresAt']()} {new Date(store.preview.expiresAt).toLocaleString()}.
			</p>

			<div class="flex gap-2">
				<Button variant="destructive" disabled={store.confirming} onclick={openConfirm}>
					{store.confirming ? m['common.confirming']() : m['admin.db.confirmImport']()}
				</Button>
				<Button variant="secondary" onclick={() => store.cancelPreview()}>{m['common.cancel']()}</Button>
			</div>
		</div>
	{/if}
</section>

<Dialog open={confirmOpen} labelledBy={CONFIRM_DIALOG_ID} onClose={closeConfirm}>
	<h2 id={CONFIRM_DIALOG_ID} class="text-lg font-semibold">{m['admin.db.confirmDialogTitle']()}</h2>
	<p class="mt-2 text-sm text-muted-foreground">
		{m['admin.db.confirmDialogText']()}
	</p>
	<div class="mt-4 flex justify-end gap-2">
		<Button variant="secondary" onclick={closeConfirm}>{m['common.cancel']()}</Button>
		<Button variant="destructive" onclick={doConfirm}>{m['common.confirm']()}</Button>
	</div>
</Dialog>
