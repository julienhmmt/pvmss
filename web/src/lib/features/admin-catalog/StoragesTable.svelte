<script lang="ts">
	import type { AdminStorage } from './admin-catalog.svelte';
	import { formatBytes } from './format';

	interface Props {
		storages: AdminStorage[];
		toggling: string | null;
		onToggle: (name: string, node: string, enabled: boolean) => void;
	}

	let { storages, toggling, onToggle }: Props = $props();
</script>

<div class="overflow-x-auto rounded-lg border">
	<table class="w-full text-sm">
		<thead class="bg-muted/50 text-left">
			<tr>
				<th class="px-4 py-2 font-medium">Name</th>
				<th class="px-4 py-2 font-medium">Node</th>
				<th class="px-4 py-2 font-medium">Type</th>
				<th class="px-4 py-2 font-medium">Usage</th>
				<th class="px-4 py-2 font-medium">Approved</th>
			</tr>
		</thead>
		<tbody>
			{#each storages as storage (storage.name + storage.node)}
				<tr class="border-t">
					<td class="px-4 py-2 font-mono">{storage.name}</td>
					<td class="px-4 py-2 font-mono">{storage.node}</td>
					<td class="px-4 py-2">{storage.type}</td>
					<td class="px-4 py-2">{formatBytes(storage.usedBytes)} / {formatBytes(storage.totalBytes)}</td>
					<td class="px-4 py-2">
						<button
							type="button"
							class="rounded-md px-3 py-1 text-xs font-medium transition-colors {storage.enabled
								? 'bg-primary text-primary-foreground'
								: 'bg-muted text-muted-foreground hover:bg-muted/80'}"
							disabled={toggling === `storage:${storage.name}@${storage.node}`}
							onclick={() => onToggle(storage.name, storage.node, !storage.enabled)}
						>
							{#if toggling === `storage:${storage.name}@${storage.node}`}
								…
							{:else}
								{storage.enabled ? 'Approved' : 'Approve'}
							{/if}
						</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
