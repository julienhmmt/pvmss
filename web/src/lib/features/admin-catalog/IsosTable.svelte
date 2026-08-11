<script lang="ts">
	import type { AdminISO } from './admin-catalog.svelte';
	import { formatBytes } from './format';

	interface Props {
		isos: AdminISO[];
		toggling: string | null;
		onToggle: (storage: string, file: string, enabled: boolean) => void;
	}

	let { isos, toggling, onToggle }: Props = $props();
</script>

<div class="overflow-x-auto rounded-lg border">
	<table class="w-full text-sm">
		<thead class="bg-muted/50 text-left">
			<tr>
				<th class="px-4 py-2 font-medium">File</th>
				<th class="px-4 py-2 font-medium">Storage</th>
				<th class="px-4 py-2 font-medium">Node</th>
				<th class="px-4 py-2 font-medium">Size</th>
				<th class="px-4 py-2 font-medium">Approved</th>
			</tr>
		</thead>
		<tbody>
			{#each isos as iso (iso.storage + iso.file)}
				<tr class="border-t">
					<td class="px-4 py-2 font-mono">{iso.file}</td>
					<td class="px-4 py-2 font-mono">{iso.storage}</td>
					<td class="px-4 py-2 font-mono">{iso.node}</td>
					<td class="px-4 py-2">{formatBytes(iso.sizeBytes)}</td>
					<td class="px-4 py-2">
						<button
							type="button"
							class="rounded-md px-3 py-1 text-xs font-medium transition-colors {iso.enabled
								? 'bg-primary text-primary-foreground'
								: 'bg-muted text-muted-foreground hover:bg-muted/80'}"
							disabled={toggling === `iso:${iso.storage}:${iso.file}`}
							onclick={() => onToggle(iso.storage, iso.file, !iso.enabled)}
						>
							{#if toggling === `iso:${iso.storage}:${iso.file}`}
								…
							{:else}
								{iso.enabled ? 'Approved' : 'Approve'}
							{/if}
						</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
