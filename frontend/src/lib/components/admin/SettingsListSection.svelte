<script lang="ts">
	import { upsertSettings } from '$lib/api/admin/settings-overview';
	import type { SectionMeta } from '$lib/api/admin/settings-overview';

	let { meta, data, table, onUpdate }: { meta: SectionMeta; data: string[]; table: string; onUpdate: () => Promise<void> } = $props();

	let items = $state([...(data || [])]);
	let newItem = $state('');
	let saving = $state(false);
	let error = $state<string | null>(null);

	// Sync items when data changes
	$effect(() => {
		items = [...(data || [])];
	});

	async function handleAdd() {
		if (!newItem.trim()) return;
		saving = true;
		error = null;
		try {
			const updated = [...items, newItem.trim()];
			await upsertSettings({ table, record: updated });
			await onUpdate();
			newItem = '';
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add item';
		} finally {
			saving = false;
		}
	}

	async function handleKeyPress(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			handleAdd();
		}
	}
</script>

<div class="list-section">
	<h3>{meta.name}</h3>
	{#if meta.last_change_by}
		<p class="audit-info">
			Last updated by {meta.last_change_by} at {new Date(meta.last_change_at || '').toLocaleString()}
		</p>
	{/if}

	<div class="add-item">
		<input
			type="text"
			bind:value={newItem}
			onkeypress={handleKeyPress}
			placeholder="Add new item..."
			disabled={saving}
		/>
		<button onclick={handleAdd} disabled={saving || !newItem.trim()} class="btn-add">
			Add
		</button>
	</div>

	{#if error}
		<p class="error">{error}</p>
	{/if}

	<ul class="item-list">
		{#each items as item}
			<li class="item">
				<span class="item-name">{item}</span>
			</li>
		{/each}
		{#if items.length === 0}
			<li class="empty">No items</li>
		{/if}
	</ul>
</div>

<style>
	.list-section {
		padding: 1rem;
		border: 1px solid #ddd;
		border-radius: 4px;
		margin-bottom: 1rem;
	}

	.list-section h3 {
		margin-top: 0;
		margin-bottom: 0.5rem;
	}

	.audit-info {
		font-size: 0.875rem;
		color: #666;
		margin-bottom: 1rem;
	}

	.add-item {
		display: flex;
		gap: 0.5rem;
		margin-bottom: 1rem;
	}

	.add-item input {
		flex: 1;
		padding: 0.5rem;
		border: 1px solid #ccc;
		border-radius: 4px;
	}

	.add-item input:disabled {
		opacity: 0.6;
	}

	.btn-add {
		padding: 0.5rem 1rem;
		background-color: #1976d2;
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}

	.btn-add:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.error {
		color: #d32f2f;
		margin-bottom: 1rem;
	}

	.item-list {
		list-style: none;
		padding: 0;
		margin: 0;
	}

	.item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.5rem;
		background: #f5f5f5;
		border-radius: 4px;
		margin-bottom: 0.5rem;
	}

	.item-name {
		flex: 1;
	}

	.empty {
		color: #999;
		font-style: italic;
		padding: 0.5rem;
	}
</style>
