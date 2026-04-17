<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { upsertSettings, TABLE_VM_LIMITS } from '$lib/api/admin/settings-overview';
	import type { SectionMeta } from '$lib/api/admin/settings-overview';

	interface VMLimitsData {
		maxVms: number;
		maxVmPerUser: number;
		maxNetworkCards: number;
		maxDiskPerVm: number;
		allowCustomYaml: boolean;
		maxSnapshots: number;
	}

	let { meta, data, onUpdate }: { meta: SectionMeta; data: VMLimitsData; onUpdate: () => Promise<void> } = $props();

	let formState = $state({
		maxVms: 0,
		maxVmPerUser: 10,
		maxNetworkCards: 4,
		maxDiskPerVm: 8,
		allowCustomYaml: false,
		maxSnapshots: 5
	});

	$effect(() => {
		formState.maxVms = data?.maxVms ?? 0;
		formState.maxVmPerUser = data?.maxVmPerUser ?? 10;
		formState.maxNetworkCards = data?.maxNetworkCards ?? 4;
		formState.maxDiskPerVm = data?.maxDiskPerVm ?? 8;
		formState.allowCustomYaml = data?.allowCustomYaml ?? false;
		formState.maxSnapshots = data?.maxSnapshots ?? 5;
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
			{#if meta.lastChangeBy}
				<p class="text-xs text-muted-foreground mt-0.5">
					{$t('admin.settings.overview.lastUpdated', { values: { user: meta.lastChangeBy, time: meta.lastChangeAt ? new Date(meta.lastChangeAt).toLocaleString() : '' } })}
				</p>
			{/if}
		</div>
	</div>

	<form onsubmit={(e) => { e.preventDefault(); handleSave(); }} class="space-y-3">
		<div class="grid gap-3 sm:grid-cols-2">
			<div class="space-y-1">
				<label for="maxVms" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.vmlimits.maxVms')}
				</label>
				<input id="maxVms" type="number" min="0" bind:value={formState.maxVms}
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="space-y-1">
				<label for="maxVmPerUser" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.vmlimits.maxVmsPerUser')}
				</label>
				<input id="maxVmPerUser" type="number" min="0" bind:value={formState.maxVmPerUser}
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="space-y-1">
				<label for="maxNetworkCards" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.vmlimits.maxNetworkCards')}
				</label>
				<input id="maxNetworkCards" type="number" min="1" max="10" bind:value={formState.maxNetworkCards}
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="space-y-1">
				<label for="maxDiskPerVm" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.vmlimits.maxDiskPerVm')}
				</label>
				<input id="maxDiskPerVm" type="number" min="0" bind:value={formState.maxDiskPerVm}
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="space-y-1">
				<label for="maxSnapshots" class="block text-xs font-medium text-muted-foreground">
					{$t('admin.settings.overview.vmlimits.maxSnapshots')}
				</label>
				<input id="maxSnapshots" type="number" min="0" bind:value={formState.maxSnapshots}
					class="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
			</div>
			<div class="flex items-center gap-2 pt-4">
				<input id="allowCustomYaml" type="checkbox" bind:checked={formState.allowCustomYaml}
					class="h-4 w-4 rounded border-input" />
				<label for="allowCustomYaml" class="text-xs font-medium text-muted-foreground">
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
