<script lang="ts">
	import { upsertSettings, TABLE_VM_LIMITS } from '$lib/api/admin/settings-overview';
	import type { SectionMeta } from '$lib/api/admin/settings-overview';

	let { meta, data, onUpdate }: { meta: SectionMeta; data: any; onUpdate: () => Promise<void> } = $props();

	let initialValues = $derived({
		max_vms: data?.max_vms ?? 0,
		max_vm_per_user: data?.max_vm_per_user ?? 10,
		max_network_cards: data?.max_network_cards ?? 4,
		max_disk_per_vm: data?.max_disk_per_vm ?? 8,
		allow_custom_yaml: data?.allow_custom_yaml ?? false,
		max_snapshots: data?.max_snapshots ?? 5
	});

	let formState = $state({
		max_vms: 0,
		max_vm_per_user: 10,
		max_network_cards: 4,
		max_disk_per_vm: 8,
		allow_custom_yaml: false,
		max_snapshots: 5
	});

	// Sync formState when initialValues changes (e.g., after reload)
	$effect(() => {
		formState.max_vms = initialValues.max_vms;
		formState.max_vm_per_user = initialValues.max_vm_per_user;
		formState.max_network_cards = initialValues.max_network_cards;
		formState.max_disk_per_vm = initialValues.max_disk_per_vm;
		formState.allow_custom_yaml = initialValues.allow_custom_yaml;
		formState.max_snapshots = initialValues.max_snapshots;
	});

	let saving = $state(false);
	let error = $state<string | null>(null);

	async function handleSave() {
		saving = true;
		error = null;
		try {
			await upsertSettings({
				table: TABLE_VM_LIMITS,
				record: formState
			});
			await onUpdate();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save';
		} finally {
			saving = false;
		}
	}
</script>

<div class="vmlimits-section">
	<h3>VM Limits</h3>
	{#if meta.last_change_by}
		<p class="audit-info">
			Last updated by {meta.last_change_by} at {new Date(meta.last_change_at || '').toLocaleString()}
		</p>
	{/if}

	<form onsubmit={handleSave}>
		<div class="form-group">
			<label for="max_vms">Max VMs (0 = unlimited)</label>
			<input id="max_vms" type="number" min="0" bind:value={formState.max_vms} />
		</div>

		<div class="form-group">
			<label for="max_vm_per_user">Max VMs per User</label>
			<input id="max_vm_per_user" type="number" min="0" bind:value={formState.max_vm_per_user} />
		</div>

		<div class="form-group">
			<label for="max_network_cards">Max Network Cards per VM</label>
			<input id="max_network_cards" type="number" min="1" max="10" bind:value={formState.max_network_cards} />
		</div>

		<div class="form-group">
			<label for="max_disk_per_vm">Max Disks per VM</label>
			<input id="max_disk_per_vm" type="number" min="0" bind:value={formState.max_disk_per_vm} />
		</div>

		<div class="form-group">
			<label for="max_snapshots">Max Snapshots per VM</label>
			<input id="max_snapshots" type="number" min="0" bind:value={formState.max_snapshots} />
		</div>

		<div class="form-group checkbox">
			<input id="allow_custom_yaml" type="checkbox" bind:checked={formState.allow_custom_yaml} />
			<label for="allow_custom_yaml">Allow Custom YAML</label>
		</div>

		{#if error}
			<p class="error">{error}</p>
		{/if}

		<button type="submit" disabled={saving} class="btn-primary">
			{saving ? 'Saving...' : 'Save'}
		</button>
	</form>
</div>

<style>
	.vmlimits-section {
		padding: 1rem;
		border: 1px solid #ddd;
		border-radius: 4px;
		margin-bottom: 1rem;
	}

	.vmlimits-section h3 {
		margin-top: 0;
		margin-bottom: 0.5rem;
	}

	.audit-info {
		font-size: 0.875rem;
		color: #666;
		margin-bottom: 1rem;
	}

	.form-group {
		margin-bottom: 1rem;
	}

	.form-group label {
		display: block;
		margin-bottom: 0.25rem;
		font-weight: 500;
	}

	.form-group input[type='number'] {
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

	.error {
		color: #d32f2f;
		margin-bottom: 1rem;
	}

	.btn-primary {
		padding: 0.5rem 1rem;
		background-color: #1976d2;
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}

	.btn-primary:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}
</style>
