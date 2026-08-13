<script lang="ts">
	import type { AdminISO } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';

	interface Props {
		isos: AdminISO[];
		toggling: string | null;
		onToggle: (storage: string, file: string, enabled: boolean) => void;
	}

	let { isos, toggling, onToggle }: Props = $props();
</script>

<div class="overflow-x-auto rounded-lg border border-border">
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
				<tr class="border-t border-border">
					<td class="px-4 py-2 font-mono">{iso.file}</td>
					<td class="px-4 py-2 font-mono">{iso.storage}</td>
					<td class="px-4 py-2 font-mono">{iso.node}</td>
					<td class="px-4 py-2">{formatBytes(iso.sizeBytes)}</td>
					<td class="px-4 py-2">
						<span
							class="inline-flex items-center gap-2"
							aria-busy={toggling === `iso:${iso.storage}:${iso.file}`}
						>
							<Switch
								checked={iso.enabled}
								label={iso.enabled ? `Revoke approval for ${iso.file}` : `Approve ${iso.file}`}
								onToggle={() => onToggle(iso.storage, iso.file, !iso.enabled)}
							/>
							<span class="text-xs text-muted-foreground">
								{#if toggling === `iso:${iso.storage}:${iso.file}`}
									…
								{:else}
									{iso.enabled ? 'Approved' : 'Approve'}
								{/if}
							</span>
						</span>
					</td>
				</tr>
			{:else}
				<tr><td colspan={5} class="p-0">
					<EmptyState title="No ISOs found on this cluster." />
				</td></tr>
			{/each}
		</tbody>
	</table>
</div>
