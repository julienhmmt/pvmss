<script lang="ts">
	import type { AdminISO } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type ISOSortColumn = 'file' | 'storage' | 'node' | 'size' | 'enabled';

	interface Props {
		isos: AdminISO[];
		toggling: string | null;
		onToggle: (node: string, storage: string, file: string, enabled: boolean) => void;
		onRemove: (node: string, storage: string, file: string) => void;
		sortBy: ISOSortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: ISOSortColumn) => void;
	}

	let { isos, toggling, onToggle, onRemove, sortBy, sortDir, onSort }: Props = $props();

	function handleSort(column: string): void {
		onSort(column as ISOSortColumn);
	}
</script>

<table class="pv-responsive-table w-full text-sm">
	<caption class="sr-only">{m['admin.isos.heading']()}</caption>
	<thead class="bg-muted/60 text-left text-sm font-medium text-muted-foreground">
			<tr>
				<TableHeader text={m['admin.catalog.file']()} column="file" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<TableHeader text={m['common.storage']()} column="storage" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<TableHeader text={m['common.node']()} column="node" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<TableHeader text={m['admin.catalog.size']()} column="size" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<TableHeader
					text={m['admin.catalog.statusColumn']()}
					tooltip={m['admin.catalog.tooltip.statusColumn']()}
					column="enabled"
					activeColumn={sortBy}
					{sortDir}
					onSort={handleSort}
				/>
				<td class="px-4 py-3"><span class="sr-only">{m['admin.catalog.remove']()}</span></td>
			</tr>
		</thead>
		<tbody class="divide-y divide-border">
			{#each isos as iso (iso.node + ':' + iso.storage + ':' + iso.file)}
				<tr class="group transition-colors {iso.missing ? 'opacity-60' : 'hover:bg-muted/40'}" data-testid="iso-row">
					<td class="px-4 py-3.5 font-mono" data-label={m['admin.catalog.file']()}>
						{iso.file}{#if iso.missing}
							<span
								class="ml-2 inline-flex items-center rounded-full border border-destructive/40 bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive"
								data-testid="iso-missing-badge"
							>
								{m['admin.catalog.missingBadge']()}
							</span>
						{/if}
					</td>
					<td class="px-4 py-3.5 font-mono" data-label={m['common.storage']()}>{iso.storage}</td>
					<td class="px-4 py-3.5 font-mono" data-label={m['common.node']()}>{iso.node}</td>
					<td class="px-4 py-3.5" data-label={m['admin.catalog.size']()}>{formatBytes(iso.sizeBytes)}</td>
					<td class="px-4 py-3.5" data-label={m['admin.catalog.statusColumn']()}>
						{#if iso.missing}
							<span class="text-xs text-muted-foreground">{m['admin.catalog.missingBadge']()}</span>
						{:else}
							<span
								class="inline-flex items-center gap-2"
								aria-busy={toggling === `iso:${iso.node}:${iso.storage}:${iso.file}`}
							>
								<Switch
									checked={iso.enabled}
									label={iso.enabled
										? m['admin.catalog.revokeApproval']({ name: iso.file })
										: m['admin.catalog.approveName']({ name: iso.file })}
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
						{/if}
					</td>
					<td class="px-4 py-3.5" data-label={m['admin.catalog.remove']()}>
						{#if iso.missing}
							<Button
								variant="ghost"
								size="sm"
								onclick={() => onRemove(iso.node, iso.storage, iso.file)}
								data-testid="iso-remove"
							>
								{m['admin.catalog.remove']()}
							</Button>
						{/if}
					</td>
				</tr>
			{:else}
				<tr>
					<td colspan={6} class="p-0">
						<EmptyState title={m['admin.catalog.noIsos']()} />
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
