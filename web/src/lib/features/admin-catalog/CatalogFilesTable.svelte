<script lang="ts">
	/**
	 * CatalogFilesTable - the ISOs and cloud-images tables, one component.
	 * Both list files on a storage (file / storage / node / size / approval
	 * columns); they used to be two copies of the same markup kept in sync
	 * by hand. `kind` picks the message keys, test ids, and toggling key
	 * prefix ("iso:{node}:{storage}:{file}" / "image:{…}").
	 */
	import type { AdminImage, AdminISO, ImageSortColumn, ISOSortColumn } from './admin-catalog.svelte';
	import { formatBytes } from './format';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import ApprovalCell from './ApprovalCell.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type FileItem = AdminISO | AdminImage;
	// ISOs and cloud images share one column set - the union stays correct
	// even if a kind later grows a kind-specific column.
	type FileSortColumn = ISOSortColumn | ImageSortColumn;

	const CONFIG = {
		iso: {
			caption: () => m['admin.isos.heading'](),
			empty: () => m['admin.catalog.noIsos']()
		},
		image: {
			caption: () => m['admin.images.heading'](),
			empty: () => m['admin.images.noImages']()
		}
	} as const;

	interface Props {
		kind: keyof typeof CONFIG;
		files: FileItem[];
		toggling: string | null;
		onToggle: (node: string, storage: string, file: string, enabled: boolean) => void;
		onRemove: (node: string, storage: string, file: string) => void;
		sortBy: FileSortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: FileSortColumn) => void;
	}

	let { kind, files, toggling, onToggle, onRemove, sortBy, sortDir, onSort }: Props = $props();

	const config = $derived(CONFIG[kind]);
</script>

<table class="pv-table pv-responsive-table">
	<caption class="sr-only">{config.caption()}</caption>
	<thead>
		<tr>
			<TableHeader text={m['admin.catalog.file']()} column="file" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader text={m['common.storage']()} column="storage" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader text={m['common.node']()} column="node" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader text={m['admin.catalog.size']()} column="size" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader
				text={m['admin.catalog.statusColumn']()}
				tooltip={m['admin.catalog.tooltip.statusColumn']()}
				column="enabled"
				activeColumn={sortBy}
				{sortDir}
				{onSort}
			/>
		</tr>
	</thead>
	<tbody>
		{#each files as file (file.node + ':' + file.storage + ':' + file.file)}
			{@const key = `${kind}:${file.node}:${file.storage}:${file.file}`}
			<tr class="group transition-colors {file.missing ? 'opacity-60' : 'hover:bg-muted/40'}" data-testid="{kind}-row">
				<td class="font-mono" data-label={m['admin.catalog.file']()}>
					{file.file}{#if file.missing}
						<Pill
							tone="error"
							dot={false}
							label={m['admin.catalog.missingBadge']()}
							class="ml-2"
							data-testid="{kind}-missing-badge"
						/>
					{/if}
				</td>
				<td class="font-mono" data-label={m['common.storage']()}>{file.storage}</td>
				<td class="font-mono" data-label={m['common.node']()}>{file.node}</td>
				<td data-label={m['admin.catalog.size']()}>{formatBytes(file.sizeBytes)}</td>
				<td data-label={m['admin.catalog.statusColumn']()}>
					<ApprovalCell
						enabled={file.enabled}
						name={file.file}
						pending={toggling === key}
						missing={file.missing}
						onToggle={() => onToggle(file.node, file.storage, file.file, !file.enabled)}
						onRemove={() => onRemove(file.node, file.storage, file.file)}
						removeTestId="{kind}-remove"
					/>
				</td>
			</tr>
		{:else}
			<tr>
				<td colspan={5} class="p-0">
					<EmptyState title={config.empty()} />
				</td>
			</tr>
		{/each}
	</tbody>
</table>
