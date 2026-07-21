<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { upsertSettings, TABLE_NODE_LIMITS } from '$lib/api/admin/settings-overview';
	import type { SectionMeta } from '$lib/api/admin/settings-overview';

	interface NodeLimit {
		node: string;
		maxVms: number;
		maxVcpus: number;
		maxRamGb: number;
		maxDiskGb: number;
	}

	const EMPTY_FORM: NodeLimit = { node: '', maxVms: 0, maxVcpus: 0, maxRamGb: 0, maxDiskGb: 0 };

	let { meta, data, onUpdate }: { meta: SectionMeta; data: NodeLimit[]; onUpdate: () => Promise<void> } = $props();

	let items = $derived([...(data ?? [])]);
	let editingNode = $state<string | null>(null); // null = not editing, '' = new item
	let editForm = $state<NodeLimit>({ ...EMPTY_FORM });
	let saving = $state(false);
	let error = $state<string | null>(null);

	function startNew() {
		editingNode = '';
		editForm = { ...EMPTY_FORM };
	}

	function startEdit(item: NodeLimit) {
		editingNode = item.node;
		editForm = { ...item };
	}

	function cancelEdit() {
		editingNode = null;
		editForm = { ...EMPTY_FORM };
		error = null;
	}

	async function handleSave() {
		if (!editForm.node) { error = $t('admin.settings.overview.nodelimits.errorNode'); return; }
		if (editForm.maxVms < 0) { error = $t('admin.settings.overview.nodelimits.errorMaxVms'); return; }
		if (editForm.maxVcpus < 0) { error = $t('admin.settings.overview.nodelimits.errorMaxVcpus'); return; }
		if (editForm.maxRamGb < 0) { error = $t('admin.settings.overview.nodelimits.errorMaxRam'); return; }
		if (editForm.maxDiskGb < 0) { error = $t('admin.settings.overview.nodelimits.errorMaxDisk'); return; }
		saving = true;
		error = null;
		try {
			await upsertSettings({ table: TABLE_NODE_LIMITS, record: { ...editForm } });
			await onUpdate();
			cancelEdit();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save';
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
		{#if editingNode === null}
			<Button size="sm" variant="outline" onclick={startNew} disabled={saving}>
				{$t('common.create')}
			</Button>
		{/if}
	</div>

	{#if editingNode !== null}
		<div class="rounded-lg border border-border bg-muted/30 p-4 space-y-3">
			<p class="text-xs font-semibold uppercase text-muted-foreground tracking-wide">
				{editingNode === '' ? $t('admin.settings.overview.nodelimits.addNodeLimit') : $t('admin.settings.overview.nodelimits.editNodeLimit')}
			</p>
			<div class="grid gap-3 sm:grid-cols-2">
				<div class="space-y-1">
					<label for="node_name" class="block text-xs font-medium text-muted-foreground">
						{$t('common.node')}
					</label>
					<input id="node_name" type="text" bind:value={editForm.node}
						disabled={saving || editingNode !== ''}
						placeholder="pve1"
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="node_max_vms" class="block text-xs font-medium text-muted-foreground">
						{$t('admin.settings.overview.nodelimits.maxVms')}
						<span class="text-muted-foreground/60 font-normal ml-1">{$t('admin.settings.overview.nodelimits.zeroUnlimited')}</span>
					</label>
					<input id="node_max_vms" type="number" min="0" bind:value={editForm.maxVms}
						disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="node_max_vcpus" class="block text-xs font-medium text-muted-foreground">
						{$t('admin.settings.overview.nodelimits.maxVcpus')}
						<span class="text-muted-foreground/60 font-normal ml-1">{$t('admin.settings.overview.nodelimits.zeroUnlimited')}</span>
					</label>
					<input id="node_max_vcpus" type="number" min="0" bind:value={editForm.maxVcpus}
						disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="node_max_ram_gb" class="block text-xs font-medium text-muted-foreground">
						{$t('admin.settings.overview.nodelimits.maxRamGb')}
						<span class="text-muted-foreground/60 font-normal ml-1">{$t('admin.settings.overview.nodelimits.zeroUnlimited')}</span>
					</label>
					<input id="node_max_ram_gb" type="number" min="0" bind:value={editForm.maxRamGb}
						disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="node_max_disk_gb" class="block text-xs font-medium text-muted-foreground">
						{$t('admin.settings.overview.nodelimits.maxDiskGb')}
						<span class="text-muted-foreground/60 font-normal ml-1">{$t('admin.settings.overview.nodelimits.zeroUnlimited')}</span>
					</label>
					<input id="node_max_disk_gb" type="number" min="0" bind:value={editForm.maxDiskGb}
						disabled={saving}
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
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

	{#if items.length === 0}
		<p class="text-sm text-muted-foreground">{$t('admin.settings.overview.nodelimits.emptyHint')}</p>
	{:else}
		<ul class="space-y-1.5">
			{#each items as item, i (i)}
				<li class="flex items-start justify-between rounded-lg border border-border bg-muted/30 px-3 py-2 gap-3">
					<div class="min-w-0">
						<span class="text-sm font-medium">{item.node}</span>
						<div class="flex flex-wrap gap-x-3 gap-y-0.5 mt-0.5">
							{#if item.maxVms > 0}
								<span class="text-xs text-muted-foreground">{$t('admin.settings.overview.nodelimits.maxVms')}: {item.maxVms}</span>
							{/if}
							{#if item.maxVcpus > 0}
								<span class="text-xs text-muted-foreground">{$t('admin.settings.overview.nodelimits.maxVcpus')}: {item.maxVcpus}</span>
							{/if}
							{#if item.maxRamGb > 0}
								<span class="text-xs text-muted-foreground">{$t('admin.settings.overview.nodelimits.maxRamGb')}: {item.maxRamGb} {$t('common.gb')}</span>
							{/if}
							{#if item.maxDiskGb > 0}
								<span class="text-xs text-muted-foreground">{$t('admin.settings.overview.nodelimits.maxDiskGb')}: {item.maxDiskGb} {$t('common.gb')}</span>
							{/if}
						</div>
					</div>
					<Button size="sm" variant="ghost" onclick={() => startEdit(item)} disabled={saving || editingNode !== null}>
						{$t('common.edit')}
					</Button>
				</li>
			{/each}
		</ul>
	{/if}
</div>
