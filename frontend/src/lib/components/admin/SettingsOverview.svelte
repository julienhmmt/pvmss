<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { Gear, WarningCircle, X, CaretDown } from 'phosphor-svelte';
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
	let noticeDismissed = $state(false);
	let expanded = $state(false);

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

<section class="rounded-xl border border-border bg-card">
	<!-- Accordion header — always visible -->
	<button
		type="button"
		onclick={() => {
			expanded = !expanded;
			if (expanded && !overview && !loading) loadOverview();
		}}
		class="w-full flex items-center justify-between gap-3 px-5 py-4 text-left"
		aria-expanded={expanded}
	>
		<div class="flex items-center gap-2">
			<div class="pv-resource-icon" style="width:28px;height:28px;">
				<Gear class="h-3.5 w-3.5" />
			</div>
			<div>
				<p class="text-sm font-semibold">{$t('admin.settings.overview.title')}</p>
				<p class="text-xs text-muted-foreground mt-0.5">{$t('admin.settings.overview.accordionHint')}</p>
			</div>
		</div>
		<CaretDown class="h-4 w-4 text-muted-foreground flex-shrink-0 transition-transform duration-200 {expanded ? 'rotate-180' : ''}" />
	</button>

	{#if expanded}
		<div class="border-t border-border px-5 pb-5 pt-4 space-y-6">
			<!-- Manual override notice -->
			{#if !noticeDismissed}
				<div class="flex items-start gap-3 rounded-lg border border-warning-soft-border bg-warning-soft px-4 py-3">
					<WarningCircle class="h-4 w-4 text-warning-soft-foreground flex-shrink-0 mt-0.5" />
					<p class="text-sm text-warning-soft-foreground flex-1">
						{$t('admin.settings.overview.manualOverrideNotice')}
					</p>
					<button
						type="button"
						onclick={() => (noticeDismissed = true)}
						class="text-warning-soft-foreground/70 hover:text-warning-soft-foreground flex-shrink-0"
						aria-label={$t('common.cancel')}
					>
						<X class="h-4 w-4" />
					</button>
				</div>
			{/if}

			{#if loading}
				<div class="py-8 text-center">
					<p class="text-sm text-muted-foreground">{$t('common.loading')}</p>
				</div>
			{:else if error}
				<div class="rounded-lg border border-destructive/50 bg-destructive/10 p-6 text-center">
					<p class="text-sm text-destructive">{error}</p>
					<button class="mt-3 text-sm text-destructive underline" onclick={loadOverview}>
						{$t('common.retry')}
					</button>
				</div>
			{:else if overview}
				<!-- Resources -->
				{#if getSectionsByCategory(CATEGORY_RESOURCES).length > 0}
					<div>
						<h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-3">
							{$t('admin.settings.overview.resources')}
						</h3>
						<div class="space-y-4">
							{#each getSectionsByCategory(CATEGORY_RESOURCES) as [table, section], i (i)}
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

				<!-- Inventory -->
				{#if getSectionsByCategory(CATEGORY_INVENTORY).length > 0}
					<div>
						<h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-3">
							{$t('admin.settings.overview.inventory')}
						</h3>
						<div class="space-y-4">
							{#each getSectionsByCategory(CATEGORY_INVENTORY) as [table, section], i (i)}
								{#if table === TABLE_ENABLED_NODES || table === TABLE_ENABLED_STORAGES || table === TABLE_ENABLED_ISOS || table === TABLE_ENABLED_VMBRS || table === TABLE_TAGS}
									<SettingsListSection meta={section} data={section.data as string[]} table={table} onUpdate={loadOverview} />
								{/if}
							{/each}
						</div>
					</div>
				{/if}

				<!-- Templates -->
				{#if getSectionsByCategory(CATEGORY_TEMPLATES).length > 0}
					<div>
						<h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-3">
							{$t('admin.settings.overview.templates')}
						</h3>
						<div class="space-y-4">
							{#each getSectionsByCategory(CATEGORY_TEMPLATES) as [table, section], i (i)}
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

				<!-- Integrations -->
				{#if getSectionsByCategory(CATEGORY_INTEGRATIONS).length > 0}
					<div>
						<h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-3">
							{$t('admin.settings.overview.integrations')}
						</h3>
						<div class="space-y-4">
							{#each getSectionsByCategory(CATEGORY_INTEGRATIONS) as [table, section], i (i)}
								{#if table === TABLE_SFTP_CONFIG}
									<SettingsSFTP meta={section} data={section.data as any} onUpdate={loadOverview} />
								{/if}
							{/each}
						</div>
					</div>
				{/if}
			{/if}
		</div>
	{/if}
</section>
