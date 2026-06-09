<script lang="ts">
	import { fade } from 'svelte/transition';
	import { t } from 'svelte-i18n';
	import { Cpu, Desktop, HardDrive, Network } from 'phosphor-svelte';
	import type { VMConfig, VMMetrics } from '$lib/api/vm-details';

	interface Props {
		config: VMConfig | null;
		metrics: VMMetrics | null;
	}

	let { config, metrics }: Props = $props();

	const status = $derived(metrics?.status ?? config?.status ?? 'stopped');
	const isRunning = $derived(status === 'running');

	const cpuPercent = $derived(isRunning && metrics ? Math.round(metrics.cpu * 100) : null);
	const ramUsedGb = $derived(isRunning && metrics ? Math.round(metrics.memMb / 1024) : null);
	const ramMaxGb = $derived(isRunning && metrics ? Math.round(metrics.maxMemMb / 1024) : Math.round((config?.maxMemMb ?? 0) / 1024));
	const totalStorageGb = $derived(config?.disks.reduce((s, d) => s + d.sizeGb, 0) ?? 0);
	const nicCount = $derived(config?.networks.length ?? 0);
</script>

<div class="mb-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
	<div class="pv-stat-card pv-stat-card--icon {isRunning ? 'pv-stat-card--running' : ''}">
		<Cpu class="pv-stat-icon" />
		<div class="min-w-0">
			<div class="pv-stat-label">{$t('vms.cpu')}</div>
			<div class="pv-stat-value" transition:fade={{ duration: 120 }}>
				{#if cpuPercent !== null}
					{cpuPercent}<span class="text-xs font-normal text-muted-foreground">%</span>
				{:else}
					{config?.cpus ?? 0} <span class="text-xs font-normal text-muted-foreground">{$t('vm.cores')}</span>
				{/if}
			</div>
		</div>
	</div>

	<div class="pv-stat-card pv-stat-card--icon">
		<Desktop class="pv-stat-icon" />
		<div class="min-w-0">
			<div class="pv-stat-label">{$t('vms.ram')}</div>
			<div class="pv-stat-value" transition:fade={{ duration: 120 }}>
				{#if ramUsedGb !== null}
					{ramUsedGb} / {ramMaxGb} <span class="text-xs font-normal text-muted-foreground">GB</span>
				{:else}
					{ramMaxGb} <span class="text-xs font-normal text-muted-foreground">GB</span>
				{/if}
			</div>
		</div>
	</div>

	<div class="pv-stat-card pv-stat-card--icon">
		<HardDrive class="pv-stat-icon" />
		<div class="min-w-0">
			<div class="pv-stat-label">{$t('common.storage')}</div>
			<div class="pv-stat-value" transition:fade={{ duration: 120 }}>
				{totalStorageGb} <span class="text-xs font-normal text-muted-foreground">GB</span>
			</div>
		</div>
	</div>

	<div class="pv-stat-card pv-stat-card--icon">
		<Network class="pv-stat-icon" />
		<div class="min-w-0">
			<div class="pv-stat-label">{$t('vm.interface')}</div>
			<div class="pv-stat-value" transition:fade={{ duration: 120 }}>
				{nicCount} <span class="text-xs font-normal text-muted-foreground">{$t('vm.interfaces')}</span>
			</div>
		</div>
	</div>
</div>
