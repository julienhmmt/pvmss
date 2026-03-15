<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import { Switch } from '$lib/components/ui/switch';
	import { Badge } from '$lib/components/ui/badge';
	import { getVMBRs, toggleVMBR } from '$lib/api/admin/vmbr';
	import { WifiHigh } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { VMBR } from '$lib/types/admin';
	import * as Table from '$lib/components/ui/table';
	import * as Select from '$lib/components/ui/select';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let vmbrs = $state<VMBR[]>([]);
	let selectedNode = $state<string>('');

	const nodes = $derived([...new Set(vmbrs.map((v) => v.node))].sort());

	const filteredVmbrs = $derived(
		selectedNode ? vmbrs.filter((v) => v.node === selectedNode) : vmbrs
	);

	async function load() {
		loading = true;
		error = null;
		try {
			vmbrs = await getVMBRs();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	async function handleToggle(iface: string, node: string) {
		try {
			await toggleVMBR(iface, node);
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	onMount(load);
</script>

<PageHeader title={$t('admin.vmbr.title')} icon={WifiHigh} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if vmbrs.length === 0}
	<EmptyState title={$t('admin.vmbr.noVmbr')} icon={WifiHigh} />
{:else}
	<div class="mb-4">
		<Select.Root type="single" value={selectedNode} onValueChange={(v) => (selectedNode = v ?? '')}>
			<Select.Trigger class="w-[200px]">
				{selectedNode || $t('admin.vmbr.allNodes')}
			</Select.Trigger>
			<Select.Content>
				<Select.Item value="">{$t('admin.vmbr.allNodes')}</Select.Item>
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
					<Table.Head>{$t('admin.vmbr.iface')}</Table.Head>
					<Table.Head>{$t('common.node')}</Table.Head>
					<Table.Head>{$t('common.type')}</Table.Head>
					<Table.Head>{$t('admin.vmbr.ports')}</Table.Head>
					<Table.Head>{$t('common.status')}</Table.Head>
					<Table.Head>{$t('common.enabled')}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each filteredVmbrs as v}
					<Table.Row>
						<Table.Cell class="font-medium">{v.iface}</Table.Cell>
						<Table.Cell>{v.node}</Table.Cell>
						<Table.Cell>{v.type}</Table.Cell>
						<Table.Cell>{v.bridge_ports || '—'}</Table.Cell>
						<Table.Cell>
							<Badge variant={v.active ? 'default' : 'secondary'}>
								{v.active ? $t('admin.vmbr.active') : $t('admin.vmbr.inactive')}
							</Badge>
						</Table.Cell>
						<Table.Cell>
							<Switch
								checked={v.enabled}
								onCheckedChange={() => handleToggle(v.iface, v.node)}
							/>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}
