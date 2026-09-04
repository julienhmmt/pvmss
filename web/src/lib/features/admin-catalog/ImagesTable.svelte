<script lang="ts">
	import type { AdminImage } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type ImageSortColumn = 'file' | 'storage' | 'node' | 'size' | 'enabled';

	interface Props {
		images: AdminImage[];
		toggling: string | null;
		onToggle: (node: string, storage: string, file: string, enabled: boolean) => void;
		onRemove: (node: string, storage: string, file: string) => void;
		sortBy: ImageSortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: ImageSortColumn) => void;
	}

	let { images, toggling, onToggle, onRemove, sortBy, sortDir, onSort }: Props = $props();

	function handleSort(column: string): void {
		onSort(column as ImageSortColumn);
	}
</script>

<table class="pv-responsive-table w-full text-sm">
	<caption class="sr-only">{m['admin.images.heading']()}</caption>
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
			{#each images as image (image.node + ':' + image.storage + ':' + image.file)}
				<tr class="group transition-colors {image.missing ? 'opacity-60' : 'hover:bg-muted/40'}" data-testid="image-row">
					<td class="px-4 py-3.5 font-mono" data-label={m['admin.catalog.file']()}>
						{image.file}{#if image.missing}
							<span
								class="ml-2 inline-flex items-center rounded-full border border-destructive/40 bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive"
								data-testid="image-missing-badge"
							>
								{m['admin.catalog.missingBadge']()}
							</span>
						{/if}
					</td>
					<td class="px-4 py-3.5 font-mono" data-label={m['common.storage']()}>{image.storage}</td>
					<td class="px-4 py-3.5 font-mono" data-label={m['common.node']()}>{image.node}</td>
					<td class="px-4 py-3.5" data-label={m['admin.catalog.size']()}>{formatBytes(image.sizeBytes)}</td>
					<td class="px-4 py-3.5" data-label={m['admin.catalog.statusColumn']()}>
						{#if image.missing}
							<span class="text-xs text-muted-foreground">{m['admin.catalog.missingBadge']()}</span>
						{:else}
							<span
								class="inline-flex items-center gap-2"
								aria-busy={toggling === `image:${image.node}:${image.storage}:${image.file}`}
							>
								<Switch
									checked={image.enabled}
									label={image.enabled
										? m['admin.catalog.revokeApproval']({ name: image.file })
										: m['admin.catalog.approveName']({ name: image.file })}
									onToggle={() => onToggle(image.node, image.storage, image.file, !image.enabled)}
								/>
								<span class="text-xs text-muted-foreground">
									{#if toggling === `image:${image.node}:${image.storage}:${image.file}`}
										…
									{:else}
										{image.enabled ? m['admin.catalog.approvedStatus']() : m['admin.catalog.approveAction']()}
									{/if}
								</span>
							</span>
						{/if}
					</td>
					<td class="px-4 py-3.5" data-label={m['admin.catalog.remove']()}>
						{#if image.missing}
							<Button
								variant="ghost"
								size="sm"
								onclick={() => onRemove(image.node, image.storage, image.file)}
								data-testid="image-remove"
							>
								{m['admin.catalog.remove']()}
							</Button>
						{/if}
					</td>
				</tr>
			{:else}
				<tr>
					<td colspan={6} class="p-0">
						<EmptyState title={m['admin.images.noImages']()} />
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
