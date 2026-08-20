<script lang="ts">
	import type { AdminISO } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import TooltipHeader from '$lib/shared/ui/TooltipHeader.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		isos: AdminISO[];
		toggling: string | null;
		onToggle: (node: string, storage: string, file: string, enabled: boolean) => void;
	}

	let { isos, toggling, onToggle }: Props = $props();
</script>

<div class="overflow-x-auto rounded-lg border border-border">
	<table class="pv-responsive-table text-sm">
		<thead class="bg-muted/50 text-left">
			<tr>
				<TooltipHeader text={m['admin.catalog.file']()} tooltip={m['admin.catalog.tooltip.isoFile']()} />
				<th class="px-4 py-2 font-medium">{m['common.storage']()}</th>
				<TooltipHeader text={m['common.node']()} tooltip={m['admin.catalog.tooltip.isoNode']()} />
				<th class="px-4 py-2 font-medium">{m['admin.catalog.size']()}</th>
				<TooltipHeader text={m['admin.catalog.statusColumn']()} tooltip={m['admin.catalog.tooltip.statusColumn']()} />
			</tr>
		</thead>
		<tbody>
			{#each isos as iso (iso.node + ':' + iso.storage + ':' + iso.file)}
				<tr class="border-t border-border">
					<td class="px-4 py-2 font-mono" data-label={m['admin.catalog.file']()}>{iso.file}</td>
					<td class="px-4 py-2 font-mono" data-label={m['common.storage']()}>{iso.storage}</td>
					<td class="px-4 py-2 font-mono" data-label={m['common.node']()}>{iso.node}</td>
					<td class="px-4 py-2" data-label={m['admin.catalog.size']()}>{formatBytes(iso.sizeBytes)}</td>
					<td class="px-4 py-2" data-label={m['admin.catalog.statusColumn']()}>
						<span
							class="inline-flex items-center gap-2"
							aria-busy={toggling === `iso:${iso.node}:${iso.storage}:${iso.file}`}
						>
							<Switch
								checked={iso.enabled}
								label={iso.enabled ? m['admin.catalog.revokeApproval']({ name: iso.file }) : m['admin.catalog.approveName']({ name: iso.file })}
								onToggle={() => onToggle(iso.node, iso.storage, iso.file, !iso.enabled)}
							/>
							<span class="text-xs text-muted-foreground">
								{#if toggling === `iso:${iso.node}:${iso.storage}:${iso.file}`}
									…
								{:else}
									{iso.enabled ? m['admin.catalog.approvedStatus']() : m['admin.catalog.approveAction']()}
								{/if}
							</span>
						</span>
					</td>
				</tr>
			{:else}
				<tr><td colspan={5} class="p-0">
					<EmptyState title={m['admin.catalog.noIsos']()} />
				</td></tr>
			{/each}
		</tbody>
	</table>
</div>
