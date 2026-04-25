<script lang="ts">
	import { t } from 'svelte-i18n';
	import type { VMConfig } from '$lib/api/vm-details';

	interface Props {
		config: VMConfig;
	}

	let { config }: Props = $props();
</script>

<div class="pv-table-wrap">
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
						<td class="pv-td-mono text-xs">{net.mac || '—'}</td>
						<td class="pv-td-mono text-xs">
							{#if net.ips && net.ips.length > 0}
								{net.ips.join(', ')}
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
