<script lang="ts">
	import type { AdminBridge } from './admin-catalog.svelte';

	interface Props {
		bridges: AdminBridge[];
		toggling: string | null;
		onToggle: (name: string, enabled: boolean) => void;
	}

	let { bridges, toggling, onToggle }: Props = $props();
</script>

<div class="overflow-x-auto rounded-lg border">
	<table class="w-full text-sm">
		<thead class="bg-muted/50 text-left">
			<tr>
				<th class="px-4 py-2 font-medium">Name</th>
				<th class="px-4 py-2 font-medium">Node</th>
				<th class="px-4 py-2 font-medium">Comment</th>
				<th class="px-4 py-2 font-medium">Approved</th>
			</tr>
		</thead>
		<tbody>
			{#each bridges as bridge (bridge.name + bridge.node)}
				<tr class="border-t">
					<td class="px-4 py-2 font-mono">{bridge.name}</td>
					<td class="px-4 py-2 font-mono">{bridge.node}</td>
					<td class="px-4 py-2">{bridge.comment || '—'}</td>
					<td class="px-4 py-2">
						<button
							type="button"
							class="rounded-md px-3 py-1 text-xs font-medium transition-colors {bridge.enabled
								? 'bg-primary text-primary-foreground'
								: 'bg-muted text-muted-foreground hover:bg-muted/80'}"
							disabled={toggling === `bridge:${bridge.name}`}
							onclick={() => onToggle(bridge.name, !bridge.enabled)}
						>
							{#if toggling === `bridge:${bridge.name}`}
								…
							{:else}
								{bridge.enabled ? 'Approved' : 'Approve'}
							{/if}
						</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
