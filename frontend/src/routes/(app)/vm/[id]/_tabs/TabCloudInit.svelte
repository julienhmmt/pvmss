<script lang="ts">
	import { t } from 'svelte-i18n';
	import { CloudArrowUp } from 'phosphor-svelte';
	import type { VMConfig } from '$lib/api/vm-details';

	interface Props {
		config: VMConfig;
	}

	let { config }: Props = $props();
</script>

<div class="pv-table-wrap">
	{#if !config.cloudInit}
		<div class="flex flex-col items-center py-12 text-muted-foreground">
			<CloudArrowUp class="mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('vm.noCloudInit')}</p>
		</div>
	{:else}
		<table class="pv-table">
			<tbody>
				{#if config.cloudInit.user}
					<tr class="pv-row">
						<td class="pv-td-label">{$t('admin.cloudinit.username')}</td>
						<td class="pv-td-mono">{config.cloudInit.user}</td>
					</tr>
				{/if}
				{#if config.cloudInit.ipConfig}
					<tr class="pv-row">
						<td class="pv-td-label">{$t('vm.ipConfig')}</td>
						<td class="pv-td-mono text-xs">{config.cloudInit.ipConfig}</td>
					</tr>
				{/if}
				{#if config.cloudInit.nameserver}
					<tr class="pv-row">
						<td class="pv-td-label">{$t('vm.nameserver')}</td>
						<td class="pv-td-mono">{config.cloudInit.nameserver}</td>
					</tr>
				{/if}
				{#if config.cloudInit.sshKeys}
					<tr class="pv-row">
						<td class="pv-td-label">{$t('vm.sshKeys')}</td>
						<td>
							<pre class="max-h-40 overflow-auto rounded bg-muted p-2 text-xs">{config.cloudInit.sshKeys}</pre>
						</td>
					</tr>
				{/if}
			</tbody>
		</table>
	{/if}
</div>
