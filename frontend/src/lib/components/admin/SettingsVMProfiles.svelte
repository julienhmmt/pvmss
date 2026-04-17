<script lang="ts">
	import { upsertSettings, TABLE_VM_PROFILES } from '$lib/api/admin/settings-overview';
	import type { SectionMeta } from '$lib/api/admin/settings-overview';

	interface VMProfile {
		id: string;
		name: string;
		description?: string;
		sockets: number;
		cores: number;
		ram_gb: number;
		disk_gb: number;
		disk_bus: string;
		icon: string;
		color: string;
		enabled: boolean;
	}

	let { meta, data, onUpdate }: { meta: SectionMeta; data: VMProfile[]; onUpdate: () => Promise<void> } = $props();

	let items = $state([...(data || [])]);
	let editingIndex = $state<number | null>(null);
	let editForm = $state<VMProfile>({
		id: '',
		name: '',
		description: '',
		sockets: 1,
		cores: 1,
		ram_gb: 4,
		disk_gb: 32,
		disk_bus: 'virtio',
		icon: 'Server',
		color: 'blue',
		enabled: true
	});
	let saving = $state(false);
	let error = $state<string | null>(null);

	// Sync items when data changes
	$effect(() => {
		items = [...(data || [])];
	});

	function startEdit(index: number) {
		editingIndex = index;
		editForm = { ...items[index] };
	}

	function cancelEdit() {
		editingIndex = null;
		editForm = {
			id: '',
			name: '',
			description: '',
			sockets: 1,
			cores: 1,
			ram_gb: 4,
			disk_gb: 32,
			disk_bus: 'virtio',
			icon: 'Server',
			color: 'blue',
			enabled: true
		};
	}

	async function handleSave() {
		if (!editForm.id || !editForm.name) return;
		saving = true;
		error = null;
		try {
			const updated = [...items];
			if (editingIndex !== null) {
				updated[editingIndex] = { ...editForm };
			} else {
				updated.push({ ...editForm });
			}
			await upsertSettings({ table: TABLE_VM_PROFILES, record: updated });
			await onUpdate();
			cancelEdit();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save';
		} finally {
			saving = false;
		}
	}

	async function handleToggle(index: number) {
		saving = true;
		error = null;
		try {
			const updated = [...items];
			updated[index].enabled = !updated[index].enabled;
			await upsertSettings({ table: TABLE_VM_PROFILES, record: updated });
			await onUpdate();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to toggle';
		} finally {
			saving = false;
		}
	}

	async function handleRemove(index: number) {
		saving = true;
		error = null;
		try {
			const updated = items.filter((_, i) => i !== index);
			await upsertSettings({ table: TABLE_VM_PROFILES, record: updated });
			await onUpdate();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to remove';
		} finally {
			saving = false;
		}
	}
</script>

