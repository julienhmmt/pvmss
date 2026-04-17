<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { upsertSettings, TABLE_CLOUDINIT_TEMPLATES } from '$lib/api/admin/settings-overview';
	import type { SectionMeta } from '$lib/api/admin/settings-overview';

	interface CloudInitTemplate {
		id: string;
		name: string;
		description?: string;
		storage: string;
		filename: string;
		yamlContent: string;
		enabled: boolean;
	}

	const emptyForm = (): CloudInitTemplate => ({
		id: '',
		name: '',
		description: '',
		storage: '',
		filename: '',
		yamlContent: '#cloud-config\n',
		enabled: true
	});

	let { meta, data, onUpdate }: { meta: SectionMeta; data: CloudInitTemplate[]; onUpdate: () => Promise<void> } = $props();

	let items = $derived([...(data ?? [])]);
	let editingId = $state<string | null>(null); // null=none, ''=new
	let editForm = $state<CloudInitTemplate>(emptyForm());
	let saving = $state(false);
	let error = $state<string | null>(null);

	function startNew() {
		editingId = '';
		editForm = emptyForm();
	}

	function startEdit(item: CloudInitTemplate) {
		editingId = item.id;
		editForm = { ...item };
	}

	function cancelEdit() {
		editingId = null;
		editForm = emptyForm();
		error = null;
	}

	async function handleSave() {
		if (!editForm.id || !editForm.name) { error = $t('admin.settings.overview.cloudinit.errorIdName'); return; }
		if (!editForm.storage) { error = $t('admin.settings.overview.cloudinit.errorStorage'); return; }
		if (!editForm.filename) { error = $t('admin.settings.overview.cloudinit.errorFilename'); return; }
		saving = true;
		error = null;
		try {
			await upsertSettings({ table: TABLE_CLOUDINIT_TEMPLATES, record: { ...editForm } });
			await onUpdate();
			cancelEdit();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save';
		} finally {
			saving = false;
		}
	}

	async function handleToggle(item: CloudInitTemplate) {
		saving = true;
		error = null;
		try {
			await upsertSettings({ table: TABLE_CLOUDINIT_TEMPLATES, record: { ...item, enabled: !item.enabled } });
			await onUpdate();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to toggle';
		} finally {
			saving = false;
		}
	}
</script>

<div class="rounded-xl border border-border bg-card p-5 space-y-4">
	<div class="flex items-center justify-between">
		<div>
			<p class="font-medium text-sm">{meta.name}</p>
			{#if meta.lastChangeBy}
				<p class="text-xs text-muted-foreground mt-0.5">
					{$t('admin.settings.overview.lastUpdated', { values: { user: meta.lastChangeBy, time: meta.lastChangeAt ? new Date(meta.lastChangeAt).toLocaleString() : '' } })}
				</p>
			{/if}
		</div>
		{#if editingId === null}
			<Button size="sm" variant="outline" onclick={startNew} disabled={saving}>
				{$t('common.create')}
			</Button>
		{/if}
	</div>

	{#if editingId !== null}
		<div class="rounded-lg border border-border bg-muted/30 p-4 space-y-3">
			<p class="text-xs font-semibold uppercase text-muted-foreground tracking-wide">
				{editingId === '' ? $t('admin.settings.overview.cloudinit.addTemplate') : $t('admin.settings.overview.cloudinit.editTemplate')}
			</p>
			<div class="grid gap-3 sm:grid-cols-2">
				<div class="space-y-1">
					<label for="ci_id" class="block text-xs font-medium text-muted-foreground">ID</label>
					<input id="ci_id" type="text" bind:value={editForm.id} disabled={saving || editingId !== ''}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="ci_name" class="block text-xs font-medium text-muted-foreground">{$t('common.name')}</label>
					<input id="ci_name" type="text" bind:value={editForm.name} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="ci_storage" class="block text-xs font-medium text-muted-foreground">{$t('common.storage')}</label>
					<input id="ci_storage" type="text" bind:value={editForm.storage} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="ci_filename" class="block text-xs font-medium text-muted-foreground">
						{$t('admin.settings.overview.cloudinit.filenameLabel')}
					</label>
					<input id="ci_filename" type="text" bind:value={editForm.filename} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1 sm:col-span-2">
					<label for="ci_desc" class="block text-xs font-medium text-muted-foreground">{$t('common.description')}</label>
					<input id="ci_desc" type="text" bind:value={editForm.description} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1 sm:col-span-2">
					<label for="ci_yaml" class="block text-xs font-medium text-muted-foreground">YAML</label>
					<textarea id="ci_yaml" rows="8" bind:value={editForm.yamlContent} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm font-mono focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"></textarea>
				</div>
				<div class="flex items-center gap-2">
					<input id="ci_enabled" type="checkbox" bind:checked={editForm.enabled}
						class="h-4 w-4 rounded border-input" />
					<label for="ci_enabled" class="text-xs font-medium text-muted-foreground">{$t('common.enabled')}</label>
				</div>
			</div>
			{#if error}
				<p class="text-sm text-destructive">{error}</p>
			{/if}
			<div class="flex gap-2">
				<Button size="sm" onclick={handleSave} disabled={saving}>{$t('common.save')}</Button>
				<Button size="sm" variant="outline" onclick={cancelEdit} disabled={saving}>{$t('common.cancel')}</Button>
			</div>
		</div>
	{/if}

	{#if editingId === null && error}
		<p class="text-sm text-destructive">{error}</p>
	{/if}

	{#if items.length === 0}
		<p class="text-sm text-muted-foreground italic">{$t('common.noData')}</p>
	{:else}
		<ul class="space-y-1.5">
			{#each items as item}
				<li class="flex items-start justify-between rounded-lg border border-border bg-muted/30 px-3 py-2 gap-3">
					<div class="space-y-0.5 min-w-0">
						<p class="text-sm font-medium truncate">{item.name}</p>
						<p class="text-xs text-muted-foreground">{item.id}{item.description ? ` · ${item.description}` : ''}</p>
						<span class="inline-block text-xs font-medium {item.enabled ? 'text-green-600 dark:text-green-400' : 'text-muted-foreground'}">
							{item.enabled ? $t('admin.settings.overview.enabled') : $t('admin.settings.overview.disabled')}
						</span>
					</div>
					<div class="flex gap-1.5 flex-shrink-0">
						<Button size="sm" variant="ghost" onclick={() => handleToggle(item)} disabled={saving || editingId !== null}>
							{item.enabled ? $t('admin.settings.overview.disable') : $t('admin.settings.overview.enable')}
						</Button>
						<Button size="sm" variant="ghost" onclick={() => startEdit(item)} disabled={saving || editingId !== null}>
							{$t('common.edit')}
						</Button>
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</div>
