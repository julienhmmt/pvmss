<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { upsertSettings } from '$lib/api/admin/settings-overview';
	import type { SectionMeta } from '$lib/api/admin/settings-overview';

	let { meta, data, table, onUpdate }: { meta: SectionMeta; data: string[]; table: string; onUpdate: () => Promise<void> } = $props();

	let items = $derived([...(data ?? [])]);
	let newItem = $state('');
	let saving = $state(false);
	let error = $state<string | null>(null);
	let itemToDelete = $state<string | null>(null);

	async function handleAdd() {
		const trimmed = newItem.trim();
		if (!trimmed) return;
		saving = true;
		error = null;
		try {
			await upsertSettings({ table, record: [...items, trimmed] });
			await onUpdate();
			newItem = '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add';
		} finally {
			saving = false;
		}
	}

	function confirmRemove(item: string) {
		itemToDelete = item;
	}

	async function handleRemove() {
		if (!itemToDelete) return;
		saving = true;
		error = null;
		try {
			await upsertSettings({ table, record: items.filter((i) => i !== itemToDelete) });
			await onUpdate();
			itemToDelete = null;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to remove';
		} finally {
			saving = false;
		}
	}

	function cancelRemove() {
		itemToDelete = null;
	}
</script>

<div class="rounded-xl border border-border bg-card p-5 space-y-4">
	<div>
		<p class="font-medium text-sm">{meta.name}</p>
		{#if meta.last_change_by}
			<p class="text-xs text-muted-foreground mt-0.5">
				{$t('admin.settings.overview.lastUpdated', { values: { user: meta.last_change_by, time: meta.last_change_at ? new Date(meta.last_change_at).toLocaleString() : '' } })}
			</p>
		{/if}
	</div>

	<div class="flex gap-2">
		<input
			type="text"
			bind:value={newItem}
			onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleAdd(); } }}
			placeholder={$t('admin.settings.overview.list.addItem')}
			disabled={saving}
			class="flex-1 rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
		/>
		<Button size="sm" onclick={handleAdd} disabled={saving || !newItem.trim()}>
			{$t('common.create')}
		</Button>
	</div>

	{#if error}
		<p class="text-sm text-destructive">{error}</p>
	{/if}

	{#if items.length === 0}
		<p class="text-sm text-muted-foreground italic">{$t('common.noData')}</p>
	{:else}
		<ul class="space-y-1.5">
			{#each items as item}
				<li class="flex items-center justify-between rounded-lg border border-border bg-muted/30 px-3 py-2">
					<span class="text-sm">{item}</span>
					<button
						type="button"
						onclick={() => confirmRemove(item)}
						disabled={saving || itemToDelete !== null}
						class="text-xs text-muted-foreground hover:text-destructive disabled:opacity-40 transition-colors"
						aria-label={$t('common.delete')}
					>
						✕
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>

{#if itemToDelete}
	<div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
		<div class="bg-background rounded-lg border border-border p-6 max-w-sm w-full mx-4 shadow-lg">
			<h3 class="text-lg font-semibold mb-2">{$t('admin.settings.overview.list.confirmDeleteTitle')}</h3>
			<p class="text-sm text-muted-foreground mb-4">
				{$t('admin.settings.overview.list.confirmDeleteMessage', { values: { item: itemToDelete } })}
			</p>
			<div class="flex gap-2 justify-end">
				<Button variant="outline" size="sm" onclick={cancelRemove} disabled={saving}>
					{$t('common.cancel')}
				</Button>
				<Button size="sm" variant="destructive" onclick={handleRemove} disabled={saving}>
					{$t('common.delete')}
				</Button>
			</div>
		</div>
	</div>
{/if}
