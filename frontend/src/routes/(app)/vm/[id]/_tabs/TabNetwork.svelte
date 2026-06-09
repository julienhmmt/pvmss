<script lang="ts">
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { CopySimple } from 'phosphor-svelte';
	import type { VMConfig } from '$lib/api/vm-details';

	interface Props {
		config: VMConfig;
	}

	let { config }: Props = $props();

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
		<span class="text-xs text-muted-foreground">
			{config.networks.length} {$t('vm.interfaces')}
		</span>
	</div>
	{#if config.networks.length === 0}
		<p class="py-8 text-center text-sm text-muted-foreground">{$t('vm.noNetworks')}</p>
	{:else}
		<table class="pv-table">
			<thead>
				<tr>
					<th>{$t('vm.interface')}</th>
					<th>{$t('vm.model')}</th>
					<th>{$t('admin.vmbr.iface')}</th>
					<th>MAC</th>
					<th>IP</th>
				</tr>
			</thead>
			<tbody>
				{#each config.networks as net, i (i)}
					<tr class="pv-row">
						<td class="pv-td-mono">{net.index || `net${i}`}</td>
						<td>{net.model || '—'}</td>
						<td class="pv-td-mono">{net.bridge || '—'}</td>
						<td class="pv-td-mono text-xs">
							{#if net.mac}
								<div class="pv-copy-cell">
									<span>{net.mac}</span>
									<button
										class="pv-copy-btn"
										onclick={() => copyToClipboard(net.mac, 'MAC')}
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
										onclick={() => copyToClipboard(net.ips!.join(', '), 'IP')}
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


