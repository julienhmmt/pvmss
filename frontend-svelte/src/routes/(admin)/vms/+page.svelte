<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import StatusBadge from '$lib/components/data/StatusBadge.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Select from '$lib/components/ui/select';
	import * as Table from '$lib/components/ui/table';
	import { getAllVMs, vmAction } from '$lib/api/admin/vms';
	import { formatBytes } from '$lib/utils/format';
	import { Desktop } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { VM, VMAction } from '$lib/types/admin';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let vms = $state<VM[]>([]);
	let page = $state(1);
	let perPage = $state(25);

	let totalPages = $derived(Math.max(1, Math.ceil(vms.length / perPage)));
	let startIndex = $derived((page - 1) * perPage);
	let endIndex = $derived(Math.min(startIndex + perPage, vms.length));
	let paginatedVMs = $derived(vms.slice(startIndex, endIndex));
	let subtitle = $derived(vms.length > 0 ? `${vms.length} ${$t('admin.vms.title')}` : undefined);

	function parseTags(tags: string): string[] {
		if (!tags) return [];
		return tags.split(';').filter((t) => t.trim().length > 0);
	}

	async function load() {
		if (vms.length > 0) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			vms = await getAllVMs();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function doAction(vm: VM, action: VMAction) {
		try {
			await vmAction(vm.vmid, action);
			toast.success($t('admin.vms.toast.actionSent', { values: { action, vmid: vm.vmid } }));
			await load();
		} catch (e) {
			toast.error($t('admin.vms.toast.actionFailed', { values: { action, vmid: vm.vmid, error: (e as Error).message } }));
		}
	}

	onMount(load);
</script>

<PageHeader title={$t('admin.vms.title')} description={subtitle} icon={Desktop} />

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={8} />
{:else if vms.length === 0}
	<EmptyState title={$t('admin.vms.noVms')} icon={Desktop} />
{:else}
	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>{$t('admin.vms.vmid')}</Table.Head>
					<Table.Head>{$t('common.name')}</Table.Head>
					<Table.Head>{$t('common.node')}</Table.Head>
					<Table.Head>{$t('common.status')}</Table.Head>
					<Table.Head>{$t('admin.vms.tags')}</Table.Head>
					<Table.Head>{$t('admin.vms.cpus')}</Table.Head>
					<Table.Head>{$t('admin.vms.ram')}</Table.Head>
					<Table.Head>{$t('common.actions')}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each paginatedVMs as vm}
					<Table.Row>
						<Table.Cell class="font-medium">{vm.vmid}</Table.Cell>
						<Table.Cell>{vm.name}</Table.Cell>
						<Table.Cell>{vm.node}</Table.Cell>
						<Table.Cell><StatusBadge status={vm.status} /></Table.Cell>
						<Table.Cell>
							<div class="flex flex-wrap gap-1">
								{#each parseTags(vm.tags) as tag}
									<Badge variant="secondary" class="text-xs">{tag}</Badge>
								{/each}
							</div>
						</Table.Cell>
						<Table.Cell>{vm.cpus}</Table.Cell>
						<Table.Cell>{formatBytes(vm.maxmem)}</Table.Cell>
						<Table.Cell>
							<DropdownMenu.Root>
								<DropdownMenu.Trigger>
									{#snippet child({ props })}
										<Button variant="outline" size="sm" {...props}>{$t('common.actions')}</Button>
									{/snippet}
								</DropdownMenu.Trigger>
								<DropdownMenu.Content>
									<DropdownMenu.Item onclick={() => doAction(vm, 'start')}>{$t('admin.vms.actions.start')}</DropdownMenu.Item>
									<DropdownMenu.Item onclick={() => doAction(vm, 'shutdown')}>{$t('admin.vms.actions.shutdown')}</DropdownMenu.Item>
									<DropdownMenu.Item onclick={() => doAction(vm, 'reboot')}>{$t('admin.vms.actions.reboot')}</DropdownMenu.Item>
									<DropdownMenu.Separator />
									<DropdownMenu.Item class="text-destructive" onclick={() => doAction(vm, 'stop')}>
										{$t('admin.vms.actions.forceStop')}
									</DropdownMenu.Item>
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>

	<div class="flex items-center justify-between pt-4">
		<p class="text-sm text-muted-foreground">
			{$t('admin.vms.pagination.showing', { values: { start: startIndex + 1, end: endIndex, total: vms.length } })}
		</p>

		<div class="flex items-center gap-4">
			<Select.Root
				type="single"
				value={String(perPage)}
				onValueChange={(v) => {
					perPage = Number(v);
					page = 1;
				}}
			>
				<Select.Trigger class="w-[110px]">
					{$t('admin.vms.pagination.perPage', { values: { count: perPage } })}
				</Select.Trigger>
				<Select.Content>
					<Select.Item value="10">10</Select.Item>
					<Select.Item value="25">25</Select.Item>
					<Select.Item value="50">50</Select.Item>
					<Select.Item value="100">100</Select.Item>
				</Select.Content>
			</Select.Root>

			<div class="flex items-center gap-2">
				<Button
					variant="outline"
					size="sm"
					disabled={page <= 1}
					onclick={() => {
						page = Math.max(1, page - 1);
					}}
				>
					{$t('common.previous')}
				</Button>
				<span class="text-sm text-muted-foreground">
					{$t('common.pageOf', { values: { page, total: totalPages } })}
				</span>
				<Button
					variant="outline"
					size="sm"
					disabled={page >= totalPages}
					onclick={() => {
						page = Math.min(totalPages, page + 1);
					}}
				>
					{$t('common.next')}
				</Button>
			</div>
		</div>
	</div>
{/if}
