<script lang="ts">
	import { t } from 'svelte-i18n';
	import { CaretLeft } from 'phosphor-svelte';
	import { goto } from '$app/navigation';
	import type { VMConfig, VMMetrics } from '$lib/api/vm-details';
	import EditableName from './EditableName.svelte';

	interface Props {
		config: VMConfig | null;
		metrics: VMMetrics | null;
		savingName: boolean;
		onSaveName: (value: string) => void;
	}

	let { config, metrics, savingName, onSaveName }: Props = $props();

	const status = $derived(metrics?.status ?? config?.status ?? 'stopped');
	const isRunning = $derived(status === 'running');
	const isStopped = $derived(status === 'stopped');
	const badgeClass = $derived(
		isRunning ? 'pv-badge--online' : isStopped ? 'pv-badge--offline' : 'pv-badge--warn'
	);
</script>

<div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
	<div class="flex min-w-0 items-center gap-3">
		<button
			class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
			onclick={() => goto('/home')}
		>
			<CaretLeft class="h-4 w-4" />
			{$t('nav.myVms')}
		</button>
		<span class="text-muted-foreground">/</span>

		{#if config}
			<EditableName
				value={config.name || `VM ${config.vmid}`}
				loading={savingName}
				onSave={onSaveName}
			/>
			<span class="pv-badge {badgeClass}">
				{$t(`common.statusMap.${status}`, { default: status })}
			</span>
		{/if}
	</div>

	{#if config}
		<div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
			<span class="font-mono">{$t('common.vmid')} {config.vmid}</span>
			<span class="hidden sm:inline">•</span>
			<span>{$t('common.node')}: <span class="font-medium text-foreground">{config.node}</span></span>
			{#if (metrics?.status ?? config.status) === 'running' && config.uptime}
				<span class="hidden sm:inline">•</span>
				<span>{$t('vm.uptime')}: <span class="font-medium text-foreground">{Math.floor(config.uptime / 3600)}h {Math.floor((config.uptime % 3600) / 60)}m</span></span>
			{/if}
			{#if config.tags}
				<span class="hidden sm:inline">•</span>
				<span class="flex items-center gap-1">
					{$t('common.tags')}:
					{#each config.tags.split(';').filter(Boolean) as tag (tag)}
						<span class="rounded bg-muted px-1.5 py-px text-xs text-foreground">{tag}</span>
					{/each}
				</span>
			{/if}
		</div>
	{/if}
</div>
