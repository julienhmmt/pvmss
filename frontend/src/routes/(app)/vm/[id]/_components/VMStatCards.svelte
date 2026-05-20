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
</script>

<div class="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
	<div class="pv-stat-card pv-stat-card--icon">
		<Cpu class="pv-stat-icon" />
		<div>
			<div class="pv-stat-label">{$t('vms.cpu')}</div>
			<div class="pv-stat-value" transition:fade={{ duration: 150 }}>
				{#if isRunning && metrics}
					{Math.round(metrics.cpu * 100)}%
				{:else}
					{config?.cpus ?? 0} {$t('vm.cores')}
				{/if}
			</div>
		</div>
	</div>
	<div class="pv-stat-card pv-stat-card--icon">
		<Desktop class="pv-stat-icon" />
		<div>
			<div class="pv-stat-label">{$t('vms.ram')}</div>
			<div class="pv-stat-value" transition:fade={{ duration: 150 }}>
				{#if isRunning && metrics}
					{Math.round(metrics.memMb / 1024)} / {Math.round(metrics.maxMemMb / 1024)} GB
				{:else}
					{Math.round((config?.maxMemMb ?? 0) / 1024)} GB
				{/if}
			</div>
		</div>
	</div>
	<div class="pv-stat-card pv-stat-card--icon">
		<HardDrive class="pv-stat-icon" />
		<div>
			<div class="pv-stat-label">{$t('common.storage')}</div>
			<div class="pv-stat-value" transition:fade={{ duration: 150 }}>
				{config?.disks.reduce((s, d) => s + d.sizeGb, 0) ?? 0} GB
			</div>
		</div>
	</div>
	<div class="pv-stat-card pv-stat-card--icon">
		<Network class="pv-stat-icon" />
		<div>
			<div class="pv-stat-label">{$t('vm.interface')}</div>
			<div class="pv-stat-value" transition:fade={{ duration: 150 }}>
				{config?.networks.length ?? 0}
				{$t('vm.interfaces')}
			</div>
		</div>
	</div>
</div>
