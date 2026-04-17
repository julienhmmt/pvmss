<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { Gear } from 'phosphor-svelte';
	import {
		getSettingsOverview,
		type OverviewResponse,
		CATEGORY_RESOURCES,
		CATEGORY_INVENTORY,
		CATEGORY_TEMPLATES,
		CATEGORY_INTEGRATIONS,
		TABLE_VM_LIMITS,
		TABLE_NODE_LIMITS,
		TABLE_ENABLED_NODES,
		TABLE_ENABLED_STORAGES,
		TABLE_ENABLED_ISOS,
		TABLE_ENABLED_VMBRS,
		TABLE_TAGS,
		TABLE_CLOUDINIT_TEMPLATES,
		TABLE_VM_PROFILES,
		TABLE_SFTP_CONFIG
	} from '$lib/api/admin/settings-overview';
	import SettingsVMLimits from './SettingsVMLimits.svelte';
	import SettingsListSection from './SettingsListSection.svelte';
	import SettingsNodeLimits from './SettingsNodeLimits.svelte';
	import SettingsCloudInit from './SettingsCloudInit.svelte';
	import SettingsVMProfiles from './SettingsVMProfiles.svelte';
	import SettingsSFTP from './SettingsSFTP.svelte';

	let loading = $state(true);
	let error = $state<string | null>(null);
	let overview: OverviewResponse | null = $state(null);

	async function loadOverview() {
		loading = true;
		error = null;
		try {
			overview = await getSettingsOverview();
		} catch (e) {
			error = (e as Error).message;
			toast.error((e as Error).message);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadOverview();
	});

	function getSectionsByCategory(category: string) {
		if (!overview) return [];
		return Object.entries(overview.sections).filter(
			([, section]) => section.category === category
		);
	}
</script>

<section>
	<div class="flex items-center gap-2 mb-4">
		<div class="pv-resource-icon" style="width:28px;height:28px;">
			<Gear class="h-3.5 w-3.5" />
		</div>
		<h2 class="text-base font-semibold">{$t('admin.settings.overview.title')}</h2>
	</div>

	{#if loading}
		<div class="rounded-xl border border-border bg-card p-8 text-center">
			<p class="text-sm text-muted-foreground">{$t('common.loading')}</p>
		</div>
	{:else if error}
		<div class="rounded-xl border border-destructive/50 bg-destructive/10 p-8 text-center">
			<p class="text-sm text-destructive">{error}</p>
			<button
				class="mt-4 text-sm text-destructive underline"
				onclick={loadOverview}
			>
				{$t('common.retry')}
			</button>
		</div>
	{:else if overview}
		<!-- Resources Category -->
		{#if getSectionsByCategory(CATEGORY_RESOURCES).length > 0}
			<div class="mb-6">
				<h3 class="text-lg font-semibold mb-3">Resources</h3>
				<div class="space-y-4">
					{#each getSectionsByCategory(CATEGORY_RESOURCES) as [table, section]}
						{#if table === TABLE_VM_LIMITS}
							<SettingsVMLimits meta={section} data={section.data as any} onUpdate={loadOverview} />
						{/if}
						{#if table === TABLE_NODE_LIMITS}
							<SettingsNodeLimits meta={section} data={section.data as any} onUpdate={loadOverview} />
						{/if}
					{/each}
				</div>
			</div>
		{/if}

		<!-- Inventory Category -->
		{#if getSectionsByCategory(CATEGORY_INVENTORY).length > 0}
			<div class="mb-6">
				<h3 class="text-lg font-semibold mb-3">Inventory</h3>
				<div class="space-y-4">
					{#each getSectionsByCategory(CATEGORY_INVENTORY) as [table, section]}
						{#if table === TABLE_ENABLED_NODES || table === TABLE_ENABLED_STORAGES || table === TABLE_ENABLED_ISOS || table === TABLE_ENABLED_VMBRS || table === TABLE_TAGS}
							<SettingsListSection meta={section} data={section.data as string[]} table={table} onUpdate={loadOverview} />
						{/if}
					{/each}
				</div>
			</div>
		{/if}

		<!-- Templates Category -->
		{#if getSectionsByCategory(CATEGORY_TEMPLATES).length > 0}
			<div class="mb-6">
				<h3 class="text-lg font-semibold mb-3">Templates</h3>
				<div class="space-y-4">
					{#each getSectionsByCategory(CATEGORY_TEMPLATES) as [table, section]}
						{#if table === TABLE_CLOUDINIT_TEMPLATES}
							<SettingsCloudInit meta={section} data={section.data as any} onUpdate={loadOverview} />
						{/if}
						{#if table === TABLE_VM_PROFILES}
							<SettingsVMProfiles meta={section} data={section.data as any} onUpdate={loadOverview} />
						{/if}
					{/each}
				</div>
			</div>
		{/if}

		<!-- Integrations Category -->
		{#if getSectionsByCategory(CATEGORY_INTEGRATIONS).length > 0}
			<div class="mb-6">
				<h3 class="text-lg font-semibold mb-3">Integrations</h3>
				<div class="space-y-4">
					{#each getSectionsByCategory(CATEGORY_INTEGRATIONS) as [table, section]}
						{#if table === TABLE_SFTP_CONFIG}
							<SettingsSFTP meta={section} data={section.data as any} onUpdate={loadOverview} />
						{/if}
					{/each}
				</div>
			</div>
		{/if}
	{/if}
</section>
