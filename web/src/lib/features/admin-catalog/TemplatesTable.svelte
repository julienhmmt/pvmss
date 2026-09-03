<script lang="ts">
	import type { AdminTemplate } from './admin-catalog.svelte';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type TemplateSortColumn = 'vmid' | 'name' | 'node' | 'disk' | 'cloudInit' | 'enabled';

	interface Props {
		templates: AdminTemplate[];
		toggling: string | null;
		onToggle: (vmid: number, enabled: boolean) => void;
		onRemove: (vmid: number) => void;
		sortBy: TemplateSortColumn;
		sortDir: 'asc' | 'desc';
		onSort: (column: TemplateSortColumn) => void;
	}

	let { templates, toggling, onToggle, onRemove, sortBy, sortDir, onSort }: Props = $props();

	function handleSort(column: string): void {
		onSort(column as TemplateSortColumn);
	}
</script>

<div class="overflow-x-auto rounded-lg border border-border">
	<table class="pv-responsive-table text-sm">
		<caption class="sr-only">{m['admin.templates.heading']()}</caption>
		<thead class="bg-muted/50 text-left">
			<tr>
				<TableHeader text={m['admin.templates.vmid']()} column="vmid" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<TableHeader text={m['admin.templates.name']()} column="name" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<TableHeader text={m['common.node']()} column="node" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<TableHeader text={m['admin.templates.disk']()} column="disk" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<TableHeader text={m['admin.templates.cloudInit']()} column="cloudInit" activeColumn={sortBy} {sortDir} onSort={handleSort} />
				<TableHeader
					text={m['admin.catalog.statusColumn']()}
					tooltip={m['admin.catalog.tooltip.statusColumn']()}
					column="enabled"
					activeColumn={sortBy}
					{sortDir}
					onSort={handleSort}
				/>
				<td class="px-4 py-3"><span class="sr-only">{m['admin.templates.remove']()}</span></td>
			</tr>
		</thead>
		<tbody>
			{#each templates as tmpl (tmpl.vmid)}
				<tr class="border-t border-border {tmpl.missing || tmpl.diskUnreadable ? 'opacity-60' : ''}" data-testid="template-row">
					<td class="px-4 py-3 font-mono" data-label={m['admin.templates.vmid']()}>{tmpl.vmid}</td>
					<td class="px-4 py-3" data-label={m['admin.templates.name']()}>
						{tmpl.name !== '' ? tmpl.name : `VMID ${tmpl.vmid}`}{#if tmpl.missing}
							<span
								class="ml-2 inline-flex items-center rounded-full border border-destructive/40 bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive"
								data-testid="template-missing-badge"
							>
								{m['admin.templates.missingBadge']()}
							</span>
						{:else if tmpl.diskUnreadable}
							<span
								class="ml-2 inline-flex items-center rounded-full border border-border bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground"
								data-testid="template-unreadable-badge"
							>
								{m['admin.templates.unreadableBadge']()}
							</span>
						{/if}
					</td>
					<td class="px-4 py-3 font-mono" data-label={m['common.node']()}>{tmpl.node}</td>
					<td class="px-4 py-3" data-label={m['admin.templates.disk']()}>
						{tmpl.diskSizeGB} GB · {tmpl.diskStorage}
					</td>
					<td class="px-4 py-3" data-label={m['admin.templates.cloudInit']()}>
						{#if tmpl.cloudInitCapable}
							<span class="inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
								{m['admin.templates.cloudInit']()}
							</span>
						{/if}
					</td>
					<td class="px-4 py-3" data-label={m['admin.catalog.statusColumn']()}>
						{#if tmpl.missing}
							<span class="text-xs text-muted-foreground">{m['admin.templates.missingBadge']()}</span>
						{:else}
							<span class="inline-flex items-center gap-2" aria-busy={toggling === `template:${tmpl.vmid}`}>
								<Switch
									checked={tmpl.enabled}
									disabled={tmpl.diskUnreadable && !tmpl.enabled}
									label={tmpl.enabled
										? m['admin.catalog.revokeApproval']({ name: tmpl.name })
										: m['admin.catalog.approveName']({ name: tmpl.name })}
									onToggle={() => onToggle(tmpl.vmid, !tmpl.enabled)}
								/>
								<span class="text-xs text-muted-foreground">
									{#if toggling === `template:${tmpl.vmid}`}
										…
									{:else}
										{tmpl.enabled ? m['admin.catalog.approvedStatus']() : m['admin.catalog.approveAction']()}
									{/if}
								</span>
							</span>
						{/if}
					</td>
					<td class="px-4 py-3" data-label={m['admin.templates.remove']()}>
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
</div>
