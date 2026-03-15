<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import StatusBadge from '$lib/components/data/StatusBadge.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Table from '$lib/components/ui/table';
	import { getAllVMs, vmAction } from '$lib/api/admin/vms';
	import { formatBytes } from '$lib/utils/format';
	import { Desktop } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { VM, VMAction } from '$lib/types/admin';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let vms = $state<VM[]>([]);

	async function load() {
		loading = true;
		error = null;
		try {
			vms = await getAllVMs();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	async function doAction(vm: VM, action: VMAction) {
		try {
			await vmAction(vm.vmid, action);
			toast.success(`${action} sent to VM ${vm.vmid}`);
			await load();
		} catch (e) {
			toast.error(`Failed to ${action} VM ${vm.vmid}: ${(e as Error).message}`);
		}
	}

	onMount(load);
</script>

<PageHeader title="Virtual Machines" icon={Desktop} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={8} />
{:else if vms.length === 0}
	<EmptyState title="No virtual machines found" icon={Desktop} />
{:else}
	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>VMID</Table.Head>
					<Table.Head>Name</Table.Head>
					<Table.Head>Node</Table.Head>
					<Table.Head>Status</Table.Head>
					<Table.Head>CPUs</Table.Head>
					<Table.Head>RAM</Table.Head>
					<Table.Head>Actions</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each vms as vm}
					<Table.Row>
						<Table.Cell class="font-medium">{vm.vmid}</Table.Cell>
						<Table.Cell>{vm.name}</Table.Cell>
						<Table.Cell>{vm.node}</Table.Cell>
						<Table.Cell><StatusBadge status={vm.status} /></Table.Cell>
						<Table.Cell>{vm.cpus}</Table.Cell>
						<Table.Cell>{formatBytes(vm.maxmem)}</Table.Cell>
						<Table.Cell>
							<DropdownMenu.Root>
								<DropdownMenu.Trigger>
									{#snippet child({ props })}
										<Button variant="outline" size="sm" {...props}>Actions</Button>
									{/snippet}
								</DropdownMenu.Trigger>
								<DropdownMenu.Content>
									<DropdownMenu.Item onclick={() => doAction(vm, 'start')}>Start</DropdownMenu.Item>
									<DropdownMenu.Item onclick={() => doAction(vm, 'shutdown')}>Shutdown</DropdownMenu.Item>
									<DropdownMenu.Item onclick={() => doAction(vm, 'reboot')}>Reboot</DropdownMenu.Item>
									<DropdownMenu.Separator />
									<DropdownMenu.Item class="text-destructive" onclick={() => doAction(vm, 'stop')}>
										Force Stop
									</DropdownMenu.Item>
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}
