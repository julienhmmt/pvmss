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
	<div class="pv-stat-card">
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
	<div class="pv-stat-card">
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
	<div class="pv-stat-card">
		<HardDrive class="pv-stat-icon" />
		<div>
			<div class="pv-stat-label">{$t('common.storage')}</div>
			<div class="pv-stat-value" transition:fade={{ duration: 150 }}>
				{config?.disks.reduce((s, d) => s + d.sizeGb, 0) ?? 0} GB
			</div>
		</div>
	</div>
	<div class="pv-stat-card">
		<Network class="pv-stat-icon" />
		<div>
			<div class="pv-stat-label">{$t('admin.vmbr.iface')}</div>
			<div class="pv-stat-value" transition:fade={{ duration: 150 }}>
				{config?.networks.length ?? 0}
				{$t('vm.interfaces')}
			</div>
		</div>
	</div>
</div>

<style>
	.pv-stat-card {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.875rem 1rem;
		background: var(--card);
		border: 1px solid var(--border);
		border-radius: 0.5rem;
	}
	.pv-stat-label {
		font-size: 0.75rem;
		color: var(--muted-foreground);
		line-height: 1.2;
	}
	.pv-stat-value {
		font-size: 0.9rem;
		font-weight: 600;
		line-height: 1.4;
	}
</style>
