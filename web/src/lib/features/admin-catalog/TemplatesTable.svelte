<script lang="ts">
	import type { AdminTemplate, TemplateSortColumn } from './admin-catalog.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import ApprovalCell from './ApprovalCell.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		templates: AdminTemplate[];
		toggling: string | null;
		onToggle: (vmid: number, enabled: boolean) => void;
		onRemove: (vmid: number) => void;
		onEdit: (template: AdminTemplate) => void;
		sortBy: TemplateSortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: TemplateSortColumn) => void;
	}

	let { templates, toggling, onToggle, onRemove, onEdit, sortBy, sortDir, onSort }: Props = $props();
</script>

<table class="pv-table pv-responsive-table">
	<caption class="sr-only">{m['admin.templates.heading']()}</caption>
	<thead>
		<tr>
			<TableHeader text={m['admin.templates.vmid']()} column="vmid" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader text={m['admin.templates.name']()} column="name" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader text={m['common.node']()} column="node" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader text={m['admin.templates.disk']()} column="disk" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader text={m['admin.templates.cloudInit']()} column="cloudInit" activeColumn={sortBy} {sortDir} {onSort} />
			<TableHeader
				text={m['admin.catalog.statusColumn']()}
				tooltip={m['admin.catalog.tooltip.statusColumn']()}
				column="enabled"
				activeColumn={sortBy}
				{sortDir}
				{onSort}
			/>
			<TableHeader text={m['common.actions']()} />
		</tr>
	</thead>
	<tbody>
		{#each templates as tmpl (tmpl.vmid)}
			<tr class="group transition-colors hover:bg-muted/40 {tmpl.missing || tmpl.diskUnreadable ? 'opacity-60' : ''}" data-testid="template-row">
				<td class="font-mono" data-label={m['admin.templates.vmid']()}>{tmpl.vmid}</td>
				<td data-label={m['admin.templates.name']()}>
					{tmpl.name !== '' ? tmpl.name : `VMID ${tmpl.vmid}`}{#if tmpl.missing}
						<Pill
							tone="error"
							dot={false}
							label={m['admin.catalog.missingBadge']()}
							class="ml-2"
							data-testid="template-missing-badge"
						/>
					{:else if tmpl.diskUnreadable}
						<Pill
							tone="off"
							dot={false}
							label={m['admin.templates.unreadableBadge']()}
							class="ml-2"
							data-testid="template-unreadable-badge"
						/>
					{/if}{#if tmpl.overrideDiscovery}
						<Pill
							tone="off"
							dot={false}
							label={m['admin.templates.overrideBadge']()}
							class="ml-2"
							data-testid="template-override-badge"
						/>
					{/if}
				</td>
				<td class="font-mono" data-label={m['common.node']()}>{tmpl.node}</td>
				<td data-label={m['admin.templates.disk']()}>
					{tmpl.diskSizeGB} GB · {tmpl.diskStorage}
				</td>
				<td data-label={m['admin.templates.cloudInit']()}>
					{#if tmpl.cloudInitCapable}
						<Pill tone="off" dot={false} label={m['admin.templates.cloudInit']()} />
					{/if}
				</td>
				<td data-label={m['admin.catalog.statusColumn']()}>
					<ApprovalCell
						enabled={tmpl.enabled}
						name={tmpl.name}
						pending={toggling === `template:${tmpl.vmid}`}
						missing={tmpl.missing}
						disabled={tmpl.diskUnreadable && !tmpl.enabled}
						onToggle={() => onToggle(tmpl.vmid, !tmpl.enabled)}
					/>
				</td>
				<td data-label={m['common.actions']()}>
					<div class="flex items-center gap-1">
						{#if !tmpl.missing}
							<Button
								variant="secondary"
								size="sm"
								onclick={() => onEdit(tmpl)}
								data-testid="template-edit"
							>
								{m['admin.templates.edit']()}
							</Button>
						{/if}
						{#if tmpl.missing}
							<Button
								variant="ghost"
								size="sm"
								onclick={() => onRemove(tmpl.vmid)}
								data-testid="template-remove"
							>
								{m['admin.templates.remove']()}
							</Button>
						{/if}
					</div>
				</td>
			</tr>
		{:else}
			<tr>
				<td colspan={7} class="p-0">
					<EmptyState title={m['admin.catalog.noTemplates']()} />
				</td>
			</tr>
		{/each}
	</tbody>
</table>
