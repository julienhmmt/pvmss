<script lang="ts">
	import { getAuditRetentionContext } from './auditRetention.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getAuditRetentionContext();

	let daysInput = $state('');
	let confirmed = $state(false);

	// Keep the numeric input in sync with the loaded config.
	$effect(() => {
		if (store.retentionDays !== null) {
			daysInput = String(store.retentionDays);
		}
	});

	function parsedDays(): number {
		const n = Number(daysInput);
		return Number.isInteger(n) ? n : 0;
	}

	function canPreview(): boolean {
		return parsedDays() >= 30 && !store.previewing && !store.saving;
	}

	async function onPreview(): Promise<void> {
		confirmed = false;
		await store.previewPrune(parsedDays());
	}

	async function onSave(): Promise<void> {
		await store.save(parsedDays());
		confirmed = false;
	}

	function onCancel(): void {
		store.resetConfirm();
		confirmed = false;
	}
</script>

<section class="space-y-3">
	<h2 class="text-xl font-semibold tracking-tight">{m['admin.audit.retention.title']()}</h2>
	<p class="text-sm text-muted-foreground">
		{m['admin.audit.retention.description']()}
	</p>

	{#if store.loading}
		<p class="text-sm text-muted-foreground" role="status" aria-live="polite">{m['common.loading']()}</p>
	{:else if store.error}
		<p role="alert" class="rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{store.error}</p>
	{:else}
		<form class="flex flex-wrap items-end gap-3" onsubmit={(e) => { e.preventDefault(); if (canPreview()) void onPreview(); }}>
			<label class="flex flex-col gap-1 text-sm">
				<span class="text-muted-foreground">{m['admin.audit.retention.daysLabel']()}</span>
				<input
					class="rounded-md border border-border bg-background px-3 py-1.5 w-32"
					type="number"
					min="30"
					bind:value={daysInput}
					aria-label={m['admin.audit.retention.daysLabel']()}
				/>
			</label>
			<Button type="submit" disabled={!canPreview()}>
				{store.previewing ? m['common.loading']() : m['admin.audit.retention.preview']()}
			</Button>
		</form>

		{#if parsedDays() < 30 && daysInput !== ''}
			<p role="alert" class="text-sm text-destructive">{m['admin.audit.retention.minDays']()}</p>
		{/if}

		{#if store.previewError}
			<p role="alert" class="rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{store.previewError}</p>
		{/if}

		{#if store.preview}
			<div class="rounded-md border border-border bg-muted/30 p-4 space-y-2">
				<p class="text-sm">
					{m['admin.audit.retention.confirmText']({ rows: store.preview.rowsToDelete, days: store.preview.retentionDays })}
				</p>
				{#if !confirmed}
					<div class="flex gap-2">
						<Button variant="secondary" onclick={onCancel}>{m['common.cancel']()}</Button>
						<Button disabled={store.saving} onclick={() => { confirmed = true; }}>
							{m['admin.audit.retention.confirmButton']()}
						</Button>
					</div>
				{:else}
					<div class="flex gap-2">
						<Button variant="secondary" onclick={() => { confirmed = false; }}>{m['common.back']()}</Button>
						<Button disabled={store.saving} onclick={() => void onSave()}>
							{store.saving ? m['common.saving']() : m['admin.audit.retention.apply']()}
						</Button>
					</div>
				{/if}
			</div>
		{/if}

		{#if store.saveError}
			<p role="alert" class="rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{store.saveError}</p>
		{/if}

		{#if store.saved}
			<p role="status" class="text-sm text-muted-foreground">{m['admin.audit.retention.saved']()}</p>
		{/if}
	{/if}
</section>
