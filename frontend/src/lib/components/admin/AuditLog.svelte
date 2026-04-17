<script lang="ts">
	import { t } from 'svelte-i18n';
	import { ClockIcon, TableIcon } from 'phosphor-svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Select from '$lib/components/ui/select';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import { getAuditLog } from '$lib/api/admin/audit';
	import type { AuditEntry } from '$lib/types/admin';

	const PAGE_SIZE = 50;

	const TABLE_OPTIONS = [
		{ value: '', label: 'All tables' },
		{ value: 'tags', label: 'tags' },
		{ value: 'vm_limits', label: 'vm_limits' },
		{ value: 'node_limits', label: 'node_limits' },
		{ value: 'enabled_nodes', label: 'enabled_nodes' },
		{ value: 'enabled_storages', label: 'enabled_storages' },
		{ value: 'enabled_isos', label: 'enabled_isos' },
		{ value: 'enabled_vmbrs', label: 'enabled_vmbrs' },
		{ value: 'cloudinit_templates', label: 'cloudinit_templates' },
		{ value: 'vm_profiles', label: 'vm_profiles' },
		{ value: 'sftp_config', label: 'sftp_config' },
	];

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let entries = $state<AuditEntry[]>([]);
	let tableFilter = $state('');
	let offset = $state(0);
	let hasMore = $state(false);

	async function load(resetPage = false) {
		if (resetPage) offset = 0;
		loading = true;
		error = null;
		try {
			// Request one extra entry to detect if there are more pages
			const resp = await getAuditLog({ table: tableFilter || undefined, limit: PAGE_SIZE + 1, offset });
			const allEntries = resp.entries ?? [];
			hasMore = allEntries.length > PAGE_SIZE;
			entries = hasMore ? allEntries.slice(0, PAGE_SIZE) : allEntries;
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	function formatDate(iso: string): string {
		try {
			return new Date(iso).toLocaleString();
		} catch {
			return iso;
		}
	}

	function actionBadgeClass(action: string): string {
		switch (action.toLowerCase()) {
			case 'insert': return 'pv-action-badge pv-action-badge--success';
			case 'update': return 'pv-action-badge pv-action-badge--vm';
			case 'delete': return 'pv-action-badge pv-action-badge--danger';
			default: return 'pv-action-badge';
		}
	}

	$effect(() => {
		load();
	});
</script>

<div class="space-y-4">
	<!-- Toolbar -->
	<div class="flex items-center gap-3 flex-wrap">
		<Select.Root
			type="single"
			value={tableFilter}
			onValueChange={(v) => { tableFilter = v ?? ''; load(true); }}
		>
			<Select.Trigger class="w-52">
				<TableIcon class="h-3.5 w-3.5 mr-2 text-muted-foreground" />
				{TABLE_OPTIONS.find((o) => o.value === tableFilter)?.label ?? 'All tables'}
			</Select.Trigger>
			<Select.Content>
				{#each TABLE_OPTIONS as opt}
					<Select.Item value={opt.value}>{opt.label}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
		<Button variant="outline" size="sm" onclick={() => load(true)}>
			{$t('common.refresh')}
		</Button>
		<span class="ml-auto text-xs text-muted-foreground">
			{$t('common.page')} {Math.floor(offset / PAGE_SIZE) + 1}
		</span>
	</div>

	<!-- Content -->
	{#if error}
		<ErrorBanner {error} onRetry={() => load()} />
	{:else if loading}
		<LoadingSkeleton variant="table" rows={8} />
	{:else if entries.length === 0}
		<EmptyState
			title={$t('admin.settings.audit.noEntries')}
			icon={ClockIcon}
			description={$t('admin.settings.audit.noEntriesDesc')}
		/>
	{:else}
		<div class="pv-table-wrap">
			<table class="pv-table text-sm">
				<thead>
					<tr>
						<th>{$t('admin.settings.audit.time')}</th>
						<th>{$t('admin.settings.audit.table')}</th>
						<th>{$t('admin.settings.audit.record')}</th>
						<th>{$t('admin.settings.audit.action')}</th>
						<th>{$t('admin.settings.audit.changedBy')}</th>
					</tr>
				</thead>
				<tbody>
					{#each entries as entry (entry.id)}
						<tr class="pv-row">
							<td class="pv-td-mono whitespace-nowrap">{formatDate(entry.changedAt)}</td>
							<td class="pv-td-mono">{entry.tableName}</td>
							<td class="pv-td-mono">{entry.recordId}</td>
							<td>
								<span class={actionBadgeClass(entry.action)}>{entry.action}</span>
							</td>
							<td class="pv-td-mono">{entry.changedBy || '—'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Pagination -->
		<div class="flex justify-between items-center pt-1">
			<Button
				variant="outline"
				size="sm"
				disabled={offset === 0}
				onclick={() => { offset = Math.max(0, offset - PAGE_SIZE); load(); }}
			>
				{$t('common.previous')}
			</Button>
			<Button
				variant="outline"
				size="sm"
				disabled={!hasMore}
				onclick={() => { offset += PAGE_SIZE; load(); }}
			>
				{$t('common.next')}
			</Button>
		</div>
	{/if}
</div>
