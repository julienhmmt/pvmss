<script lang="ts">
	import type { AdminBridge } from './admin-catalog.svelte';
	import Button from '$lib/shared/ui/Button.svelte';

	interface Props {
		bridges: AdminBridge[];
		toggling: string | null;
		onToggle: (name: string, enabled: boolean) => void;
	}

	let { bridges, toggling, onToggle }: Props = $props();
</script>

<div class="overflow-x-auto rounded-lg border border-border">
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
				<tr class="border-t border-border">
					<td class="px-4 py-2 font-mono">{bridge.name}</td>
					<td class="px-4 py-2 font-mono">{bridge.node}</td>
					<td class="px-4 py-2">{bridge.comment || '—'}</td>
					<td class="px-4 py-2">
						<Button
							variant={bridge.enabled ? 'primary' : 'secondary'}
							size="sm"
							disabled={toggling === `bridge:${bridge.name}`}
							label={bridge.enabled ? `Revoke approval for ${bridge.name}` : `Approve ${bridge.name}`}
							onclick={() => onToggle(bridge.name, !bridge.enabled)}
						>
							{#if toggling === `bridge:${bridge.name}`}
								…
							{:else}
								{bridge.enabled ? 'Approved' : 'Approve'}
							{/if}
						</Button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
