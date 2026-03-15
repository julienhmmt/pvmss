<script lang="ts">
	import { onMount } from 'svelte';
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

<PageHeader title="Network Bridges" icon={WifiHigh} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if vmbrs.length === 0}
	<EmptyState title="No network bridges found" icon={WifiHigh} />
{:else}
	<div class="mb-4">
		<Select.Root type="single" value={selectedNode} onValueChange={(v) => (selectedNode = v ?? '')}>
			<Select.Trigger class="w-[200px]">
				{selectedNode || 'All Nodes'}
			</Select.Trigger>
			<Select.Content>
				<Select.Item value="">All Nodes</Select.Item>
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
					<Table.Head>Interface</Table.Head>
					<Table.Head>Node</Table.Head>
					<Table.Head>Type</Table.Head>
					<Table.Head>Ports</Table.Head>
					<Table.Head>Active</Table.Head>
					<Table.Head>Enabled</Table.Head>
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
								{v.active ? 'Active' : 'Inactive'}
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