<div class="vmprofiles-section">
	<h3>{meta.name}</h3>
	{#if meta.last_change_by}
		<p class="audit-info">
			Last updated by {meta.last_change_by} at {new Date(meta.last_change_at || '').toLocaleString()}
		</p>
	{/if}

	{#if editingIndex !== null}
		<div class="edit-form">
			<h4>{editingIndex === -1 ? 'Add New Profile' : 'Edit Profile'}</h4>
			<div class="form-row">
				<div class="form-group">
					<label for="profile_id">ID</label>
					<input id="profile_id" type="text" bind:value={editForm.id} disabled={saving || editingIndex !== -1} />
				</div>
				<div class="form-group">
					<label for="profile_name">Name</label>
					<input id="profile_name" type="text" bind:value={editForm.name} disabled={saving} />
				</div>
			</div>
			<div class="form-group">
				<label for="profile_description">Description</label>
				<input id="profile_description" type="text" bind:value={editForm.description} disabled={saving} />
			</div>
			<div class="form-row">
				<div class="form-group">
					<label for="profile_sockets">Sockets</label>
					<input id="profile_sockets" type="number" min="1" bind:value={editForm.sockets} disabled={saving} />
				</div>
				<div class="form-group">
					<label for="profile_cores">Cores</label>
					<input id="profile_cores" type="number" min="1" bind:value={editForm.cores} disabled={saving} />
				</div>
			</div>
			<div class="form-row">
				<div class="form-group">
					<label for="profile_ram">RAM (GB)</label>
					<input id="profile_ram" type="number" min="1" bind:value={editForm.ram_gb} disabled={saving} />
				</div>
				<div class="form-group">
					<label for="profile_disk">Disk (GB)</label>
					<input id="profile_disk" type="number" min="1" bind:value={editForm.disk_gb} disabled={saving} />
				</div>
			</div>
			<div class="form-row">
				<div class="form-group">
					<label for="profile_disk_bus">Disk Bus</label>
					<select id="profile_disk_bus" bind:value={editForm.disk_bus} disabled={saving}>
						<option value="virtio">VirtIO</option>
						<option value="ide">IDE</option>
						<option value="sata">SATA</option>
						<option value="scsi">SCSI</option>
					</select>
				</div>
				<div class="form-group">
					<label for="profile_icon">Icon</label>
					<input id="profile_icon" type="text" bind:value={editForm.icon} disabled={saving} />
				</div>
			</div>
			<div class="form-row">
				<div class="form-group">
					<label for="profile_color">Color</label>
					<input id="profile_color" type="text" bind:value={editForm.color} disabled={saving} />
				</div>
				<div class="form-group checkbox">
					<input id="profile_enabled" type="checkbox" bind:checked={editForm.enabled} disabled={saving} />
					<label for="profile_enabled">Enabled</label>
				</div>
			</div>
			<div class="form-actions">
				<button onclick={handleSave} disabled={saving} class="btn-save">Save</button>
				<button onclick={cancelEdit} disabled={saving} class="btn-cancel">Cancel</button>
			</div>
		</div>
	{:else}
		<button onclick={() => (editingIndex = -1)} disabled={saving} class="btn-add">
			Add Profile
		</button>
	{/if}

	{#if error}
		<p class="error">{error}</p>
	{/if}

	<ul class="item-list">
		{#each items as item, index}
			<li class="item">
				<div class="item-details">
					<strong>{item.name}</strong>
					<span>ID: {item.id}</span>
					{#if item.description}
						<span>{item.description}</span>
					{/if}
					<span>
						{item.sockets} sockets, {item.cores} cores, {item.ram_gb}GB RAM, {item.disk_gb}GB disk
					</span>
					<span class="status {item.enabled ? 'enabled' : 'disabled'}">
						{item.enabled ? 'Enabled' : 'Disabled'}
					</span>
				</div>
				<div class="item-actions">
					<button onclick={() => handleToggle(index)} disabled={saving} class="btn-toggle">
						{item.enabled ? 'Disable' : 'Enable'}
					</button>
					<button onclick={() => startEdit(index)} disabled={saving} class="btn-edit">Edit</button>
					<button onclick={() => handleRemove(index)} disabled={saving} class="btn-remove">Remove</button>
				</div>
			</li>
		{/each}
		{#if items.length === 0}
			<li class="empty">No VM profiles configured</li>
		{/if}
	</ul>
</div>

<style>
	.vmprofiles-section {
		padding: 1rem;
		border: 1px solid #ddd;
		border-radius: 4px;
		margin-bottom: 1rem;
	}

	.vmprofiles-section h3 {
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

	.form-row {
		display: flex;
		gap: 1rem;
		margin-bottom: 0.75rem;
	}

	.form-row .form-group {
		flex: 1;
	}

	.form-group {
		margin-bottom: 0.75rem;
	}

	.form-group label {
		display: block;
		margin-bottom: 0.25rem;
		font-weight: 500;
	}

	.form-group input,
	.form-group select {
		width: 100%;
		padding: 0.5rem;
		border: 1px solid #ccc;
		border-radius: 4px;
	}

	.form-group.checkbox {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.form-group.checkbox input {
		width: auto;
	}

	.form-actions {
		display: flex;
		gap: 0.5rem;
	}

	.btn-add,
	.btn-save,
	.btn-cancel,
	.btn-edit,
	.btn-remove,
	.btn-toggle {
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

	.btn-remove {
		background-color: #d32f2f;
		color: white;
	}

	.btn-toggle {
		background-color: #388e3c;
		color: white;
	}

	.btn-add:disabled,
	.btn-save:disabled,
	.btn-cancel:disabled,
	.btn-edit:disabled,
	.btn-remove:disabled,
	.btn-toggle:disabled {
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
		align-items: flex-start;
		padding: 0.75rem;
		background: #f5f5f5;
		border-radius: 4px;
		margin-bottom: 0.5rem;
	}

	.item-details {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		flex: 1;
	}

	.item-details span {
		font-size: 0.875rem;
		color: #666;
	}

	.status {
		font-weight: 500;
	}

	.status.enabled {
		color: #388e3c;
	}

	.status.disabled {
		color: #d32f2f;
	}

	.item-actions {
		display: flex;
		gap: 0.5rem;
		flex-shrink: 0;
	}

	.empty {
		color: #999;
		font-style: italic;
		padding: 0.5rem;
	}
</style>
