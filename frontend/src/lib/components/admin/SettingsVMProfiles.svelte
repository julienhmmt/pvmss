<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
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
		node?: string;
		storage?: string;
		icon: string;
		color: string;
		enabled: boolean;
	}

	const emptyForm = (): VMProfile => ({
		id: '',
		name: '',
		description: '',
		sockets: 1,
		cores: 2,
		ram_gb: 4,
		disk_gb: 32,
		disk_bus: 'virtio',
		node: '',
		storage: '',
		icon: 'Server',
		color: 'blue',
		enabled: true
	});

	let { meta, data, onUpdate }: { meta: SectionMeta; data: VMProfile[]; onUpdate: () => Promise<void> } = $props();

	let items = $derived([...(data ?? [])]);
	let editingId = $state<string | null>(null); // null=none, ''=new
	let editForm = $state<VMProfile>(emptyForm());
	let saving = $state(false);
	let error = $state<string | null>(null);

	function startNew() {
		editingId = '';
		editForm = emptyForm();
	}

	function startEdit(item: VMProfile) {
		editingId = item.id;
		editForm = { ...item };
	}

	function cancelEdit() {
		editingId = null;
		editForm = emptyForm();
		error = null;
	}

	async function handleSave() {
		if (!editForm.id || !editForm.name) { error = $t('admin.settings.overview.vmprofiles.errorIdName'); return; }
		if (editForm.sockets < 1) { error = $t('admin.settings.overview.vmprofiles.errorSockets'); return; }
		if (editForm.cores < 1) { error = $t('admin.settings.overview.vmprofiles.errorCores'); return; }
		if (editForm.ram_gb < 1) { error = $t('admin.settings.overview.vmprofiles.errorRam'); return; }
		if (editForm.disk_gb < 1) { error = $t('admin.settings.overview.vmprofiles.errorDisk'); return; }
		saving = true;
		error = null;
		try {
			await upsertSettings({ table: TABLE_VM_PROFILES, record: { ...editForm } });
			await onUpdate();
			cancelEdit();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save';
		} finally {
			saving = false;
		}
	}

	async function handleToggle(item: VMProfile) {
		saving = true;
		error = null;
		try {
			await upsertSettings({ table: TABLE_VM_PROFILES, record: { ...item, enabled: !item.enabled } });
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
			{#if meta.last_change_by}
				<p class="text-xs text-muted-foreground mt-0.5">
					{$t('admin.settings.overview.lastUpdated', { values: { user: meta.last_change_by, time: meta.last_change_at ? new Date(meta.last_change_at).toLocaleString() : '' } })}
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
				{editingId === '' ? $t('admin.settings.overview.vmprofiles.addProfile') : $t('admin.settings.overview.vmprofiles.editProfile')}
			</p>
			<div class="grid gap-3 sm:grid-cols-2">
				<div class="space-y-1">
					<label for="p_id" class="block text-xs font-medium text-muted-foreground">ID</label>
					<input id="p_id" type="text" bind:value={editForm.id} disabled={saving || editingId !== ''}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="p_name" class="block text-xs font-medium text-muted-foreground">{$t('common.name')}</label>
					<input id="p_name" type="text" bind:value={editForm.name} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="p_sockets" class="block text-xs font-medium text-muted-foreground">
						{$t('admin.settings.overview.vmprofiles.sockets')}
					</label>
					<input id="p_sockets" type="number" min="1" bind:value={editForm.sockets} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="p_cores" class="block text-xs font-medium text-muted-foreground">
						{$t('admin.settings.overview.vmprofiles.cores')}
					</label>
					<input id="p_cores" type="number" min="1" bind:value={editForm.cores} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="p_ram" class="block text-xs font-medium text-muted-foreground">RAM (GB)</label>
					<input id="p_ram" type="number" min="1" bind:value={editForm.ram_gb} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="p_disk" class="block text-xs font-medium text-muted-foreground">Disk (GB)</label>
					<input id="p_disk" type="number" min="1" bind:value={editForm.disk_gb} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="p_disk_bus" class="block text-xs font-medium text-muted-foreground">
						{$t('admin.settings.overview.vmprofiles.diskBus')}
					</label>
					<select id="p_disk_bus" bind:value={editForm.disk_bus} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50">
						<option value="virtio">VirtIO</option>
						<option value="scsi">SCSI</option>
						<option value="sata">SATA</option>
						<option value="ide">IDE</option>
					</select>
				</div>
				<div class="space-y-1">
					<label for="p_node" class="block text-xs font-medium text-muted-foreground">{$t('common.node')} (optional)</label>
					<input id="p_node" type="text" bind:value={editForm.node} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="p_storage" class="block text-xs font-medium text-muted-foreground">{$t('common.storage')} (optional)</label>
					<input id="p_storage" type="text" bind:value={editForm.storage} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1 sm:col-span-2">
					<label for="p_desc" class="block text-xs font-medium text-muted-foreground">{$t('common.description')}</label>
					<input id="p_desc" type="text" bind:value={editForm.description} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="p_icon" class="block text-xs font-medium text-muted-foreground">Icon</label>
					<input id="p_icon" type="text" bind:value={editForm.icon} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="p_color" class="block text-xs font-medium text-muted-foreground">Color</label>
					<input id="p_color" type="text" bind:value={editForm.color} disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="flex items-center gap-2 sm:col-span-2">
					<input id="p_enabled" type="checkbox" bind:checked={editForm.enabled}
						class="h-4 w-4 rounded border-input" />
					<label for="p_enabled" class="text-xs font-medium text-muted-foreground">{$t('common.enabled')}</label>
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
						<p class="text-xs text-muted-foreground">
							{item.sockets}s · {item.cores}c · {item.ram_gb}GB RAM · {item.disk_gb}GB {item.disk_bus}
						</p>
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
