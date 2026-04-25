<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Lock } from 'phosphor-svelte';
	import type { VMConfig, VMMetrics } from '$lib/api/vm-details';

	interface Props {
		config: VMConfig;
		metrics: VMMetrics | null;
		onOpenHardware: () => void;
	}

	let { config, metrics, onOpenHardware }: Props = $props();

	function statusClass(status: string): string {
		if (status === 'running') return 'pv-badge--online';
		if (status === 'stopped') return 'pv-badge--offline';
		return 'pv-badge--warn';
	}

	function uptimeLabel(seconds: number): string {
		if (!seconds) return '—';
		const d = Math.floor(seconds / 86400);
		const h = Math.floor((seconds % 86400) / 3600);
		if (d > 0) return `${d}d ${h}h`;
		const m = Math.floor((seconds % 3600) / 60);
		return h > 0 ? `${h}h ${m}m` : `${m}m`;
	}

	const status = $derived(metrics?.status ?? config.status);
</script>

<div class="pv-table-wrap">
	<table class="pv-table">
		<tbody>
			<tr class="pv-row">
				<td class="pv-td-label">VMID</td>
				<td class="pv-td-mono">{config.vmid}</td>
			</tr>
			<tr class="pv-row">
				<td class="pv-td-label">{$t('common.name')}</td>
				<td>{config.name || '—'}</td>
			</tr>
			<tr class="pv-row">
				<td class="pv-td-label">{$t('common.node')}</td>
				<td class="pv-td-mono">{config.node}</td>
			</tr>
			<tr class="pv-row">
				<td class="pv-td-label">{$t('common.status')}</td>
				<td>
					<span class="pv-badge {statusClass(status)}">
						{$t(`common.statusMap.${status}`, { default: status })}
					</span>
				</td>
			</tr>
			<tr class="pv-row">
				<td class="pv-td-label">{$t('vms.uptime')}</td>
				<td class="pv-td-mono">{uptimeLabel(config.uptime)}</td>
			</tr>
			<tr class="pv-row">
				<td class="pv-td-label">{$t('admin.vms.cpus')}</td>
				<td class="pv-td-mono">{config.cpus}</td>
			</tr>
			<tr class="pv-row">
				<td class="pv-td-label">{$t('vms.ram')}</td>
				<td class="pv-td-mono">{Math.round(config.maxMemMb / 1024)} GB</td>
			</tr>
			<tr class="pv-row">
				<td class="pv-td-label">{$t('admin.vms.tags')}</td>
				<td>
					{#if config.tags}
						<div class="flex flex-wrap gap-1">
							{#each config.tags.split(';').filter(Boolean) as tag (tag)}
								<span class="pv-badge">{tag.trim()}</span>
							{/each}
						</div>
					{:else}
						<span class="text-muted-foreground">—</span>
					{/if}
				</td>
			</tr>
			{#if config.efiEnabled}
				<tr class="pv-row">
					<td class="pv-td-label">{$t('vm.efi')}</td>
					<td>
						<Lock class="inline h-4 w-4 text-green-600" />
						{$t('common.enabled')}
					</td>
				</tr>
			{/if}
			{#if config.tpmEnabled}
				<tr class="pv-row">
					<td class="pv-td-label">{$t('vm.tpm')}</td>
					<td>
						<Lock class="inline h-4 w-4 text-green-600" />
						{$t('common.enabled')}
					</td>
				</tr>
			{/if}
		</tbody>
	</table>
	<div class="border-t border-border px-4 py-3">
		<button
			class="inline-flex items-center gap-1 text-sm text-primary hover:underline"
			onclick={onOpenHardware}
		>
			{$t('vm.hardware.modifyHardware')}
		</button>
	</div>
</div>
