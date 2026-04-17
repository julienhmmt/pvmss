<script lang="ts">
	import { upsertSettings, TABLE_NODE_LIMITS } from '$lib/api/admin/settings-overview';
	import type { SectionMeta } from '$lib/api/admin/settings-overview';

	interface NodeLimit {
		node: string;
		max_vms: number;
	}

	let { meta, data, onUpdate }: { meta: SectionMeta; data: NodeLimit[]; onUpdate: () => Promise<void> } = $props();

	let items = $derived.by(() => [...(data || [])]);
	let editingIndex = $state<number | null>(null);
	let editForm = $state<NodeLimit>({ node: '', max_vms: 0 });
	let saving = $state(false);
	let error = $state<string | null>(null);


	function startEdit(index: number) {
		editingIndex = index;
		editForm = { ...items[index] };
	}

	function cancelEdit() {
		editingIndex = null;
		editForm = { node: '', max_vms: 0 };
	}

	async function handleSave() {
		if (!editForm.node || editForm.max_vms < 0) return;
		saving = true;
		error = null;
		try {
			const updated = [...items];
			if (editingIndex !== null) {
				updated[editingIndex] = { ...editForm };
			} else {
				updated.push({ ...editForm });
			}
			await upsertSettings({ table: TABLE_NODE_LIMITS, record: updated });
			await onUpdate();
			cancelEdit();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save';
		} finally {
			saving = false;
		}
	}
</script>

<div class="nodelimits-section">
	<h3>{meta.name}</h3>
	{#if meta.last_change_by}
		<p class="audit-info">
			Last updated by {meta.last_change_by} at {new Date(meta.last_change_at || '').toLocaleString()}
		</p>
	{/if}

	{#if editingIndex !== null}
		<div class="edit-form">
			<h4>{editingIndex === -1 ? 'Add New Node Limit' : 'Edit Node Limit'}</h4>
			<div class="form-group">
				<label for="node-name">Node Name</label>
				<input id="node-name" type="text" bind:value={editForm.node} disabled={saving} />
			</div>
			<div class="form-group">
				<label for="max-vms">Max VMs</label>
				<input id="max-vms" type="number" min="0" bind:value={editForm.max_vms} disabled={saving} />
			</div>
			<div class="form-actions">
				<button onclick={handleSave} disabled={saving} class="btn-save">Save</button>
				<button onclick={cancelEdit} disabled={saving} class="btn-cancel">Cancel</button>
			</div>
		</div>
	{:else}
		<button onclick={() => (editingIndex = -1)} disabled={saving} class="btn-add">
			Add Node Limit
		</button>
	{/if}

	{#if error}
		<p class="error">{error}</p>
	{/if}

	<ul class="item-list">
		{#each items as item, index}
			<li class="item">
				<div class="item-details">
					<strong>{item.node}</strong>
					<span>Max VMs: {item.max_vms}</span>
				</div>
				<div class="item-actions">
					<button onclick={() => startEdit(index)} disabled={saving} class="btn-edit">Edit</button>
				</div>
			</li>
		{/each}
		{#if items.length === 0}
			<li class="empty">No node limits configured</li>
		{/if}
	</ul>
</div>

<style>
	.nodelimits-section {
		padding: 1rem;
		border: 1px solid #ddd;
		border-radius: 4px;
		margin-bottom: 1rem;
	}

	.nodelimits-section h3 {
		margin-top: 0;
		margin-bottom: 0.5rem;
	}

	.audit-info {
		font-size: 0.875rem;
		color: #666;
		margin-bottom: 1rem;
	}

	.edit-form {
		background: #f5f5f5;
		padding: 1rem;
		border-radius: 4px;
		margin-bottom: 1rem;
	}

	.edit-form h4 {
		margin-top: 0;
		margin-bottom: 0.75rem;
	}

	.form-group {
		margin-bottom: 0.75rem;
	}

	.form-group label {
		display: block;
		margin-bottom: 0.25rem;
		font-weight: 500;
	}

	.form-group input {
		width: 100%;
		padding: 0.5rem;
		border: 1px solid #ccc;
		border-radius: 4px;
	}

	.form-actions {
		display: flex;
		gap: 0.5rem;
	}

	.btn-add,
	.btn-save,
	.btn-cancel,
	.btn-edit {
		padding: 0.5rem 1rem;
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}

	.btn-add,
	.btn-save {
		background-color: #1976d2;
		color: white;
	}

	.btn-cancel {
		background-color: #666;
		color: white;
	}

	.btn-edit {
		background-color: #f57c00;
		color: white;
	}

	.btn-add:disabled,
	.btn-save:disabled,
	.btn-cancel:disabled,
	.btn-edit:disabled {
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
		padding: 0.75rem;
		background: #f5f5f5;
		border-radius: 4px;
		margin-bottom: 0.5rem;
	}

	.item-details {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.item-details span {
		font-size: 0.875rem;
		color: #666;
	}

	.item-actions {
		display: flex;
		gap: 0.5rem;
	}

	.empty {
		color: #999;
		font-style: italic;
		padding: 0.5rem;
	}
</style>
