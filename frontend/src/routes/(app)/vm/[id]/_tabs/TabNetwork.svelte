<script lang="ts">
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { CopySimple, Network } from 'phosphor-svelte';
	import type { VMConfig, VMMetrics } from '$lib/api/vm-details';

	interface Props {
		config: VMConfig;
		metrics?: VMMetrics | null;
		onOpenHardware?: () => void;
	}

	let { config, metrics, onOpenHardware }: Props = $props();

	const isRunning = $derived((metrics?.status ?? config.status) === 'running');

	async function copyToClipboard(text: string, label: string): Promise<void> {
		try {
			await navigator.clipboard.writeText(text);
			toast.success($t('common.copied', { values: { value: label } }));
		} catch {
			toast.error($t('common.copyFailed'));
		}
	}
</script>

<div class="pv-table-wrap">
	<div class="flex items-center justify-between border-b border-border px-4 py-3">
		<span class="text-sm font-medium">{$t('vm.tabNetwork')}</span>
		<div class="flex items-center gap-2">
			<span class="text-xs text-muted-foreground">
				{config.networks.length} {$t('vm.interfaceCount', { values: { count: config.networks.length } })}
			</span>
			{#if onOpenHardware}
				<button
					class="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
					onclick={onOpenHardware}
					disabled={isRunning}
					title={isRunning ? $t('vm.disk.vmRunningWarning') : ''}
				>
					+ {$t('vm.network.addCard')}
				</button>
			{/if}
		</div>
	</div>
	{#if config.networks.length === 0}
		<div class="flex flex-col items-center py-10 text-center text-muted-foreground">
			<Network class="mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('vm.noNetworks')}</p>
		</div>
	{:else}
		<table class="pv-table">
			<thead>
				<tr>
					<th>{$t('vm.interface')}</th>
					<th>{$t('vm.model')}</th>
					<th>{$t('admin.vmbr.iface')}</th>
					<th>{$t('common.mac')}</th>
					<th>{$t('common.ip')}</th>
				</tr>
			</thead>
			<tbody>
				{#each config.networks as net, i (i)}
					<tr class="pv-row">
						<td>
							<div class="flex items-center gap-2">
								<Network class="h-4 w-4 text-muted-foreground" />
								<span class="text-sm pv-td-mono">{net.index || `net${i}`}</span>
							</div>
						</td>
						<td>{net.model || '—'}</td>
						<td class="pv-td-mono">{net.bridge || '—'}</td>
						<td class="pv-td-mono text-xs">
							{#if net.mac}
								<div class="pv-copy-cell">
									<span>{net.mac}</span>
									<button
										class="pv-copy-btn"
										onclick={() => copyToClipboard(net.mac, $t('common.mac'))}
										title={$t('common.copy')}
										aria-label={$t('common.copy')}
									>
										<CopySimple class="h-3 w-3" />
									</button>
								</div>
							{:else}
								<span class="text-muted-foreground">—</span>
							{/if}
						</td>
						<td class="pv-td-mono text-xs">
							{#if net.ips && net.ips.length > 0}
								<div class="pv-copy-cell">
									<span>{net.ips.join(', ')}</span>
									<button
										class="pv-copy-btn"
										onclick={() => copyToClipboard(net.ips!.join(', '), $t('common.ip'))}
										title={$t('common.copy')}
										aria-label={$t('common.copy')}
									>
										<CopySimple class="h-3 w-3" />
									</button>
								</div>
							{:else}
								<span class="text-muted-foreground">—</span>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</div>


