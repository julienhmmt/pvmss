<script lang="ts">
	import type { AdminStorage } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		storages: AdminStorage[];
		toggling: string | null;
		onToggle: (name: string, node: string, enabled: boolean) => void;
	}

	let { storages, toggling, onToggle }: Props = $props();
</script>

<div class="overflow-x-auto rounded-lg border border-border">
	<table class="pv-responsive-table text-sm">
		<thead class="bg-muted/50 text-left">
			<tr>
				<th class="px-4 py-2 font-medium">{m['common.name']()}</th>
				<th class="px-4 py-2 font-medium">{m['common.node']()}</th>
				<th class="px-4 py-2 font-medium">{m['common.type']()}</th>
				<th class="px-4 py-2 font-medium">{m['admin.catalog.usage']()}</th>
				<th class="px-4 py-2 font-medium">{m['admin.catalog.approvedStatus']()}</th>
			</tr>
		</thead>
		<tbody>
			{#each storages as storage (storage.name + storage.node)}
				<tr class="border-t border-border">
					<td class="px-4 py-2 font-mono" data-label={m['common.name']()}>{storage.name}</td>
					<td class="px-4 py-2 font-mono" data-label={m['common.node']()}>{storage.node}</td>
					<td class="px-4 py-2" data-label={m['common.type']()}>{storage.type}</td>
					<td class="px-4 py-2" data-label={m['admin.catalog.usage']()}>{formatBytes(storage.usedBytes)} / {formatBytes(storage.totalBytes)}</td>
					<td class="px-4 py-2" data-label={m['admin.catalog.approvedStatus']()}>
						<span
							class="inline-flex items-center gap-2"
							aria-busy={toggling === `storage:${storage.name}@${storage.node}`}
						>
							<Switch
								checked={storage.enabled}
								label={storage.enabled ? m['admin.catalog.revokeApproval']({ name: storage.name }) : m['admin.catalog.approveName']({ name: storage.name })}
								onToggle={() => onToggle(storage.name, storage.node, !storage.enabled)}
							/>
							<span class="text-xs text-muted-foreground">
								{#if toggling === `storage:${storage.name}@${storage.node}`}
									…
								{:else}
									{storage.enabled ? m['admin.catalog.approvedStatus']() : m['admin.catalog.approveAction']()}
								{/if}
							</span>
						</span>
					</td>
				</tr>
			{:else}
				<tr><td colspan={5} class="p-0">
					<EmptyState title={m['admin.catalog.noStorages']()} />
				</td></tr>
			{/each}
		</tbody>
	</table>
</div>
