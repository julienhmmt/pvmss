<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
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

	let formState = $state({
		enabled: false,
		host: '',
		port: 22,
		username: '',
		private_key_path: '',
		remote_path: ''
	});

	$effect(() => {
		formState.enabled = data?.enabled ?? false;
		formState.host = data?.host ?? '';
		formState.port = data?.port ?? 22;
		formState.username = data?.username ?? '';
		formState.private_key_path = data?.private_key_path ?? '';
		formState.remote_path = data?.remote_path ?? '';
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

<div class="rounded-xl border border-border bg-card p-5 space-y-4">
	<div>
		<p class="font-medium text-sm">{meta.name}</p>
		{#if meta.last_change_by}
			<p class="text-xs text-muted-foreground mt-0.5">
				{$t('admin.settings.overview.lastUpdated', { values: { user: meta.last_change_by, time: meta.last_change_at ? new Date(meta.last_change_at).toLocaleString() : '' } })}
			</p>
		{/if}
	</div>

	<form onsubmit={(e) => { e.preventDefault(); handleSave(); }} class="space-y-3">
		<div class="flex items-center gap-2">
			<input id="sftp_enabled" type="checkbox" bind:checked={formState.enabled}
				class="h-4 w-4 rounded border-input" />
			<label for="sftp_enabled" class="text-xs font-medium text-muted-foreground">
				{$t('admin.settings.overview.sftp.enableUpload')}
			</label>
		</div>

		<div class="grid gap-3 sm:grid-cols-2">
			<div class="space-y-1">
				<label for="sftp_host" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.sftp.host')}
				</label>
				<input id="sftp_host" type="text" bind:value={formState.host} placeholder="pve.example.com"
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="space-y-1">
				<label for="sftp_port" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.sftp.port')}
				</label>
				<input id="sftp_port" type="number" min="1" max="65535" bind:value={formState.port}
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="space-y-1">
				<label for="sftp_username" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.sftp.username')}
				</label>
				<input id="sftp_username" type="text" bind:value={formState.username}
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="space-y-1">
				<label for="sftp_private_key" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.sftp.privateKeyPath')}
				</label>
				<input id="sftp_private_key" type="text" bind:value={formState.private_key_path} placeholder="/app/key"
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="space-y-1 sm:col-span-2">
				<label for="sftp_remote_path" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.sftp.remotePath')}
				</label>
				<input id="sftp_remote_path" type="text" bind:value={formState.remote_path} placeholder="/var/lib/vz/snippets"
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
		</div>

		{#if error}
			<p class="text-sm text-destructive">{error}</p>
		{/if}

		<Button type="submit" size="sm" disabled={saving}>
			{saving ? $t('common.saving') : $t('common.save')}
		</Button>
	</form>
</div>
