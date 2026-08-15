<script lang="ts">
	import type { AdminNode } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		nodes: AdminNode[];
		toggling: string | null;
		onToggle: (name: string, enabled: boolean) => void;
	}

	let { nodes, toggling, onToggle }: Props = $props();
</script>

<div class="overflow-x-auto rounded-lg border border-border">
	<table class="pv-responsive-table text-sm">
		<thead class="bg-muted/50 text-left">
			<tr>
				<th class="px-4 py-2 font-medium">{m['common.name']()}</th>
				<th class="px-4 py-2 font-medium">{m['common.status']()}</th>
				<th class="px-4 py-2 font-medium">{m['common.vms']()}</th>
				<th class="px-4 py-2 font-medium">{m['common.cpu']()}</th>
				<th class="px-4 py-2 font-medium">{m['common.memory']()}</th>
				<th class="px-4 py-2 font-medium">{m['admin.catalog.approvedStatus']()}</th>
			</tr>
		</thead>
		<tbody>
			{#each nodes as node (node.name)}
				<tr class="border-t border-border">
					<td class="px-4 py-2 font-mono" data-label={m['common.name']()}>{node.name}</td>
					<td class="px-4 py-2" data-label={m['common.status']()}>{node.status}</td>
					<td class="px-4 py-2" data-label={m['common.vms']()}>{node.vmCount}</td>
					<td class="px-4 py-2" data-label={m['common.cpu']()}>{node.cpuCores} {m['common.cores']()} ({(node.cpuUsage * 100).toFixed(0)}%)</td>
					<td class="px-4 py-2" data-label={m['common.memory']()}>{formatBytes(node.memoryUsed)} / {formatBytes(node.memoryTotal)}</td>
					<td class="px-4 py-2" data-label={m['admin.catalog.approvedStatus']()}>
						<span class="inline-flex items-center gap-2" aria-busy={toggling === `node:${node.name}`}>
							<Switch
								checked={node.enabled}
								label={node.enabled ? m['admin.catalog.revokeApproval']({ name: node.name }) : m['admin.catalog.approveName']({ name: node.name })}
								onToggle={() => onToggle(node.name, !node.enabled)}
							/>
							<span class="text-xs text-muted-foreground">
								{#if toggling === `node:${node.name}`}
									…
								{:else}
									{node.enabled ? m['admin.catalog.approvedStatus']() : m['admin.catalog.approveAction']()}
								{/if}
							</span>
						</span>
					</td>
				</tr>
			{:else}
				<tr><td colspan={6} class="p-0">
					<EmptyState title={m['admin.catalog.noNodes']()} />
				</td></tr>
			{/each}
		</tbody>
	</table>
</div>
