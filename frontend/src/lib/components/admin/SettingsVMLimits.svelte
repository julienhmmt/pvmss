<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { upsertSettings, TABLE_VM_LIMITS } from '$lib/api/admin/settings-overview';
	import type { SectionMeta } from '$lib/api/admin/settings-overview';

	interface VMLimitsData {
		max_vms: number;
		max_vm_per_user: number;
		max_network_cards: number;
		max_disk_per_vm: number;
		allow_custom_yaml: boolean;
		max_snapshots: number;
	}

	let { meta, data, onUpdate }: { meta: SectionMeta; data: VMLimitsData; onUpdate: () => Promise<void> } = $props();

	let formState = $state({
		max_vms: 0,
		max_vm_per_user: 10,
		max_network_cards: 4,
		max_disk_per_vm: 8,
		allow_custom_yaml: false,
		max_snapshots: 5
	});

	$effect(() => {
		formState.max_vms = data?.max_vms ?? 0;
		formState.max_vm_per_user = data?.max_vm_per_user ?? 10;
		formState.max_network_cards = data?.max_network_cards ?? 4;
		formState.max_disk_per_vm = data?.max_disk_per_vm ?? 8;
		formState.allow_custom_yaml = data?.allow_custom_yaml ?? false;
		formState.max_snapshots = data?.max_snapshots ?? 5;
	});

	let saving = $state(false);
	let error = $state<string | null>(null);

	async function handleSave() {
		saving = true;
		error = null;
		try {
			await upsertSettings({ table: TABLE_VM_LIMITS, record: formState });
			await onUpdate();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save';
		} finally {
			saving = false;
		}
	}
</script>

<div class="rounded-xl border border-border bg-card p-5 space-y-4">
	<div class="flex items-start justify-between">
		<div>
			<p class="font-medium text-sm">{meta.name}</p>
			{#if meta.last_change_by}
				<p class="text-xs text-muted-foreground mt-0.5">
					{$t('admin.settings.overview.lastUpdated', { values: { user: meta.last_change_by, time: meta.last_change_at ? new Date(meta.last_change_at).toLocaleString() : '' } })}
				</p>
			{/if}
		</div>
	</div>

	<form onsubmit={(e) => { e.preventDefault(); handleSave(); }} class="space-y-3">
		<div class="grid gap-3 sm:grid-cols-2">
			<div class="space-y-1">
				<label for="max_vms" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.vmlimits.maxVms')}
				</label>
				<input id="max_vms" type="number" min="0" bind:value={formState.max_vms}
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="space-y-1">
				<label for="max_vm_per_user" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.vmlimits.maxVmsPerUser')}
				</label>
				<input id="max_vm_per_user" type="number" min="0" bind:value={formState.max_vm_per_user}
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="space-y-1">
				<label for="max_network_cards" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.vmlimits.maxNetworkCards')}
				</label>
				<input id="max_network_cards" type="number" min="1" max="10" bind:value={formState.max_network_cards}
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="space-y-1">
				<label for="max_disk_per_vm" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.vmlimits.maxDiskPerVm')}
				</label>
				<input id="max_disk_per_vm" type="number" min="0" bind:value={formState.max_disk_per_vm}
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="space-y-1">
				<label for="max_snapshots" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.vmlimits.maxSnapshots')}
				</label>
				<input id="max_snapshots" type="number" min="0" bind:value={formState.max_snapshots}
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="flex items-center gap-2 pt-4">
				<input id="allow_custom_yaml" type="checkbox" bind:checked={formState.allow_custom_yaml}
					class="h-4 w-4 rounded border-input" />
				<label for="allow_custom_yaml" class="text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.vmlimits.allowCustomYaml')}
				</label>
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
