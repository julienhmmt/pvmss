<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import { Switch } from '$lib/components/ui/switch';
	import { Badge } from '$lib/components/ui/badge';
	import { getStorages, toggleStorage } from '$lib/api/admin/storage';
	import { formatBytes, formatPercent } from '$lib/utils/format';
	import { Database } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { Storage } from '$lib/types/admin';
	import * as Table from '$lib/components/ui/table';
	import * as Select from '$lib/components/ui/select';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let storages = $state<Storage[]>([]);
	let selectedNode = $state<string>('');

	const nodes = $derived([...new Set(storages.map((s) => s.node))].sort());

	const filteredStorages = $derived(
		selectedNode ? storages.filter((s) => s.node === selectedNode) : storages
	);

	function usageColor(used: number, total: number): string {
		const pct = Number(formatPercent(used, total));
		if (pct >= 80) return 'bg-destructive';
		if (pct >= 60) return 'bg-chart-2';
		return 'bg-primary';
	}

	async function load() {
		loading = true;
		error = null;
		try {
			storages = await getStorages();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	async function handleToggle(storage: string, node: string) {
		try {
			await toggleStorage(storage, node);
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	onMount(load);
</script>

<PageHeader title={$t('admin.storage.title')} icon={Database} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if storages.length === 0}
	<EmptyState title={$t('admin.storage.noStorage')} icon={Database} />
{:else}
	<div class="mb-4">
		<Select.Root type="single" value={selectedNode} onValueChange={(v) => (selectedNode = v ?? '')}>
			<Select.Trigger class="w-[200px]">
				{selectedNode || $t('admin.storage.allNodes')}
			</Select.Trigger>
			<Select.Content>
				<Select.Item value="">{$t('admin.storage.allNodes')}</Select.Item>
				{#each nodes as node}
					<Select.Item value={node}>{node}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>

	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>{$t('common.name')}</Table.Head>
					<Table.Head>{$t('common.type')}</Table.Head>
					<Table.Head>{$t('admin.storage.content')}</Table.Head>
					<Table.Head>{$t('common.node')}</Table.Head>
					<Table.Head>{$t('admin.storage.total')}</Table.Head>
					<Table.Head>{$t('admin.storage.used')}</Table.Head>
					<Table.Head>{$t('admin.storage.free')}</Table.Head>
					<Table.Head>{$t('admin.storage.usage')}</Table.Head>
					<Table.Head>{$t('common.enabled')}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each filteredStorages as s}
					<Table.Row>
						<Table.Cell class="font-medium">{s.storage}</Table.Cell>
						<Table.Cell>{s.type}</Table.Cell>
						<Table.Cell>
							<div class="flex flex-wrap gap-1">
								{#each (s.content ?? '').split(',').filter(Boolean) as ct}
									<Badge variant="outline" class="text-xs">{ct}</Badge>
								{/each}
							</div>
						</Table.Cell>
						<Table.Cell>{s.node}</Table.Cell>
						<Table.Cell>{formatBytes(s.total)}</Table.Cell>
						<Table.Cell>{formatBytes(s.used)}</Table.Cell>
						<Table.Cell>{formatBytes(s.free)}</Table.Cell>
						<Table.Cell>
							<div class="flex items-center gap-2">
								<div class="h-2 w-20 rounded-full bg-muted">
									<div
										class="h-2 rounded-full {usageColor(s.used, s.total)}"
										style="width: {formatPercent(s.used, s.total)}%"
									></div>
								</div>
								<span class="text-xs">{formatPercent(s.used, s.total)}%</span>
							</div>
						</Table.Cell>
						<Table.Cell>
							<Switch
								checked={s.enabled}
								onCheckedChange={() => handleToggle(s.storage, s.node)}
							/>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}
