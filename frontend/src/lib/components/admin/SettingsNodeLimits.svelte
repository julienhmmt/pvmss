<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { upsertSettings, TABLE_NODE_LIMITS } from '$lib/api/admin/settings-overview';
	import type { SectionMeta } from '$lib/api/admin/settings-overview';

	interface NodeLimit {
		node: string;
		max_vms: number;
	}

	let { meta, data, onUpdate }: { meta: SectionMeta; data: NodeLimit[]; onUpdate: () => Promise<void> } = $props();

	let items = $derived([...(data ?? [])]);
	let editingNode = $state<string | null>(null); // null = not editing, '' = new item
	let editForm = $state<NodeLimit>({ node: '', max_vms: 0 });
	let saving = $state(false);
	let error = $state<string | null>(null);

	function startNew() {
		editingNode = '';
		editForm = { node: '', max_vms: 0 };
	}

	function startEdit(item: NodeLimit) {
		editingNode = item.node;
		editForm = { ...item };
	}

	function cancelEdit() {
		editingNode = null;
		editForm = { node: '', max_vms: 0 };
	}

	async function handleSave() {
		if (!editForm.node) { error = $t('admin.settings.overview.nodelimits.errorNode'); return; }
		if (editForm.max_vms < 0) { error = $t('admin.settings.overview.nodelimits.errorMaxVms'); return; }
		saving = true;
		error = null;
		try {
			await upsertSettings({ table: TABLE_NODE_LIMITS, record: { node: editForm.node, max_vms: editForm.max_vms } });
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
			{#if meta.last_change_by}
				<p class="text-xs text-muted-foreground mt-0.5">
					{$t('admin.settings.overview.lastUpdated', { values: { user: meta.last_change_by, time: meta.last_change_at ? new Date(meta.last_change_at).toLocaleString() : '' } })}
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
						class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50" />
				</div>
				<div class="space-y-1">
					<label for="node_max_vms" class="block text-xs font-medium text-muted-foreground">
						{$t('admin.settings.overview.nodelimits.maxVms')}
					</label>
					<input id="node_max_vms" type="number" min="0" bind:value={editForm.max_vms}
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

	{#if editingNode === null && error}
		<p class="text-sm text-destructive">{error}</p>
	{/if}

	{#if items.length === 0}
		<p class="text-sm text-muted-foreground">{$t('admin.settings.overview.nodelimits.emptyHint')}</p>
	{:else}
		<ul class="space-y-1.5">
			{#each items as item}
				<li class="flex items-center justify-between rounded-lg border border-border bg-muted/30 px-3 py-2">
					<div>
						<span class="text-sm font-medium">{item.node}</span>
						<span class="text-xs text-muted-foreground ml-2">max {item.max_vms} VMs</span>
					</div>
					<Button size="sm" variant="ghost" onclick={() => startEdit(item)} disabled={saving || editingNode !== null}>
						{$t('common.edit')}
					</Button>
				</li>
			{/each}
		</ul>
	{/if}
</div>
