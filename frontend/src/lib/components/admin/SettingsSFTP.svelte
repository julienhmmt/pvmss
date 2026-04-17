<script lang="ts">
	import { upsertSettings, TABLE_SFTP_CONFIG } from '$lib/api/admin/settings-overview';
	import type { SectionMeta } from '$lib/api/admin/settings-overview';

	interface SFTPConfig {
		enabled: boolean;
		host: string;
		port: number;
		username: string;
		private_key_path: string;
		remote_path: string;
	}

	let { meta, data, onUpdate }: { meta: SectionMeta; data: SFTPConfig; onUpdate: () => Promise<void> } = $props();

	let initialValues = $derived({
		enabled: data?.enabled ?? false,
		host: data?.host ?? '',
		port: data?.port ?? 22,
		username: data?.username ?? '',
		private_key_path: data?.private_key_path ?? '',
		remote_path: data?.remote_path ?? ''
	});

	let formState = $state({
		enabled: false,
		host: '',
		port: 22,
		username: '',
		private_key_path: '',
		remote_path: ''
	});

	// Sync formState when initialValues changes (e.g., after reload)
	$effect(() => {
		formState.enabled = initialValues.enabled;
		formState.host = initialValues.host;
		formState.port = initialValues.port;
		formState.username = initialValues.username;
		formState.private_key_path = initialValues.private_key_path;
		formState.remote_path = initialValues.remote_path;
	});

	let saving = $state(false);
	let error = $state<string | null>(null);

	async function handleSave() {
		saving = true;
		error = null;
		try {
			await upsertSettings({ table: TABLE_SFTP_CONFIG, record: formState });
			await onUpdate();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save';
		} finally {
			saving = false;
		}
	}
</script>

<div class="sftp-section">
	<h3>{meta.name}</h3>
	{#if meta.last_change_by}
		<p class="audit-info">
			Last updated by {meta.last_change_by} at {new Date(meta.last_change_at || '').toLocaleString()}
		</p>
	{/if}

	<form onsubmit={handleSave}>
		<div class="form-group checkbox">
			<input id="sftp_enabled" type="checkbox" bind:checked={formState.enabled} />
			<label for="sftp_enabled">Enable SFTP Upload</label>
		</div>

		<div class="form-group">
			<label for="sftp_host">Host</label>
			<input id="sftp_host" type="text" bind:value={formState.host} placeholder="pve.example.com" />
		</div>

		<div class="form-group">
			<label for="sftp_port">Port</label>
			<input id="sftp_port" type="number" min="1" max="65535" bind:value={formState.port} />
		</div>

		<div class="form-group">
			<label for="sftp_username">Username</label>
			<input id="sftp_username" type="text" bind:value={formState.username} />
		</div>

		<div class="form-group">
			<label for="sftp_private_key">Private Key Path</label>
			<input id="sftp_private_key" type="text" bind:value={formState.private_key_path} placeholder="/app/key" />
		</div>

		<div class="form-group">
			<label for="sftp_remote_path">Remote Path</label>
			<input id="sftp_remote_path" type="text" bind:value={formState.remote_path} placeholder="/var/lib/vz/snippets" />
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
	.sftp-section {
		padding: 1rem;
		border: 1px solid #ddd;
		border-radius: 4px;
		margin-bottom: 1rem;
	}

	.sftp-section h3 {
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

	.form-group input[type='text'],
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
