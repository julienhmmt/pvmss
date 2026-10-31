<script lang="ts">
	/**
	 * CatalogFilesTable - the ISOs table: files on a storage (file / storage
	 * / node / size / approval columns). The toggling key prefix is
	 * "iso:{node}:{storage}:{file}".
	 */
	import type { AdminISO, ISOSortColumn } from './admin-catalog.svelte';
	import { formatBytes } from '#lib/shared/format-bytes.js';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import TableHeader from '#lib/shared/ui/TableHeader.svelte';
	import ApprovalCell from './ApprovalCell.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		files: AdminISO[];
		toggling: string | null;
		onToggle: (node: string, storage: string, file: string, enabled: boolean) => void;
		onRemove: (node: string, storage: string, file: string) => void;
		sortBy: ISOSortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: ISOSortColumn) => void;
	}

	let { files, toggling, onToggle, onRemove, sortBy, sortDir, onSort }: Props = $props();
</script>

<table class="pv-table pv-responsive-table">
	<caption class="sr-only">{m['admin.isos.heading']()}</caption>
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
			{@const key = `iso:${file.node}:${file.storage}:${file.file}`}
			<tr class="group transition-colors {file.missing ? 'opacity-60' : 'hover:bg-muted/40'}" data-testid="iso-row">
				<td class="font-mono" data-label={m['admin.catalog.file']()}>
					{file.file}{#if file.missing}
						<Pill
							tone="error"
							dot={false}
							label={m['admin.catalog.missingBadge']()}
							class="ml-2"
							data-testid="iso-missing-badge"
						/>
					{/if}
				</td>
				<td class="font-mono" data-label={m['common.storage']()}>{file.storage}</td>
				<td class="font-mono" data-label={m['common.node']()}>{file.node}</td>
				<td data-label={m['admin.catalog.size']()}>{formatBytes(file.sizeBytes)}</td>
				<td data-label={m['admin.catalog.statusColumn']()}>
					<ApprovalCell
						enabled={file.enabled}
						name={`${file.file} (${file.node})`}
						pending={toggling === key}
						missing={file.missing}
						onToggle={() => onToggle(file.node, file.storage, file.file, !file.enabled)}
						onRemove={() => onRemove(file.node, file.storage, file.file)}
						removeTestId="iso-remove"
					/>
				</td>
			</tr>
		{:else}
			<tr>
				<td colspan={5} class="p-0">
					<EmptyState title={m['admin.catalog.noIsos']()} />
				</td>
			</tr>
		{/each}
	</tbody>
</table>
